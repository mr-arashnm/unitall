// Package domain holds property's core model: buildings, units, assets,
// occupancy parties, transfer history, and contracts — plus the ports
// usecases depend on. No framework imports.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrCodeTaken         = errors.New("code already exists")
	ErrUnitNotInBuilding = errors.New("unit is in a different building")
	ErrAssetOccupied     = errors.New("asset is already assigned")
	ErrAssetFree         = errors.New("asset is not assigned to this unit")
	ErrInvalidState      = errors.New("invalid state transition")
	ErrMissingSignatures = errors.New("contract not fully signed")
	ErrForbidden         = errors.New("not allowed for this building")
)

// Building is a managed complex.
type Building struct {
	ID        string
	Name      string
	Code      string // unique human code
	Type      string // residential|commercial|office|mixed
	Address   string
	Floors    int
	Features  []string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Unit is a dwelling/office inside a building.
type Unit struct {
	ID         string
	BuildingID string
	Floor      int
	Number     string // unique per building
	AreaM2     float64
	Rooms      int
	Status     string // occupied|vacant|under_construction
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AssetKind distinguishes parkings from warehouses.
type AssetKind string

const (
	AssetParking   AssetKind = "parking"
	AssetWarehouse AssetKind = "warehouse"
)

// Asset is a parking slot or warehouse assignable to units.
type Asset struct {
	ID         string
	Kind       AssetKind
	BuildingID string
	Code       string // unique per kind+building
	Floor      int
	AreaM2     float64 // warehouses may have area
	UnitID     string  // "" when available
}

// PartyRole is ownership or residency.
type PartyRole string

const (
	PartyOwner    PartyRole = "owner"
	PartyResident PartyRole = "resident"
)

// UnitParty is a validity-ranged owner/resident record. Current party
// has To=nil; history is the ordered list — replacing the mutable-FK
// pattern of the legacy system.
type UnitParty struct {
	ID     string
	UnitID string
	Role   PartyRole
	UserID string
	From   time.Time
	To     *time.Time
}

// TransferRecord audits a party change (projection of UnitParty rows).
type TransferRecord struct {
	ID             string
	UnitID         string
	Role           PartyRole
	PreviousUserID string
	NewUserID      string
	EffectiveDate  time.Time
	ContractNumber string
	RecordedBy     string
	Description    string
	CreatedAt      time.Time
}

// Contract types & statuses.
const (
	ContractPurchase = "purchase"
	ContractRental   = "rental"
	ContractTransfer = "transfer"

	ContractDraft     = "draft"
	ContractActive    = "active"
	ContractExpired   = "expired"
	ContractCancelled = "cancelled"
)

// Contract binds two parties over a unit.
type Contract struct {
	ID             string
	Number         string // unique, generated CONTRACT-YYYYMMDD-NNNN
	Type           string
	UnitID         string
	FirstPartyID   string
	SecondPartyID  string
	Title          string
	Amount         int64 // Rial minor units
	DepositAmount  int64
	StartDate      time.Time
	EndDate        *time.Time
	DurationMonths int
	Status         string
	FirstSigned    bool
	SecondSigned   bool
	SignedDate     *time.Time
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Ports.
type BuildingStore interface {
	Create(ctx context.Context, b *Building) error
	ByID(ctx context.Context, id string) (*Building, error)
	Update(ctx context.Context, b *Building) error
	ListByUser(ctx context.Context, userID string) ([]Building, error) // buildings where user manages or holds a unit
}

type UnitStore interface {
	Create(ctx context.Context, u *Unit) error
	ByID(ctx context.Context, id string) (*Unit, error)
	Update(ctx context.Context, u *Unit) error
	ListByBuilding(ctx context.Context, buildingID string) ([]Unit, error)
	CountByBuilding(ctx context.Context, buildingID string) (int, error)
}

type AssetStore interface {
	Create(ctx context.Context, a *Asset) error
	ByID(ctx context.Context, id string) (*Asset, error)
	Update(ctx context.Context, a *Asset) error
	ByCode(ctx context.Context, kind AssetKind, buildingID, code string) (*Asset, error)
	ListByBuilding(ctx context.Context, buildingID string, kind AssetKind) ([]Asset, error)
	Assign(ctx context.Context, assetID, unitID string) error
	Release(ctx context.Context, assetID string) error
}

type PartyStore interface {
	Current(ctx context.Context, unitID string, role PartyRole) (*UnitParty, error)
	Append(ctx context.Context, p *UnitParty) error // also closes the previous current row
	History(ctx context.Context, unitID string) ([]UnitParty, error)
	RecordTransfer(ctx context.Context, rec *TransferRecord) error
	Transfers(ctx context.Context, unitID string) ([]TransferRecord, error)
	UnitIDsByUser(ctx context.Context, userID string) ([]string, error)
}

type ContractStore interface {
	Create(ctx context.Context, c *Contract) error
	ByID(ctx context.Context, id string) (*Contract, error)
	Update(ctx context.Context, c *Contract) error
	ListByUnit(ctx context.Context, unitID string) ([]Contract, error)
	NextSequence(ctx context.Context, dateKey string) (int, error)
}

// MembershipChecker asks identity whether a user holds a role in a
// building (internal call or cached replica in production).
type MembershipChecker interface {
	HasRole(ctx context.Context, userID, buildingID, role string) (bool, error)
	AnyRole(ctx context.Context, userID, buildingID string, roles ...string) (bool, error)
}
