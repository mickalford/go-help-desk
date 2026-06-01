// Package clientstore implements domain/client.Store against PostgreSQL using
// raw SQL (bypassing sqlc-generated code so we don't need to regenerate).
package clientstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mickalford/opsmuster/backend/internal/dbgen"
	"github.com/mickalford/opsmuster/backend/internal/domain/client"
)

// Store implements client.Store.
type Store struct{ db dbgen.DBTX }

func New(db dbgen.DBTX) *Store { return &Store{db: db} }

// ── helpers ───────────────────────────────────────────────────────────────────

func scan(row *sql.Row) (client.Client, error) {
	var c client.Client
	err := row.Scan(&c.ID, &c.Name, &c.Domain, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return client.Client{}, client.ErrNotFound
	}
	return c, err
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func (s *Store) Create(ctx context.Context, c client.Client) (client.Client, error) {
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	const q = `
		INSERT INTO clients (id, name, domain, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, domain, notes, created_at, updated_at`
	row := s.db.QueryRowContext(ctx, q, c.ID, c.Name, c.Domain, c.Notes, c.CreatedAt, c.UpdatedAt)
	out, err := scan(row)
	if err != nil {
		if isUniqueViolation(err) {
			return client.Client{}, client.ErrDomainInUse
		}
		return client.Client{}, fmt.Errorf("creating client: %w", err)
	}
	return out, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (client.Client, error) {
	const q = `SELECT id, name, domain, notes, created_at, updated_at FROM clients WHERE id = $1`
	c, err := scan(s.db.QueryRowContext(ctx, q, id))
	if err != nil {
		return client.Client{}, fmt.Errorf("getting client: %w", err)
	}
	return c, nil
}

func (s *Store) GetByDomain(ctx context.Context, domain string) (client.Client, error) {
	const q = `SELECT id, name, domain, notes, created_at, updated_at FROM clients WHERE domain = $1`
	c, err := scan(s.db.QueryRowContext(ctx, q, domain))
	if err != nil {
		return client.Client{}, fmt.Errorf("getting client by domain: %w", err)
	}
	return c, nil
}

func (s *Store) List(ctx context.Context) ([]client.Client, error) {
	const q = `SELECT id, name, domain, notes, created_at, updated_at FROM clients ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer rows.Close()
	var out []client.Client
	for rows.Next() {
		var c client.Client
		if err := rows.Scan(&c.ID, &c.Name, &c.Domain, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning client: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Update(ctx context.Context, c client.Client) (client.Client, error) {
	c.UpdatedAt = time.Now()
	const q = `
		UPDATE clients SET name=$1, domain=$2, notes=$3, updated_at=$4
		WHERE id=$5
		RETURNING id, name, domain, notes, created_at, updated_at`
	out, err := scan(s.db.QueryRowContext(ctx, q, c.Name, c.Domain, c.Notes, c.UpdatedAt, c.ID))
	if err != nil {
		if isUniqueViolation(err) {
			return client.Client{}, client.ErrDomainInUse
		}
		return client.Client{}, fmt.Errorf("updating client: %w", err)
	}
	return out, nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM clients WHERE id = $1`
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

// ── Ticket associations ───────────────────────────────────────────────────────

func (s *Store) SetForTicket(ctx context.Context, ticketID uuid.UUID, clientID *uuid.UUID) error {
	const q = `UPDATE tickets SET client_id = $1, updated_at = now() WHERE id = $2`
	_, err := s.db.ExecContext(ctx, q, clientID, ticketID)
	return err
}

func (s *Store) GetForTicket(ctx context.Context, ticketID uuid.UUID) (*client.Client, error) {
	const q = `
		SELECT c.id, c.name, c.domain, c.notes, c.created_at, c.updated_at
		FROM clients c
		JOIN tickets t ON t.client_id = c.id
		WHERE t.id = $1`
	c, err := scan(s.db.QueryRowContext(ctx, q, ticketID))
	if errors.Is(err, client.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting client for ticket: %w", err)
	}
	return &c, nil
}

func (s *Store) GetMapForTickets(ctx context.Context, ticketIDs []uuid.UUID) (map[uuid.UUID]client.Client, error) {
	if len(ticketIDs) == 0 {
		return nil, nil
	}
	// Build a VALUES list for the IN clause.
	args := make([]interface{}, len(ticketIDs))
	placeholders := make([]string, len(ticketIDs))
	for i, id := range ticketIDs {
		args[i] = id
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	q := fmt.Sprintf(`
		SELECT t.id AS ticket_id, c.id, c.name, c.domain, c.notes, c.created_at, c.updated_at
		FROM clients c
		JOIN tickets t ON t.client_id = c.id
		WHERE t.id IN (%s)`,
		joinStrings(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("getting clients for tickets: %w", err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID]client.Client)
	for rows.Next() {
		var ticketID uuid.UUID
		var c client.Client
		if err := rows.Scan(&ticketID, &c.ID, &c.Name, &c.Domain, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out[ticketID] = c
	}
	return out, rows.Err()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique")
}

func joinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}
