// Communications domain: announcements, meetings, and support tickets.
// These ride on the notification service's membership/resolver/inbox
// infrastructure (see docs/PROJECT_PLAN.md P5).
package domain

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidState = errors.New("invalid state or input")

// Announcement lifecycle.
const (
	AnnDraft     = "draft"
	AnnPublished = "published"
)

// Announcement is a building-wide notice; publishing fans it out to the
// in-app inbox of the targeted audience.
type Announcement struct {
	ID          string     `json:"id"`
	BuildingID  string     `json:"building_id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Priority    string     `json:"priority"` // low|normal|high|urgent
	Target      Target     `json:"target"`   // all|role|users (units arrives with the property internal API)
	Status      string     `json:"status"`   // draft|published
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	DeliveredTo int        `json:"delivered_to"` // inbox recipients at publish time
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Meeting lifecycle.
const (
	MeetScheduled = "scheduled"
	MeetOngoing   = "ongoing"
	MeetCompleted = "completed"
	MeetCancelled = "cancelled"
)

type Meeting struct {
	ID          string    `json:"id"`
	BuildingID  string    `json:"building_id"`
	Title       string    `json:"title"`
	Type        string    `json:"type"` // board|general|committee|emergency
	Description string    `json:"description,omitempty"`
	Agenda      string    `json:"agenda,omitempty"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Location    string    `json:"location,omitempty"`
	DurationMin int       `json:"duration_min,omitempty"`
	Status      string    `json:"status"`
	Target      Target    `json:"target"` // invited audience
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// RSVP statuses.
const (
	RsvpInvited   = "invited"
	RsvpConfirmed = "confirmed"
	RsvpDeclined  = "declined"
	RsvpAttended  = "attended"
	RsvpAbsent    = "absent"
)

// Attendance is one user's RSVP on a meeting (unique per meeting+user).
type Attendance struct {
	ID          string     `json:"id"`
	MeetingID   string     `json:"meeting_id"`
	UserID      string     `json:"user_id"`
	Status      string     `json:"status"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Minute is the single minutes document of a meeting.
type Minute struct {
	ID          string    `json:"id"`
	MeetingID   string    `json:"meeting_id"`
	Content     string    `json:"content"`
	Decisions   string    `json:"decisions,omitempty"`
	ActionItems string    `json:"action_items,omitempty"`
	SignedBy    []string  `json:"signed_by"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Support-ticket lifecycle.
const (
	TicketOpen       = "open"
	TicketInProgress = "in_progress"
	TicketResolved   = "resolved"
	TicketClosed     = "closed"
)

type Ticket struct {
	ID          string     `json:"id"`
	BuildingID  string     `json:"building_id"`
	UnitID      string     `json:"unit_id"`
	SubmittedBy string     `json:"submitted_by"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Type        string     `json:"type"` // technical|financial|complaint|suggestion|general
	Priority    string     `json:"priority"`
	Status      string     `json:"status"`
	AssignedTo  string     `json:"assigned_to,omitempty"`
	TeamID      string     `json:"team_id,omitempty"`
	SubmittedAt time.Time  `json:"submitted_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

// TicketResponse is one reply in a ticket thread.
type TicketResponse struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticket_id"`
	Author    string    `json:"author"`
	Message   string    `json:"message"`
	Internal  bool      `json:"internal"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------- ports ----------

type AnnouncementStore interface {
	Create(ctx context.Context, a *Announcement) error
	ByID(ctx context.Context, id string) (*Announcement, error)
	Update(ctx context.Context, a *Announcement) error
	Delete(ctx context.Context, id string) error
	ListByBuilding(ctx context.Context, buildingID string) ([]Announcement, error)
}

type MeetingStore interface {
	Create(ctx context.Context, m *Meeting) error
	ByID(ctx context.Context, id string) (*Meeting, error)
	Update(ctx context.Context, m *Meeting) error
	ListByBuilding(ctx context.Context, buildingID string) ([]Meeting, error)
}

type AttendanceStore interface {
	Upsert(ctx context.Context, a *Attendance) error // keyed by meeting+user
	ByMeeting(ctx context.Context, meetingID string) ([]Attendance, error)
}

type MinuteStore interface {
	Upsert(ctx context.Context, m *Minute) error // one per meeting
	ByMeeting(ctx context.Context, meetingID string) (*Minute, error)
}

type TicketStore interface {
	Create(ctx context.Context, t *Ticket) error
	ByID(ctx context.Context, id string) (*Ticket, error)
	Update(ctx context.Context, t *Ticket) error
	List(ctx context.Context, f TicketFilter) ([]Ticket, error)
}

type TicketFilter struct {
	BuildingID, Status, SubmittedBy string
}

type TicketResponseStore interface {
	Create(ctx context.Context, r *TicketResponse) error
	ByTicket(ctx context.Context, ticketID string) ([]TicketResponse, error)
}
