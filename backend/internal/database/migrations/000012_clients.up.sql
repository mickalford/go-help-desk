-- Clients represent external organisations that tickets are raised on behalf of.
-- A client is identified primarily by its email domain, which the email-bridge
-- uses to auto-match inbound forwarded emails.

CREATE TABLE clients (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    domain     TEXT NOT NULL UNIQUE,  -- e.g. "acme.com.au", used for auto-matching
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX clients_domain_idx ON clients (domain);

ALTER TABLE tickets
    ADD COLUMN client_id UUID REFERENCES clients (id) ON DELETE SET NULL;

CREATE INDEX tickets_client_id_idx ON tickets (client_id);
