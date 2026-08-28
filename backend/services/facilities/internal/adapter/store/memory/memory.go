// Package memory provides in-memory facilities stores (dev/tests).
package memory

import (
	"context"
	"sync"
	"time"

	"unital/backend/services/facilities/internal/domain"
)

type core struct {
	mu     sync.Mutex
	facs   map[string]*domain.Facility
	books  map[string]*domain.Booking
	maints map[string]*domain.Maintenance
	roles  map[string]string
}

type Bundle struct {
	Facilities  *FacilityStore
	Bookings    *BookingStore
	Maintenance *MaintenanceStore
	Membership  *MembershipTable
}

func New() Bundle {
	c := &core{facs: map[string]*domain.Facility{}, books: map[string]*domain.Booking{}, maints: map[string]*domain.Maintenance{}, roles: map[string]string{}}
	return Bundle{
		Facilities:  &FacilityStore{c},
		Bookings:    &BookingStore{c},
		Maintenance: &MaintenanceStore{c},
		Membership:  &MembershipTable{c},
	}
}

// MembershipTable implements domain.MembershipChecker (seedable).
type MembershipTable struct{ *core }

func (m *MembershipTable) Seed(userID, buildingID, role string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roles[userID+"|"+buildingID] = role
}

func (m *MembershipTable) HasRole(_ context.Context, userID, buildingID, role string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	got, ok := m.roles[userID+"|"+buildingID]
	if !ok {
		return false, nil
	}
	return role == "*" || got == role, nil
}

func (m *MembershipTable) AnyRole(_ context.Context, userID, buildingID string, roles ...string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

type FacilityStore struct{ *core }

func (s *FacilityStore) Create(_ context.Context, f *domain.Facility) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.facs[f.ID] = f
	return nil
}

func (s *FacilityStore) ByID(_ context.Context, id string) (*domain.Facility, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.facs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return f, nil
}

func (s *FacilityStore) Update(_ context.Context, f *domain.Facility) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.facs[f.ID]; !ok {
		return domain.ErrNotFound
	}
	s.facs[f.ID] = f
	return nil
}

func (s *FacilityStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.facs, id)
	return nil
}

func (s *FacilityStore) ListByBuilding(_ context.Context, buildingID string) ([]domain.Facility, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Facility
	for _, f := range s.facs {
		if f.BuildingID == buildingID {
			out = append(out, *f)
		}
	}
	return out, nil
}

type BookingStore struct{ *core }

func (s *BookingStore) Create(_ context.Context, b *domain.Booking) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books[b.ID] = b
	return nil
}

func (s *BookingStore) ByID(_ context.Context, id string) (*domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (s *BookingStore) Update(_ context.Context, b *domain.Booking) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.books[b.ID]; !ok {
		return domain.ErrNotFound
	}
	s.books[b.ID] = b
	return nil
}

// Overlaps returns bookings overlapping [start,end) on the facility.
func (s *BookingStore) Overlaps(_ context.Context, facilityID string, start, end time.Time) ([]domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Booking
	for _, b := range s.books {
		if b.FacilityID == facilityID && b.Start.Before(end) && start.Before(b.End) {
			out = append(out, *b)
		}
	}
	return out, nil
}

func (s *BookingStore) ByUser(_ context.Context, userID string) ([]domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Booking
	for _, b := range s.books {
		if b.UserID == userID {
			out = append(out, *b)
		}
	}
	return out, nil
}

func (s *BookingStore) ByBuilding(_ context.Context, buildingID, status string) ([]domain.Booking, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Booking
	for _, b := range s.books {
		if b.BuildingID == buildingID && (status == "" || b.Status == status) {
			out = append(out, *b)
		}
	}
	return out, nil
}

type MaintenanceStore struct{ *core }

func (s *MaintenanceStore) Create(_ context.Context, m *domain.Maintenance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maints[m.ID] = m
	return nil
}

func (s *MaintenanceStore) ByID(_ context.Context, id string) (*domain.Maintenance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.maints[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m, nil
}

func (s *MaintenanceStore) Update(_ context.Context, m *domain.Maintenance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.maints[m.ID]; !ok {
		return domain.ErrNotFound
	}
	s.maints[m.ID] = m
	return nil
}

func (s *MaintenanceStore) Overlaps(_ context.Context, facilityID string, start, end time.Time) ([]domain.Maintenance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Maintenance
	for _, m := range s.maints {
		if m.FacilityID == facilityID && m.ScheduledStart.Before(end) && start.Before(m.ScheduledEnd) {
			out = append(out, *m)
		}
	}
	return out, nil
}
