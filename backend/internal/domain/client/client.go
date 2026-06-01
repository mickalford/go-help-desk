package client

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Client represents an external organisation that tickets are raised for.
// The domain field is the canonical email domain used by the email-bridge
// to auto-match inbound forwarded messages.
type Client struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Domain    string    `json:"domain"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	ErrNotFound      = errors.New("client not found")
	ErrDomainInUse   = errors.New("a client with that domain already exists")
)
