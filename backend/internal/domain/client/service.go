package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Service encapsulates client business logic.
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// List returns all clients ordered by name.
func (s *Service) List(ctx context.Context) ([]Client, error) {
	return s.store.List(ctx)
}

// GetByID returns a single client.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Client, error) {
	return s.store.GetByID(ctx, id)
}

// GetByDomain returns the client matching the given email domain, or ErrNotFound.
func (s *Service) GetByDomain(ctx context.Context, domain string) (Client, error) {
	return s.store.GetByDomain(ctx, strings.ToLower(strings.TrimSpace(domain)))
}

// Create stores a new client. Returns ErrDomainInUse if the domain is taken.
func (s *Service) Create(ctx context.Context, name, domain, notes string) (Client, error) {
	name = strings.TrimSpace(name)
	domain = strings.ToLower(strings.TrimSpace(domain))
	if name == "" {
		return Client{}, fmt.Errorf("name is required")
	}
	if domain == "" {
		return Client{}, fmt.Errorf("domain is required")
	}
	c := Client{
		ID:     uuid.New(),
		Name:   name,
		Domain: domain,
		Notes:  strings.TrimSpace(notes),
	}
	return s.store.Create(ctx, c)
}

// Update edits an existing client's name, domain, and notes.
func (s *Service) Update(ctx context.Context, id uuid.UUID, name, domain, notes string) (Client, error) {
	c, err := s.store.GetByID(ctx, id)
	if err != nil {
		return Client{}, err
	}
	if n := strings.TrimSpace(name); n != "" {
		c.Name = n
	}
	if d := strings.ToLower(strings.TrimSpace(domain)); d != "" {
		c.Domain = d
	}
	c.Notes = strings.TrimSpace(notes)
	return s.store.Update(ctx, c)
}

// Delete removes a client. Linked tickets have their client_id set to NULL.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.store.Delete(ctx, id)
}

// ResolveByDomain looks up the client for a given email domain. If no client
// exists and name is non-empty, a new client is created automatically.
// This is the primary entry point for the email-bridge.
func (s *Service) ResolveByDomain(ctx context.Context, domain, name string) (Client, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	c, err := s.store.GetByDomain(ctx, domain)
	if err == nil {
		return c, nil
	}
	if err != ErrNotFound {
		return Client{}, err
	}
	if name == "" {
		return Client{}, ErrNotFound
	}
	return s.Create(ctx, name, domain, "")
}

// SetForTicket links a ticket to a client. Pass nil to unlink.
func (s *Service) SetForTicket(ctx context.Context, ticketID uuid.UUID, clientID *uuid.UUID) error {
	return s.store.SetForTicket(ctx, ticketID, clientID)
}

// GetForTicket returns the client associated with a ticket, or nil.
func (s *Service) GetForTicket(ctx context.Context, ticketID uuid.UUID) (*Client, error) {
	return s.store.GetForTicket(ctx, ticketID)
}

// GetMapForTickets returns a ticketID → Client map for bulk ticket lists.
func (s *Service) GetMapForTickets(ctx context.Context, ticketIDs []uuid.UUID) (map[uuid.UUID]Client, error) {
	return s.store.GetMapForTickets(ctx, ticketIDs)
}
