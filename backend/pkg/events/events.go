// Package events defines the domain-event contract (CloudEvents-style)
// shared by all services. The default Publisher logs JSON; a NATS
// JetStream adapter drops in here without touching usecases.
package events

import (
	"context"
	"log/slog"
	"time"

	"unital/backend/pkg/ids"
)

// Event is a CloudEvents 1.0-shaped envelope.
type Event struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`              // e.g. "contract.activated"
	Source  string         `json:"source"`            // e.g. "property"
	Subject string         `json:"subject,omitempty"` // aggregate id
	Time    time.Time      `json:"time"`
	Data    map[string]any `json:"data"`
}

// Publisher emits domain events. Implementations must be safe for
// concurrent use and at-least-once; consumers dedupe by ID.
type Publisher interface {
	Publish(ctx context.Context, e Event) error
}

// New builds an event with fresh id and timestamp.
func New(source, typ, subject string, data map[string]any) Event {
	return Event{ID: ids.New(), Type: typ, Source: source, Subject: subject, Time: time.Now().UTC(), Data: data}
}

// LogPublisher is the dev/default publisher: structured log output.
type LogPublisher struct{}

func (LogPublisher) Publish(ctx context.Context, e Event) error {
	slog.Info("event", "id", e.ID, "type", e.Type, "source", e.Source, "subject", e.Subject, "data", e.Data)
	return nil
}
