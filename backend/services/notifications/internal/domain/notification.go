// Package domain holds the notification service's core model and ports.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrTemplateMissing = errors.New("template not found")
	ErrBadTarget       = errors.New("invalid target")
	ErrBadChannel      = errors.New("unknown channel")
	ErrForbidden       = errors.New("not allowed for this building")
)

// Channel names (extensible — registry-driven).
const (
	ChanInApp   = "inapp"
	ChanEmail   = "email"
	ChanSMS     = "sms"
	ChanWebhook = "webhook"
)

// Delivery / campaign states.
const (
	CampScheduled = "scheduled"
	CampSending   = "sending"
	CampDone      = "done"

	DelivPending = "pending"
	DelivSent    = "sent"
	DelivFailed  = "failed"
)

// Severity influences quiet-hours bypass.
const (
	SevNormal = "normal"
	SevUrgent = "urgent"
)

// TemplateVariant is the per-channel rendering of a template.
type TemplateVariant struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Template maps event-ish names to per-channel variants.
type Template struct {
	Name     string                     `json:"name"`
	Severity string                     `json:"severity"`
	Channels []string                   `json:"channels"` // default channels
	Variants map[string]TemplateVariant `json:"variants"` // channel -> variant
}

// TargetKind enumerates audience resolution.
type TargetKind string

const (
	TargetAll   TargetKind = "all"   // everyone in the building
	TargetRole  TargetKind = "role"  // owners | residents | staff | board
	TargetUsers TargetKind = "users" // explicit user ids
	TargetUnits TargetKind = "units" // current parties of units
)

// Target describes the audience of a campaign.
type Target struct {
	Kind   TargetKind `json:"kind"`
	Values []string   `json:"values,omitempty"` // role/user ids/unit ids depending on Kind
}

// Campaign is one send request fanned out to deliveries.
type Campaign struct {
	ID         string            `json:"id"`
	BuildingID string            `json:"building_id"`
	Template   string            `json:"template"`
	Target     Target            `json:"target"`
	Vars       map[string]string `json:"vars"`
	Channels   []string          `json:"channels"`
	Severity   string            `json:"severity"`
	ScheduleAt *time.Time        `json:"schedule_at,omitempty"`
	Status     string            `json:"status"`
	CreatedBy  string            `json:"created_by"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Delivery is the unit of dispatch: one recipient on one channel.
type Delivery struct {
	ID          string     `json:"id"`
	CampaignID  string     `json:"campaign_id"`
	RecipientID string     `json:"recipient_id"`
	Channel     string     `json:"channel"`
	To          string     `json:"to"` // endpoint: email/phone/user id
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	NextRetryAt time.Time  `json:"next_retry_at"`
	ProviderRef string     `json:"provider_ref,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
}

// InboxMessage is the in-app channel artifact.
type InboxMessage struct {
	ID         string            `json:"id"`
	UserID     string            `json:"user_id"`
	Title      string            `json:"title"`
	Body       string            `json:"body"`
	Vars       map[string]string `json:"vars,omitempty"`
	Read       bool              `json:"read"`
	CampaignID string            `json:"campaign_id"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Recipient carries per-user channel endpoints (resolved upstream).
type Recipient struct {
	ID    string
	Name  string
	Email string // "" if opted out / missing
	Phone string // MSISDN or ""
}

// Endpoint returns the address for a channel, ok=false when unavailable.
func (r Recipient) Endpoint(channel string) (string, bool) {
	switch channel {
	case ChanInApp:
		return r.ID, true
	case ChanEmail:
		return r.Email, r.Email != ""
	case ChanSMS:
		return r.Phone, r.Phone != ""
	}
	return "", false
}

// ---------- ports ----------

// TemplateStore persists templates.
type TemplateStore interface {
	Upsert(ctx context.Context, t *Template) error
	ByName(ctx context.Context, name string) (*Template, error)
	List(ctx context.Context) ([]Template, error)
}

// CampaignStore persists campaigns.
type CampaignStore interface {
	Create(ctx context.Context, c *Campaign) error
	ByID(ctx context.Context, id string) (*Campaign, error)
	Due(ctx context.Context, now time.Time) ([]Campaign, error) // scheduled & due
	MarkSending(ctx context.Context, id string) error
}

// DeliveryStore persists deliveries and supports worker claiming.
type DeliveryStore interface {
	Create(ctx context.Context, d *Delivery) error
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]Delivery, error)
	Update(ctx context.Context, d *Delivery) error
	ByCampaign(ctx context.Context, campaignID string) ([]Delivery, error)
}

// InboxStore persists per-user inbox messages.
type InboxStore interface {
	Push(ctx context.Context, m *InboxMessage) error
	ByUser(ctx context.Context, userID string, unreadOnly bool) ([]InboxMessage, error)
	MarkRead(ctx context.Context, id, userID string) error
}

// RecipientResolver expands targets into recipients (identity/property
// internal APIs in production; seeded directory in dev/tests).
type RecipientResolver interface {
	Resolve(ctx context.Context, buildingID string, t Target) ([]Recipient, error)
}

// MembershipChecker is the shared authorization port.
type MembershipChecker interface {
	HasRole(ctx context.Context, userID, buildingID, role string) (bool, error)
	AnyRole(ctx context.Context, userID, buildingID string, roles ...string) (bool, error)
}

// Message is what every channel sends.
type Message struct {
	To    string
	Title string
	Body  string
	Meta  map[string]string // campaign_id, delivery_id, severity
}

// Channel is the provider SPI implemented by adapters (inapp, email,
// sms, webhook, push…). Registered by name in the usecase.
type Channel interface {
	Name() string
	Send(ctx context.Context, m Message) (ref string, err error)
}
