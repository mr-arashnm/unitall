// Package domain holds billing's core model and ports.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrDuplicate      = errors.New("charge already exists for period")
	ErrForbidden      = errors.New("not allowed for this building")
	ErrAlreadySettled = errors.New("already fully paid")
	ErrOverpayment    = errors.New("payment exceeds remaining amount")
)

// Charge statuses.
const (
	ChargePending       = "pending"
	ChargePaid          = "paid"
	ChargePartiallyPaid = "partially_paid"
	ChargeOverdue       = "overdue"
	ChargeCancelled     = "cancelled"
)

// Payment methods.
const (
	MethodOnline = "online"
	MethodCash   = "cash"
	MethodBank   = "bank_transfer"
	MethodCheque = "cheque"
	MethodPOS    = "pos"
)

// Transaction statuses.
const (
	TxPending   = "pending"
	TxCompleted = "completed"
	TxFailed    = "failed"
)

// Template defines a recurring charge for a building.
type Template struct {
	ID          string
	BuildingID  string
	Name        string
	Type        string // monthly|maintenance|elevator|cleaning|security|green_space|pool|gym|other
	Amount      int64  // Rial
	Active      bool
	Description string
	CreatedAt   time.Time
}

// Charge is one unit's due amount for a template and period (Jalali "YYYY-MM").
type Charge struct {
	ID         string
	BuildingID string
	UnitID     string
	TemplateID string
	Period     string
	Amount     int64
	DueDate    time.Time
	Status     string
	Paid       int64
	Remaining  int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Transaction records a payment attempt against a charge.
type Transaction struct {
	ID          string
	ChargeID    string
	Amount      int64
	Method      string
	Status      string
	Reference   string // TX-XXXXXXXXXXXX, unique
	PaymentDate *time.Time
	CreatedAt   time.Time
}

// Invoice aggregates a unit's charges for a period.
type Invoice struct {
	ID         string
	BuildingID string
	UnitID     string
	Period     string
	Total      int64
	Paid       int64
	Remaining  int64
	IsPaid     bool
	DueDate    time.Time
	UpdatedAt  time.Time
}

// Ports.
type TemplateStore interface {
	Create(ctx context.Context, t *Template) error
	ByID(ctx context.Context, id string) (*Template, error)
	Update(ctx context.Context, t *Template) error
	Delete(ctx context.Context, id string) error
	ListByBuilding(ctx context.Context, buildingID string) ([]Template, error)
}

type ChargeStore interface {
	Upsert(ctx context.Context, c *Charge) (created bool, err error) // idempotent on (unit, template, period)
	ByID(ctx context.Context, id string) (*Charge, error)
	List(ctx context.Context, filter ChargeFilter) ([]Charge, error)
	Update(ctx context.Context, c *Charge) error
}

type ChargeFilter struct {
	BuildingID, UnitID, Period, Status string
}

type TxStore interface {
	Create(ctx context.Context, tx *Transaction) error
	ByID(ctx context.Context, id string) (*Transaction, error)
	Update(ctx context.Context, tx *Transaction) error
}

type InvoiceStore interface {
	GetOrCreate(ctx context.Context, buildingID, unitID, period string) (*Invoice, error)
	Save(ctx context.Context, inv *Invoice) error
	List(ctx context.Context, buildingID string, unitID *string) ([]Invoice, error)
}

// UnitDirectory answers "which units belong to this building" without
// billing owning property data (anti-corruption port; production calls
// property's internal API, cached).
type UnitDirectory interface {
	UnitIDs(ctx context.Context, buildingID string) ([]string, error)
}

// MembershipChecker mirrors property's authorization port.
type MembershipChecker interface {
	HasRole(ctx context.Context, userID, buildingID, role string) (bool, error)
	AnyRole(ctx context.Context, userID, buildingID string, roles ...string) (bool, error)
}
