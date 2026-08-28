// Package memory provides in-memory property stores and a membership
// checker (dev adapter; production uses identity's internal API).
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"unital/backend/pkg/ids"
	"unital/backend/services/property/internal/domain"
)

// core is the shared data graph all stores operate on.
type core struct {
	mu        sync.RWMutex
	buildings map[string]*domain.Building
	units     map[string]*domain.Unit
	assets    map[string]*domain.Asset
	parties   []*domain.UnitParty
	transfers []*domain.TransferRecord
	contracts map[string]*domain.Contract
	sequences map[string]int
}

func newCore() *core {
	return &core{
		buildings: map[string]*domain.Building{},
		units:     map[string]*domain.Unit{},
		assets:    map[string]*domain.Asset{},
		contracts: map[string]*domain.Contract{},
		sequences: map[string]int{},
	}
}

// Bundle wires every store over one core.
type Bundle struct {
	Buildings  *BuildingStore
	Units      *UnitStore
	Assets     *AssetStore
	Parties    *PartyStore
	Contracts  *ContractStore
	Membership *MembershipTable
}

func New() Bundle {
	c := newCore()
	return Bundle{
		Buildings:  &BuildingStore{c},
		Units:      &UnitStore{c},
		Assets:     &AssetStore{c},
		Parties:    &PartyStore{c},
		Contracts:  &ContractStore{c},
		Membership: &MembershipTable{roles: map[string]string{}},
	}
}

// --- MembershipTable implements domain.MembershipChecker ---

type MembershipTable struct {
	mu    sync.RWMutex
	roles map[string]string // "user|building" -> role
}

func (m *MembershipTable) Seed(userID, buildingID, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[userAndBuilding(userID, buildingID)] = role
}

func (m *MembershipTable) HasRole(_ context.Context, userID, buildingID, role string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	got, ok := m.roles[userAndBuilding(userID, buildingID)]
	if !ok {
		return false, nil
	}
	return role == "*" || got == role, nil
}

func (m *MembershipTable) AnyRole(_ context.Context, userID, buildingID string, roles ...string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	got, ok := m.roles[userAndBuilding(userID, buildingID)]
	if !ok {
		return false, nil
	}
	for _, r := range roles {
		if got == r || r == "*" {
			return true, nil
		}
	}
	return false, nil
}

func userAndBuilding(u, b string) string { return u + "|" + b }

// --- BuildingStore ---

type BuildingStore struct{ *core }

func (s *BuildingStore) Create(_ context.Context, b *domain.Building) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.buildings {
		if e.Code == b.Code {
			return domain.ErrCodeTaken
		}
	}
	s.buildings[b.ID] = b
	return nil
}

