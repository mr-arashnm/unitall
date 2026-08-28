// Package domain holds facilities' core model and ports.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrForbidden        = errors.New("not allowed for this building")
	ErrConflict         = errors.New("conflicts with an existing booking or maintenance")
	ErrInvalidState     = errors.New("invalid state or input")
	ErrClosed           = errors.New("outside facility opening hours")
	ErrTooFar           = errors.New("outside advance-booking window")
	ErrTooSoon          = errors.New("inside minimum advance window")
	ErrOverCapacity     = errors.New("participants exceed capacity")
	ErrUnderMaintenance = errors.New("facility under maintenance")
)

// Booking lifecycle.
const (
	BookingPending   = "pending"
	BookingConfirmed = "confirmed"
	BookingCompleted = "completed"
	BookingCancelled = "cancelled"
	BookingRejected  = "rejected"
)

// Facility is a bookable shared amenity.
type Facility struct {
	ID          string    `json:"id"`
	BuildingID  string    `json:"building_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // pool|gym|roof_garden|meeting_room|party_hall|guest_parking|playground|sports_court|library|business_center|other
	Description string    `json:"description,omitempty"`
	Capacity    int       `json:"capacity"`
	OpeningTime string    `json:"opening_time"` // "08:00"
	ClosingTime string    `json:"closing_time"` // "22:00"
	HourlyRate  int64     `json:"hourly_rate"`  // Rial; 0 = free
	MinAdvanceH int       `json:"min_advance_hours"`
	MaxAdvanceH int       `json:"max_advance_hours"`
	Rules       []string  `json:"rules,omitempty"`
	Images      []string  `json:"images,omitempty"` // object URLs
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// Booking reserves a facility for [Start,End).
type Booking struct {
	ID           string    `json:"id"`
	FacilityID   string    `json:"facility_id"`
	BuildingID   string    `json:"building_id"`
	UserID       string    `json:"user_id"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	Purpose      string    `json:"purpose"`
	Participants int       `json:"participants"`
	Status       string    `json:"status"`
	TotalCost    int64     `json:"total_cost"`
	ApprovedBy   string    `json:"approved_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// Maintenance blocks a facility for a period (affect_bookings=true also
// cancels overlapping pending/confirmed bookings).
type Maintenance struct {
	ID             string     `json:"id"`
	FacilityID     string     `json:"facility_id"`
	Title          string     `json:"title"`
	Type           string     `json:"type"` // routine|repair|cleaning|inspection|upgrade
	Priority       string     `json:"priority"`
	ScheduledStart time.Time  `json:"scheduled_start"`
	ScheduledEnd   time.Time  `json:"scheduled_end"`
	ActualStart    *time.Time `json:"actual_start,omitempty"`
	ActualEnd      *time.Time `json:"actual_end,omitempty"`
	Status         string     `json:"status"` // scheduled|in_progress|done
	AffectBookings bool       `json:"affect_bookings"`
	CreatedBy      string     `json:"created_by"`
}

// Ports.
type FacilityStore interface {
	Create(ctx context.Context, f *Facility) error
	ByID(ctx context.Context, id string) (*Facility, error)
	Update(ctx context.Context, f *Facility) error
	Delete(ctx context.Context, id string) error
	ListByBuilding(ctx context.Context, buildingID string) ([]Facility, error)
}

type BookingStore interface {
	Create(ctx context.Context, b *Booking) error
	ByID(ctx context.Context, id string) (*Booking, error)
	Update(ctx context.Context, b *Booking) error
	Overlaps(ctx context.Context, facilityID string, start, end time.Time) ([]Booking, error)
	ByUser(ctx context.Context, userID string) ([]Booking, error)
	ByBuilding(ctx context.Context, buildingID string, status string) ([]Booking, error)
}

type MaintenanceStore interface {
	Create(ctx context.Context, m *Maintenance) error
	ByID(ctx context.Context, id string) (*Maintenance, error)
	Update(ctx context.Context, m *Maintenance) error
	Overlaps(ctx context.Context, facilityID string, start, end time.Time) ([]Maintenance, error)
}

type MembershipChecker interface {
	HasRole(ctx context.Context, userID, buildingID, role string) (bool, error)
	AnyRole(ctx context.Context, userID, buildingID string, roles ...string) (bool, error)
}
