package client

import (
	"context"

	"github.com/google/uuid"
)

// Store is the persistence interface for clients and their ticket associations.
type Store interface {
	Create(ctx context.Context, c Client) (Client, error)
	GetByID(ctx context.Context, id uuid.UUID) (Client, error)
	GetByDomain(ctx context.Context, domain string) (Client, error)
	List(ctx context.Context) ([]Client, error)
	Update(ctx context.Context, c Client) (Client, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// SetForTicket links (or clears, when clientID is nil) a ticket to a client.
	SetForTicket(ctx context.Context, ticketID uuid.UUID, clientID *uuid.UUID) error
	// GetForTicket returns the client linked to a ticket, or nil when none.
	GetForTicket(ctx context.Context, ticketID uuid.UUID) (*Client, error)
	// GetMapForTickets returns a map of ticketID → Client for a batch of tickets.
	GetMapForTickets(ctx context.Context, ticketIDs []uuid.UUID) (map[uuid.UUID]Client, error)
}