func (s *BuildingStore) ByID(_ context.Context, id string) (*domain.Building, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.buildings[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (s *BuildingStore) Update(_ context.Context, b *domain.Building) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.buildings[b.ID]; !ok {
		return domain.ErrNotFound
	}
	s.buildings[b.ID] = b
	return nil
}

func (s *BuildingStore) ListByUser(_ context.Context, userID string) ([]domain.Building, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Building
	for _, b := range s.buildings {
		if b.CreatedBy == userID || s.userInBuilding(userID, b.ID) {
			out = append(out, *b)
		}
	}
	return out, nil
}

// userInBuilding reports whether the user manages the building or holds
// a current party row on one of its units.
func (s *core) userInBuilding(userID, buildingID string) bool {
	for _, p := range s.parties {
		if p.UserID != userID || p.To != nil {
			continue
		}
		if u, ok := s.units[p.UnitID]; ok && u.BuildingID == buildingID {
			return true
		}
	}
	return false
}

// --- UnitStore ---

type UnitStore struct{ *core }

func (s *UnitStore) Create(_ context.Context, u *domain.Unit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.units {
		if e.BuildingID == u.BuildingID && e.Number == u.Number {
			return domain.ErrCodeTaken
		}
	}
	s.units[u.ID] = u
	return nil
}

func (s *UnitStore) ByID(_ context.Context, id string) (*domain.Unit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.units[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (s *UnitStore) Update(_ context.Context, u *domain.Unit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.units[u.ID]; !ok {
		return domain.ErrNotFound
	}
	s.units[u.ID] = u
	return nil
}

func (s *UnitStore) ListByBuilding(_ context.Context, buildingID string) ([]domain.Unit, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Unit
	for _, u := range s.units {
		if u.BuildingID == buildingID {
			out = append(out, *u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Floor != out[j].Floor {
			return out[i].Floor < out[j].Floor
		}
		return out[i].Number < out[j].Number
	})
	return out, nil
}

func (s *UnitStore) CountByBuilding(_ context.Context, buildingID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, u := range s.units {
		if u.BuildingID == buildingID {
			n++
		}
	}
	return n, nil
}

// --- AssetStore ---

type AssetStore struct{ *core }

func (s *AssetStore) Create(_ context.Context, a *domain.Asset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.assets {
		if e.Kind == a.Kind && e.BuildingID == a.BuildingID && e.Code == a.Code {
			return domain.ErrCodeTaken
		}
	}
	s.assets[a.ID] = a
	return nil
}

func (s *AssetStore) ByID(_ context.Context, id string) (*domain.Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.assets[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (s *AssetStore) Update(_ context.Context, a *domain.Asset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.assets[a.ID]; !ok {
		return domain.ErrNotFound
	}
	// If code changed, ensure uniqueness within the same kind+building
	for _, e := range s.assets {
		if e.ID == a.ID {
			continue
		}
		if e.Kind == a.Kind && e.BuildingID == a.BuildingID && e.Code == a.Code {
			return domain.ErrCodeTaken
		}
	}
	s.assets[a.ID] = a
	return nil
}

func (s *AssetStore) ByCode(_ context.Context, kind domain.AssetKind, buildingID, code string) (*domain.Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.assets {
		if a.Kind == kind && a.BuildingID == buildingID && a.Code == code {
			return a, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (s *AssetStore) ListByBuilding(_ context.Context, buildingID string, kind domain.AssetKind) ([]domain.Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Asset
	for _, a := range s.assets {
		if a.BuildingID == buildingID && a.Kind == kind {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (s *AssetStore) Assign(_ context.Context, assetID, unitID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assets[assetID]
	if !ok {
		return domain.ErrNotFound
	}
	a.UnitID = unitID
	return nil
}

func (s *AssetStore) Release(_ context.Context, assetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assets[assetID]
	if !ok {
		return domain.ErrNotFound
	}
	a.UnitID = ""
	return nil
}

// --- PartyStore ---

type PartyStore struct{ *core }

func (s *PartyStore) Current(_ context.Context, unitID string, role domain.PartyRole) (*domain.UnitParty, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.parties) - 1; i >= 0; i-- {
		p := s.parties[i]
		if p.UnitID == unitID && p.Role == role && p.To == nil {
			return p, nil
		}
	}
	return nil, domain.ErrNotFound
}

// Append closes any current party row for the unit+role, then adds p.
func (s *PartyStore) Append(_ context.Context, p *domain.UnitParty) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for _, e := range s.parties {
		if e.UnitID == p.UnitID && e.Role == p.Role && e.To == nil {
			closed := now
			e.To = &closed
		}
	}
	s.parties = append(s.parties, p)
	return nil
}

func (s *PartyStore) History(_ context.Context, unitID string) ([]domain.UnitParty, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.UnitParty
	for _, p := range s.parties {
		if p.UnitID == unitID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (s *PartyStore) RecordTransfer(_ context.Context, rec *domain.TransferRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rec.ID == "" {
		rec.ID = ids.New()
	}
	s.transfers = append(s.transfers, rec)
	return nil
}

func (s *PartyStore) Transfers(_ context.Context, unitID string) ([]domain.TransferRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.TransferRecord
	for _, r := range s.transfers {
		if r.UnitID == unitID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (s *PartyStore) UnitIDsByUser(_ context.Context, userID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, p := range s.parties {
		if p.UserID == userID && !seen[p.UnitID] {
			seen[p.UnitID] = true
			out = append(out, p.UnitID)
		}
	}
	return out, nil
}

// --- ContractStore ---

type ContractStore struct{ *core }

func (s *ContractStore) Create(_ context.Context, c *domain.Contract) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contracts[c.ID] = c
	return nil
}

func (s *ContractStore) ByID(_ context.Context, id string) (*domain.Contract, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.contracts[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (s *ContractStore) Update(_ context.Context, c *domain.Contract) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.contracts[c.ID]; !ok {
		return domain.ErrNotFound
	}
	s.contracts[c.ID] = c
	return nil
}

func (s *ContractStore) ListByUnit(_ context.Context, unitID string) ([]domain.Contract, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Contract
	for _, c := range s.contracts {
		if c.UnitID == unitID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (s *ContractStore) NextSequence(_ context.Context, dateKey string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequences[dateKey]++
	return s.sequences[dateKey], nil
}
