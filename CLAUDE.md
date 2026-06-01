# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
# Run all tests (unit + integration)
cd backend && go test ./...

# Unit tests only (no DB required)
cd backend && go test ./internal/domain/... ./internal/config/... ./internal/middleware/...

# Integration tests (requires postgres)
TEST_DATABASE_URL="postgres://helpdesk:helpdesk@localhost:5432/helpdesk?sslmode=disable" go test ./...

# Run server locally
cd backend && go run ./cmd/server

# Frontend dev server (proxies /api to :8080)
cd frontend && npm ci && npm run dev

# Regenerate sqlc (after editing queries/*.sql)
cd backend && sqlc generate

# Docker (full stack, recommended)
cd docker && cp .env.example .env && docker compose up -d
```

## Architecture

Three-layer stack: `domain` → `database store` → `server handler`. Each layer has a strict boundary.

```
HTTP → Chi Router → Auth Middleware → RBAC Middleware → Handler → Domain Service → DB Store → PostgreSQL
```

- **`internal/domain/`** — Pure business logic. No HTTP, no DB imports. Each subdomain has `entity types`, a `Service`, and a `Store` interface.
- **`internal/database/<entity>store/`** — Implements domain `Store` interfaces using sqlc-generated queries.
- **`internal/dbgen/`** — sqlc output. Never edit by hand; regenerate with `sqlc generate`.
- **`internal/server/`** — Chi HTTP handlers. Auth is enforced in middleware; handlers call domain services only.
- **`internal/middleware/`** — Three auth methods run in series: `SessionAuth` → `APIKeyAuth` → `BearerAuth`. First match sets the `Actor`.
- **`internal/mcp/`** — Model Context Protocol server at `/mcp/` (SSE). Registered tools operate directly against service layer.

## Actor & Auth

Handlers retrieve the authenticated caller via `middleware.GetActor(r)`, which returns:

```go
type Actor struct {
    UserID    uuid.UUID
    Role      user.Role     // admin | staff | user
    MFAPassed bool
    ClientID  string        // non-empty for OAuth2 clients
}
```

Role-gating via `RequireRole(roles...)` middleware; ticket visibility is enforced in the service layer (not the handler). Staff see assigned/group-assigned tickets; admins see all.

## Database & Migrations

- Migrations in `internal/database/migrations/*.sql` run automatically at startup (golang-migrate, advisory lock).
- To add a new migration: create `000012_description.up.sql` / `000012_description.down.sql` in that directory.
- Add SQL queries to `queries/*.sql`, annotate with sqlc directives, then run `sqlc generate` to update `internal/dbgen/`.
- Domain ↔ DB conversions live in `internal/database/convert.go` — keep them there, not in handlers or services.

## Testing Patterns

Integration tests get a real DB transaction per test, rolled back on cleanup — no truncation needed:

```go
db, closeDB := testutil.NewDB(t)        // connects to TEST_DATABASE_URL, runs migrations
defer closeDB()
q, rollback := testutil.TxQueries(t, db) // isolated transaction
defer rollback()
store := ticketstore.New(q)
```

Unit tests use table-driven cases with `t.Run`:

```go
cases := []struct{ name string; input X; want Y; wantErr bool }{ ... }
for _, tc := range cases {
    t.Run(tc.name, func(t *testing.T) { ... })
}
```

## Key Conventions

- **Error wrapping:** always `fmt.Errorf("context: %w", err)`. Domain returns errors; handlers/main log them.
- **No global state:** no `init()` side effects; all dependencies wired explicitly in `cmd/server/main.go`.
- **Unexported by default:** only export types needed across package boundaries.
- **No breaking changes to v1 API:** check `docs/DESIGN.md` for scope before adding features.
- Custom fields are stored in a JSONB column on tickets (`ticket_custom_fields`) keyed by field definition UUID — no migration needed to add new field types.

## Configuration

All config from environment variables (via `envconfig`). Required vars:

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | `postgres://user:pass@host/db?sslmode=disable` |
| `BASE_URL` | Public URL (used in SAML metadata + email links) |
| `SESSION_SECRET` | ≥32 chars, signs session cookies |
| `JWT_SECRET` | ≥32 chars, signs JWTs |

Optional: `SMTP_HOST/PORT/USER/PASSWORD/FROM` (email off if unset), `CLAMAV_ADDR` (virus scan off if unset), `ATTACHMENT_DIR` (default `/data/attachments`), `HTTP_PORT` (default `8080`).

First-run setup (`POST /api/v1/setup`) is permanently blocked once any user exists.
