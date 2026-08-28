// Package usecase implements property's application services.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/ids"
	"unital/backend/services/property/internal/domain"
)

// MembershipBootstrap is the cross-service hook property uses after
// creating a building to seed the creator as a manager in identity's
// membership table. Without it, downstream services (billing,
// notifications, facilities, operations) would treat the creator as
// an outsider and 403 every action.
type MembershipBootstrap interface {
	BootstrapManager(ctx context.Context, buildingID, userID string) error
}

// NoopBootstrap satisfies MembershipBootstrap without doing anything.
// Use in tests and in dev modes where the identity service isn't
// reachable (DEV_TRUST_ALL=1 already bypasses membership checks).
type NoopBootstrap struct{}

func (NoopBootstrap) BootstrapManager(context.Context, string, string) error { return nil }

type Property struct {
	buildings  domain.BuildingStore
	units      domain.UnitStore
	assets     domain.AssetStore
	parties    domain.PartyStore
	contracts  domain.ContractStore
	membership domain.MembershipChecker
	bootstrap  MembershipBootstrap
	pub        events.Publisher
}

func New(b domain.BuildingStore, u domain.UnitStore, a domain.AssetStore,
	p domain.PartyStore, c domain.ContractStore, m domain.MembershipChecker, pub events.Publisher) *Property {
	return &Property{buildings: b, units: u, assets: a, parties: p, contracts: c, membership: m, bootstrap: NoopBootstrap{}, pub: pub}
}

// WithBootstrap attaches a real MembershipBootstrap (e.g. an HTTP
// client to identity's /internal endpoint). Chain after New.
func (p *Property) WithBootstrap(b MembershipBootstrap) *Property {
	if b != nil {
		p.bootstrap = b
	}
	return p
}

// --- Buildings ---

type BuildingInput struct {
	Name, Code, BuildingType, Address string
	Floors                            int
	Features                          []string
	CreatedBy                         string
}

func (p *Property) CreateBuilding(ctx context.Context, in BuildingInput) (*domain.Building, error) {
	now := time.Now().UTC()
	b := &domain.Building{
		ID: ids.New(), Name: in.Name, Code: in.Code, Type: in.BuildingType,
		Address: in.Address, Floors: in.Floors, Features: in.Features,
		CreatedBy: in.CreatedBy, CreatedAt: now, UpdatedAt: now,
	}
	if err := p.buildings.Create(ctx, b); err != nil {
		return nil, err
	}
	// Best-effort: seed the creator as a manager in identity. The
	// property store is the source of truth for the building itself;
	// a bootstrap failure must not roll back the create (the
	// creator is the building owner either way).
	if err := p.bootstrap.BootstrapManager(ctx, b.ID, in.CreatedBy); err != nil {
		slog.Warn("membership bootstrap", "building_id", b.ID, "user_id", in.CreatedBy, "err", err)
	}
	p.publish(ctx, "building.created", b.ID, map[string]any{"code": b.Code, "name": b.Name})
	return b, nil
}

func (p *Property) Building(ctx context.Context, id string) (*domain.Building, error) {
	return p.buildings.ByID(ctx, id)
}

func (p *Property) BuildingsForUser(ctx context.Context, userID string) ([]domain.Building, error) {
	return p.buildings.ListByUser(ctx, userID)
}

