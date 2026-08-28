// Package memory provides in-memory billing stores (dev & tests).
package memory

import (
	"context"
	"sync"

	"unital/backend/pkg/ids"
	"unital/backend/services/billing/internal/domain"
)

type core struct {
	mu        sync.RWMutex
	templates map[string]*domain.Template
	charges   map[string]*domain.Charge
	txs       map[string]*domain.Transaction
	invoices  map[string]*domain.Invoice
	roles     map[string]string // membership table "user|building" -> role
	units     map[string][]string
}

// Bundle groups all store implementations.
type Bundle struct {
	Templates  *TemplateStore
	Charges    *ChargeStore
	Txs        *TxStore
	Invoices   *InvoiceStore
	Membership *MembershipTable
	Directory  *UnitDirectory
}

func New() Bundle {
	c := &core{
		templates: map[string]*domain.Template{},
		charges:   map[string]*domain.Charge{},
		txs:       map[string]*domain.Transaction{},
		invoices:  map[string]*domain.Invoice{},
		roles:     map[string]string{},
		units:     map[string][]string{},
	}
	return Bundle{
		Templates:  &TemplateStore{c},
		Charges:    &ChargeStore{c},
		Txs:        &TxStore{c},
		Invoices:   &InvoiceStore{c},
		Membership: &MembershipTable{c},
		Directory:  &UnitDirectory{c},
	}
}

// --- MembershipTable implements domain.MembershipChecker ---

type MembershipTable struct{ *core }

func (m *MembershipTable) Seed(userID, buildingID, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[userID+"|"+buildingID] = role
}

func (m *MembershipTable) HasRole(_ context.Context, userID, buildingID, role string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	got, ok := m.roles[userID+"|"+buildingID]
	if !ok {
		return false, nil
	}
	return role == "*" || got == role, nil
}

func (m *MembershipTable) AnyRole(_ context.Context, userID, buildingID string, roles ...string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	got, ok := m.roles[userID+"|"+buildingID]
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

// --- UnitDirectory implements domain.UnitDirectory (seedable) ---

type UnitDirectory struct{ *core }

func (d *UnitDirectory) SeedUnits(buildingID string, unitIDs ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.units[buildingID] = append(d.units[buildingID], unitIDs...)
}

func (d *UnitDirectory) UnitIDs(_ context.Context, buildingID string) ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]string(nil), d.units[buildingID]...), nil
}

// --- TemplateStore ---

type TemplateStore struct{ *core }

func (s *TemplateStore) Create(_ context.Context, t *domain.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templates[t.ID] = t
	return nil
}

func (s *TemplateStore) ByID(_ context.Context, id string) (*domain.Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.templates[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *TemplateStore) Update(_ context.Context, t *domain.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.templates[t.ID]; !ok {
		return domain.ErrNotFound
	}
	s.templates[t.ID] = t
	return nil
}

func (s *TemplateStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.templates, id)
	return nil
}

func (s *TemplateStore) ListByBuilding(_ context.Context, buildingID string) ([]domain.Template, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Template
	for _, t := range s.templates {
		if t.BuildingID == buildingID {
			out = append(out, *t)
		}
	}
	return out, nil
}

// --- ChargeStore ---

type ChargeStore struct{ *core }

// Upsert is idempotent on (unit, template, period): if a charge exists
// it is returned unchanged (created=false).
func (s *ChargeStore) Upsert(_ context.Context, c *domain.Charge) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.charges {
		if e.UnitID == c.UnitID && e.TemplateID == c.TemplateID && e.Period == c.Period {
			return false, nil
		}
	}
	s.charges[c.ID] = c
	return true, nil
}

func (s *ChargeStore) ByID(_ context.Context, id string) (*domain.Charge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.charges[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (s *ChargeStore) List(_ context.Context, f domain.ChargeFilter) ([]domain.Charge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Charge
	for _, c := range s.charges {
		if f.BuildingID != "" && c.BuildingID != f.BuildingID {
			continue
		}
		if f.UnitID != "" && c.UnitID != f.UnitID {
			continue
		}
		if f.Period != "" && c.Period != f.Period {
			continue
		}
		if f.Status != "" && c.Status != f.Status {
			continue
		}
		out = append(out, *c)
	}
	return out, nil
}

func (s *ChargeStore) Update(_ context.Context, c *domain.Charge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.charges[c.ID]; !ok {
		return domain.ErrNotFound
	}
	s.charges[c.ID] = c
	return nil
}

// --- TxStore ---

type TxStore struct{ *core }

func (s *TxStore) Create(_ context.Context, tx *domain.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tx.ID == "" {
		tx.ID = ids.New()
	}
	s.txs[tx.ID] = tx
	return nil
}

func (s *TxStore) ByID(_ context.Context, id string) (*domain.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tx, ok := s.txs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return tx, nil
}

func (s *TxStore) Update(_ context.Context, tx *domain.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.txs[tx.ID]; !ok {
		return domain.ErrNotFound
	}
	s.txs[tx.ID] = tx
	return nil
}

// --- InvoiceStore ---

type InvoiceStore struct{ *core }

func invoiceKey(buildingID, unitID, period string) string {
	return buildingID + "|" + unitID + "|" + period
}

func (s *InvoiceStore) GetOrCreate(_ context.Context, buildingID, unitID, period string) (*domain.Invoice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := invoiceKey(buildingID, unitID, period)
	if inv, ok := s.invoices[key]; ok {
		return inv, nil
	}
	inv := &domain.Invoice{ID: ids.New(), BuildingID: buildingID, UnitID: unitID, Period: period}
	s.invoices[key] = inv
	return inv, nil
}

func (s *InvoiceStore) Save(_ context.Context, inv *domain.Invoice) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.invoices[invoiceKey(inv.BuildingID, inv.UnitID, inv.Period)] = inv
	return nil
}

func (s *InvoiceStore) List(_ context.Context, buildingID string, unitID *string) ([]domain.Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Invoice
	for _, inv := range s.invoices {
		if inv.BuildingID != buildingID {
			continue
		}
		if unitID != nil && inv.UnitID != *unitID {
			continue
		}
		out = append(out, *inv)
	}
	return out, nil
}
