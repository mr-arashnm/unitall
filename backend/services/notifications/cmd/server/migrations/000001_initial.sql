-- Notifications initial schema: support tickets.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS tickets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    building_id   TEXT NOT NULL,
    unit_id       TEXT NOT NULL DEFAULT '',
    submitted_by  TEXT NOT NULL DEFAULT '',
    title         TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    type          TEXT NOT NULL DEFAULT 'general',
    priority      TEXT NOT NULL DEFAULT 'medium',
    status        TEXT NOT NULL DEFAULT 'open',
    assigned_to   TEXT NOT NULL DEFAULT '',
    team_id       TEXT NOT NULL DEFAULT '',
    submitted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ,
    closed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tickets_building ON tickets(building_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(building_id, status);

CREATE TABLE IF NOT EXISTS ticket_responses (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id  UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author     TEXT NOT NULL DEFAULT '',
    message    TEXT NOT NULL DEFAULT '',
    internal   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ticket_responses_ticket ON ticket_responses(ticket_id);
