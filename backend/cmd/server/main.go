package main

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/mickalford/opsmuster/backend/internal/config"
	"github.com/mickalford/opsmuster/backend/internal/database"
	"github.com/mickalford/opsmuster/backend/internal/database/adminstore"
	"github.com/mickalford/opsmuster/backend/internal/database/auditstore"
	"github.com/mickalford/opsmuster/backend/internal/database/authstore"
	"github.com/mickalford/opsmuster/backend/internal/database/categorystore"
	"github.com/mickalford/opsmuster/backend/internal/database/clientstore"
	"github.com/mickalford/opsmuster/backend/internal/database/customfieldstore"
	"github.com/mickalford/opsmuster/backend/internal/database/groupstore"
	"github.com/mickalford/opsmuster/backend/internal/database/registrationstore"
	"github.com/mickalford/opsmuster/backend/internal/database/slastore"
	"github.com/mickalford/opsmuster/backend/internal/database/tagstore"
	"github.com/mickalford/opsmuster/backend/internal/database/ticketstore"
	"github.com/mickalford/opsmuster/backend/internal/database/userstore"
	"github.com/mickalford/opsmuster/backend/internal/dbgen"
	"github.com/mickalford/opsmuster/backend/internal/domain/admin"
	"github.com/mickalford/opsmuster/backend/internal/domain/auth"
	"github.com/mickalford/opsmuster/backend/internal/domain/category"
	"github.com/mickalford/opsmuster/backend/internal/domain/client"
	"github.com/mickalford/opsmuster/backend/internal/domain/customfield"
	"github.com/mickalford/opsmuster/backend/internal/domain/group"
	"github.com/mickalford/opsmuster/backend/internal/domain/plugin"
	"github.com/mickalford/opsmuster/backend/internal/domain/registration"
	"github.com/mickalford/opsmuster/backend/internal/domain/sla"
	"github.com/mickalford/opsmuster/backend/internal/domain/tag"
	"github.com/mickalford/opsmuster/backend/internal/domain/ticket"
	"github.com/mickalford/opsmuster/backend/internal/domain/user"
	authmw "github.com/mickalford/opsmuster/backend/internal/middleware"
	"github.com/mickalford/opsmuster/backend/internal/mcp"
	"github.com/mickalford/opsmuster/backend/internal/server"
	"github.com/mickalford/opsmuster/backend/internal/server/notify"
	"github.com/mickalford/opsmuster/backend/internal/ui"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── Config ────────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// ── Logger ────────────────────────────────────────────────────────────────
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		log.Printf("warn: invalid LOG_LEVEL %q, defaulting to info", cfg.LogLevel)
		logLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	// ── Database ─────────────────────────────────────────────────────────────
	// Run migrations before opening the pool so the schema is always current.
	if err := database.Migrate(ctx, database.MigrateURL(cfg.DatabaseURL)); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	pool, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	// sqlc requires a database/sql-compatible interface; wrap the pgxpool.
	sqlDB := stdlib.OpenDBFromPool(pool)
	q := dbgen.New(sqlDB)

	// ── Stores ────────────────────────────────────────────────────────────────
	uStore := userstore.New(q)
	tStore := ticketstore.New(q)
	cStore := categorystore.New(q)
	gStore := groupstore.New(q)
	aStore := adminstore.New(q)
	auStore := auditstore.New(q)
	slStore := slastore.New(q)
	authStore := authstore.New(q)
	cfStore := customfieldstore.New(q)
	regStore := registrationstore.New(q)

	// ── Domain services ───────────────────────────────────────────────────────
	tagStore := tagstore.New(q)
	clientStore := clientstore.New(sqlDB)

	userSvc := user.NewService(uStore)
	categorySvc := category.NewService(cStore)
	groupSvc := group.NewService(gStore)
	tagSvc := tag.NewService(tagStore)
	clientSvc := client.NewService(clientStore)
	adminSvc := admin.NewService(aStore)
	customFieldSvc := customfield.NewService(cfStore)

	// slaPolicySvc is always created so the admin blade can manage policies
	// regardless of whether SLA enforcement is active.
	slaPolicySvc := sla.NewService(slStore)

	// slaSvc is passed to the ticket service for enforcement; nil when disabled
	// via the SLA_ENABLED env var (preserves existing behaviour).
	// Declared as the interface type so the zero value is a true nil interface,
	// not a (*sla.Service)(nil) wrapped in an interface (which would pass != nil checks).
	var slaSvc ticket.SLAService
	if cfg.SLAEnabled {
		slaSvc = sla.NewService(slStore)
	}

	// ── Notifications ─────────────────────────────────────────────────────────
	emailDisp, err := notify.NewEmailDispatcher(cfg)
	if err != nil {
		return fmt.Errorf("initialising email dispatcher: %w", err)
	}
	webhookDisp := notify.NewWebhookDispatcher(authStore)
	dispatcher := notify.NewMulti(emailDisp, webhookDisp)

	// ── Registration service ──────────────────────────────────────────────────
	registrationSvc := registration.NewService(regStore, userSvc, emailDisp, cfg.BaseURL)

	// ── Plugin registry ───────────────────────────────────────────────────────
	pluginRegistry := plugin.NewRegistry()

	// ── Ticket service ────────────────────────────────────────────────────────
	ticketSvc := ticket.NewService(tStore, tStore, dispatcher, auStore, slaSvc)
	if err := ticketSvc.LoadSystemStatuses(ctx); err != nil {
		return fmt.Errorf("loading system statuses: %w", err)
	}

	// ── Auth helpers ──────────────────────────────────────────────────────────
	gob.Register(auth.SessionData{})
	sessionStore := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	apiKeyLookup := authmw.APIKeyAuthFunc(func(ctx context.Context, hashed string) (auth.APIKey, user.User, error) {
		key, err := authStore.GetByHash(ctx, hashed)
		if err != nil {
			return auth.APIKey{}, user.User{}, err
		}
		u, err := userSvc.GetByID(ctx, key.UserID)
		if err != nil {
			return auth.APIKey{}, user.User{}, err
		}
		return key, u, nil
	})

	// ── HTTP server ───────────────────────────────────────────────────────────
	srv := server.New(
		cfg,
		sessionStore,
		userSvc,
		ticketSvc,
		categorySvc,
		groupSvc,
		tagSvc,
		clientSvc,
		adminSvc,
		customFieldSvc,
		slaPolicySvc,
		pluginRegistry,
		apiKeyLookup,
		authStore,
		authStore,
		registrationSvc,
	)

	srv.InitSAML(ctx)

	// ── MCP server (mounted under /mcp) ───────────────────────────────────────
	mcpSrv := mcp.New(ticketSvc)

	mux := http.NewServeMux()
	mux.Handle("/mcp/", mcpSrv.Handler())
	mux.Handle("/api/", srv)
	mux.Handle("/health", srv)
	mux.Handle("/", server.NewSPAHandler(ui.FS()))

	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	shutdownDone := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		shutdownDone <- httpSrv.Shutdown(shutdownCtx)
	}()

	slog.Info("opsmuster listening", "port", cfg.HTTPPort)
	if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	return <-shutdownDone
}