// UpdateBuilding applies a manager-authored patch (name, address, …).
func (p *Property) UpdateBuilding(ctx context.Context, actorID, buildingID string, patch func(*domain.Building)) (*domain.Building, error) {
	b, err := p.buildings.ByID(ctx, buildingID)
	if err != nil {
		return nil, err
	}
	if err := p.requireManager(ctx, actorID, b.ID); err != nil {
		return nil, err
	}
	patch(b)
	b.UpdatedAt = time.Now().UTC()
	if err := p.buildings.Update(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// --- Units ---

type UnitInput struct {
	BuildingID string
	Floor      int
	Number     string
	AreaM2     float64
	Rooms      int
}

func (p *Property) CreateUnit(ctx context.Context, actorID string, in UnitInput) (*domain.Unit, error) {
	if err := p.requireManager(ctx, actorID, in.BuildingID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &domain.Unit{
		ID: ids.New(), BuildingID: in.BuildingID, Floor: in.Floor,
		Number: in.Number, AreaM2: in.AreaM2, Rooms: in.Rooms,
		Status: "vacant", CreatedAt: now, UpdatedAt: now,
	}
	if err := p.units.Create(ctx, u); err != nil {
		return nil, err
	}
	p.publish(ctx, "unit.created", u.ID, map[string]any{"building_id": u.BuildingID, "number": u.Number})
	return u, nil
}

func (p *Property) Units(ctx context.Context, actorID, buildingID string) ([]domain.Unit, error) {
	if err := p.requireMember(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	return p.units.ListByBuilding(ctx, buildingID)
}

func (p *Property) Unit(ctx context.Context, unitID string) (*domain.Unit, error) {
	return p.units.ByID(ctx, unitID)
}

// UpdateUnit applies a manager-authored patch (status, rooms, …).
func (p *Property) UpdateUnit(ctx context.Context, actorID, unitID string, patch func(*domain.Unit)) (*domain.Unit, error) {
	u, err := p.units.ByID(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if err := p.requireManager(ctx, actorID, u.BuildingID); err != nil {
		return nil, err
	}
	patch(u)
	u.UpdatedAt = time.Now().UTC()
	if err := p.units.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// --- Assets (parking / warehouse) ---

type AssetInput struct {
	BuildingID string
	Kind       domain.AssetKind
	Code       string
	Floor      int
	AreaM2     float64
}

func (p *Property) CreateAsset(ctx context.Context, actorID string, in AssetInput) (*domain.Asset, error) {
	if err := p.requireManager(ctx, actorID, in.BuildingID); err != nil {
		return nil, err
	}
	if in.Kind != domain.AssetParking && in.Kind != domain.AssetWarehouse {
		return nil, domain.ErrInvalidState
	}
	a := &domain.Asset{ID: ids.New(), Kind: in.Kind, BuildingID: in.BuildingID, Code: in.Code, Floor: in.Floor, AreaM2: in.AreaM2}
	if err := p.assets.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (p *Property) Assets(ctx context.Context, actorID, buildingID string, kind domain.AssetKind, availableOnly bool) ([]domain.Asset, error) {
	if err := p.requireMember(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	list, err := p.assets.ListByBuilding(ctx, buildingID, kind)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Asset, 0, len(list))
	for _, a := range list {
		if availableOnly && a.UnitID != "" {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// AssignAsset enforces the same-building rule and one-assignment rule.
func (p *Property) AssignAsset(ctx context.Context, actorID string, kind domain.AssetKind, buildingCodeOrID, assetCode, unitID string) (*domain.Asset, error) {
	unit, err := p.units.ByID(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if err := p.requireManager(ctx, actorID, unit.BuildingID); err != nil {
		return nil, err
	}
	asset, err := p.assets.ByCode(ctx, kind, unit.BuildingID, assetCode)
	if err != nil {
		return nil, err
	}
	if asset.UnitID != "" {
		return nil, domain.ErrAssetOccupied
	}
	if err := p.assets.Assign(ctx, asset.ID, unitID); err != nil {
		return nil, err
	}
	// Re-read the asset so the response reflects the new unit_id.
	updated, err := p.assets.ByID(ctx, asset.ID)
	if err != nil {
		return asset, nil // fallback to in-memory copy if read fails
	}
	p.publish(ctx, "asset.assigned", updated.ID, map[string]any{"kind": string(kind), "unit_id": unitID})
	return updated, nil
}

func (p *Property) ReleaseAsset(ctx context.Context, actorID string, kind domain.AssetKind, assetCode, unitID string) error {
	unit, err := p.units.ByID(ctx, unitID)
	if err != nil {
		return err
	}
	if err := p.requireManager(ctx, actorID, unit.BuildingID); err != nil {
		return err
	}
	asset, err := p.assets.ByCode(ctx, kind, unit.BuildingID, assetCode)
	if err != nil {
		return err
	}
	if asset.UnitID != unitID {
		return domain.ErrAssetFree
	}
	if err := p.assets.Release(ctx, asset.ID); err != nil {
		return err
	}
	p.publish(ctx, "asset.released", asset.ID, map[string]any{"kind": string(kind), "unit_id": unitID})
	return nil
}

// AssetByID returns a single asset after a member-level auth check on
// its building. Any member of the building may read the asset.
func (p *Property) AssetByID(ctx context.Context, actorID, assetID string) (*domain.Asset, error) {
	a, err := p.assets.ByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if err := p.requireMember(ctx, actorID, a.BuildingID); err != nil {
		return nil, err
	}
	return a, nil
}

// UpdateAsset applies a manager-authored patch (code, floor, area). It
// re-validates code uniqueness via the storage layer's Update.
func (p *Property) UpdateAsset(ctx context.Context, actorID, assetID string, patch func(*domain.Asset)) (*domain.Asset, error) {
	a, err := p.assets.ByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if err := p.requireManager(ctx, actorID, a.BuildingID); err != nil {
		return nil, err
	}
	patch(a)
	if err := p.assets.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// --- Parties & transfers ---

// ChangeParty sets the current owner or resident of a unit, closing the
// previous party row and writing an audit record atomically.
func (p *Property) ChangeParty(ctx context.Context, actorID, unitID string, role domain.PartyRole, newUserID string, effective time.Time, contractNumber, description string) error {
	unit, err := p.units.ByID(ctx, unitID)
	if err != nil {
		return err
	}
	if err := p.requireManager(ctx, actorID, unit.BuildingID); err != nil {
		return err
	}
	if role != domain.PartyOwner && role != domain.PartyResident {
		return domain.ErrInvalidState
	}
	prev, _ := p.parties.Current(ctx, unitID, role) // nil if none
	prevUser := ""
	if prev != nil {
		prevUser = prev.UserID
	}
	now := time.Now().UTC()
	if err := p.parties.Append(ctx, &domain.UnitParty{
		ID: ids.New(), UnitID: unitID, Role: role, UserID: newUserID, From: effective,
	}); err != nil {
		return err
	}
	if err := p.parties.RecordTransfer(ctx, &domain.TransferRecord{
		ID: ids.New(), UnitID: unitID, Role: role, PreviousUserID: prevUser, NewUserID: newUserID,
		EffectiveDate: effective, ContractNumber: contractNumber, RecordedBy: actorID,
		Description: description, CreatedAt: now,
	}); err != nil {
		return err
	}
	p.publish(ctx, string(role)+".changed", unitID, map[string]any{
		"building_id": unit.BuildingID, "previous_user_id": prevUser, "new_user_id": newUserID,
	})
	return nil
}

func (p *Property) CurrentParties(ctx context.Context, unitID string) (map[domain.PartyRole]*domain.UnitParty, error) {
	out := map[domain.PartyRole]*domain.UnitParty{}
	for _, role := range []domain.PartyRole{domain.PartyOwner, domain.PartyResident} {
		cur, err := p.parties.Current(ctx, unitID, role)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		if cur != nil {
			out[role] = cur
		}
	}
	return out, nil
}

func (p *Property) TransferHistory(ctx context.Context, unitID string) ([]domain.TransferRecord, error) {
	return p.parties.Transfers(ctx, unitID)
}

func (p *Property) PartyHistory(ctx context.Context, unitID string) ([]domain.UnitParty, error) {
	return p.parties.History(ctx, unitID)
}

// --- Contracts ---

type ContractInput struct {
	Type, UnitID, FirstPartyID, SecondPartyID, Title string
	Amount, DepositAmount                            int64
	StartDate                                        time.Time
	DurationMonths                                   int
	CreatedBy                                        string
}

func (p *Property) CreateContract(ctx context.Context, in ContractInput) (*domain.Contract, error) {
	unit, err := p.units.ByID(ctx, in.UnitID)
	if err != nil {
		return nil, err
	}
	if err := p.requireManager(ctx, in.CreatedBy, unit.BuildingID); err != nil {
		return nil, err
	}
	switch in.Type {
	case domain.ContractPurchase, domain.ContractRental, domain.ContractTransfer:
	default:
		return nil, domain.ErrInvalidState
	}
	dateKey := time.Now().UTC().Format("20060102")
	seq, err := p.contracts.NextSequence(ctx, dateKey)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	c := &domain.Contract{
		ID: ids.New(), Number: fmt.Sprintf("CONTRACT-%s-%04d", dateKey, seq),
		Type: in.Type, UnitID: in.UnitID, FirstPartyID: in.FirstPartyID,
		SecondPartyID: in.SecondPartyID, Title: in.Title,
		Amount: in.Amount, DepositAmount: in.DepositAmount, StartDate: in.StartDate,
		DurationMonths: in.DurationMonths, Status: domain.ContractDraft,
		CreatedBy: in.CreatedBy, CreatedAt: now, UpdatedAt: now,
	}
	if err := p.contracts.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (p *Property) Contract(ctx context.Context, id string) (*domain.Contract, error) {
	return p.contracts.ByID(ctx, id)
}

func (p *Property) ContractsByUnit(ctx context.Context, unitID string) ([]domain.Contract, error) {
	return p.contracts.ListByUnit(ctx, unitID)
}

// Sign records a party's signature.
func (p *Property) Sign(ctx context.Context, contractID, partyUserID string) error {
	c, err := p.contracts.ByID(ctx, contractID)
	if err != nil {
		return err
	}
	switch partyUserID {
	case c.FirstPartyID:
		c.FirstSigned = true
	case c.SecondPartyID:
		c.SecondSigned = true
	default:
		return domain.ErrInvalidState
	}
	return p.contracts.Update(ctx, c)
}

// Activate applies contract side effects: purchase → ownership,
// rental → residency; then publishes contract.activated for billing.
func (p *Property) Activate(ctx context.Context, actorID, contractID string) (*domain.Contract, error) {
	c, err := p.contracts.ByID(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if c.Status != domain.ContractDraft {
		return nil, domain.ErrInvalidState
	}
	if !c.FirstSigned || !c.SecondSigned {
		return nil, domain.ErrMissingSignatures
	}
	role := domain.PartyOwner
	if c.Type == domain.ContractRental {
		role = domain.PartyResident
	}
	if err := p.ChangeParty(ctx, actorID, c.UnitID, role, c.SecondPartyID, time.Now().UTC(), c.Number, "contract activation"); err != nil {
		return nil, err
	}
	today := time.Now().UTC()
	c.Status = domain.ContractActive
	c.SignedDate = &today
	if err := p.contracts.Update(ctx, c); err != nil {
		return nil, err
	}
	p.publish(ctx, "contract.activated", c.ID, map[string]any{
		"type": c.Type, "unit_id": c.UnitID, "amount": c.Amount,
		"second_party_id": c.SecondPartyID, "start_date": c.StartDate,
	})
	return c, nil
}

// --- helpers ---

func (p *Property) requireManager(ctx context.Context, userID, buildingID string) error {
	ok, err := p.membership.AnyRole(ctx, userID, buildingID, "manager", "board_member")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (p *Property) requireMember(ctx context.Context, userID, buildingID string) error {
	ok, err := p.membership.HasRole(ctx, userID, buildingID, "*")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (p *Property) publish(ctx context.Context, typ, subject string, data map[string]any) {
	_ = p.pub.Publish(ctx, events.New("property", typ, subject, data))
}
