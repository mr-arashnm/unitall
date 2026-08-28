// Package usecase implements facilities booking, availability, and
// maintenance workflows.
package usecase

import (
	"context"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/ids"
	"unital/backend/services/facilities/internal/domain"
)

type Facilities struct {
	fac    domain.FacilityStore
	books  domain.BookingStore
	maint  domain.MaintenanceStore
	member domain.MembershipChecker
	pub    events.Publisher
	now    func() time.Time
}

func New(f domain.FacilityStore, b domain.BookingStore, m domain.MaintenanceStore,
	mc domain.MembershipChecker, pub events.Publisher) *Facilities {
	return &Facilities{fac: f, books: b, maint: m, member: mc, pub: pub, now: time.Now}
}

// --- facilities ---

func (u *Facilities) CreateFacility(ctx context.Context, actorID string, f *domain.Facility) (*domain.Facility, error) {
	if err := u.requireManager(ctx, actorID, f.BuildingID); err != nil {
		return nil, err
	}
	if f.Name == "" || f.Capacity <= 0 {
		return nil, domain.ErrInvalidState
	}
	if f.MinAdvanceH == 0 {
		f.MinAdvanceH = 1
	}
	if f.MaxAdvanceH == 0 {
		f.MaxAdvanceH = 168
	}
	f.ID = ids.New()
	f.IsActive = true
	f.CreatedAt = u.now().UTC()
	if err := u.fac.Create(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (u *Facilities) Facility(ctx context.Context, id string) (*domain.Facility, error) {
	return u.fac.ByID(ctx, id)
}

func (u *Facilities) UpdateFacility(ctx context.Context, actorID, id string, patch func(*domain.Facility)) (*domain.Facility, error) {
	f, err := u.fac.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := u.requireManager(ctx, actorID, f.BuildingID); err != nil {
		return nil, err
	}
	patch(f)
	if err := u.fac.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (u *Facilities) DeleteFacility(ctx context.Context, actorID, id string) error {
	f, err := u.fac.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := u.requireManager(ctx, actorID, f.BuildingID); err != nil {
		return err
	}
	return u.fac.Delete(ctx, id)
}

func (u *Facilities) List(ctx context.Context, actorID, buildingID string) ([]domain.Facility, error) {
	ok, err := u.member.HasRole(ctx, actorID, buildingID, "*")
	if err != nil || !ok {
		return nil, domain.ErrForbidden
	}
	return u.fac.ListByBuilding(ctx, buildingID)
}

// --- availability ---

// Availability returns free hour slots for a facility on the given day,
// honoring opening hours, existing bookings, and maintenance windows.
func (u *Facilities) Availability(ctx context.Context, facilityID string, day time.Time) ([]int, error) {
	f, err := u.fac.ByID(ctx, facilityID)
	if err != nil {
		return nil, err
	}
	open, close_ := parseHour(f.OpeningTime, 8), parseHour(f.ClosingTime, 22)
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	overlaps := func(s, e time.Time) (bool, error) {
		bs, err := u.books.Overlaps(ctx, facilityID, s, e)
		if err != nil {
			return false, err
		}
		for _, b := range bs {
			if b.Status == domain.BookingPending || b.Status == domain.BookingConfirmed {
				return true, nil
			}
		}
		ms, err := u.maint.Overlaps(ctx, facilityID, s, e)
		if err != nil {
			return false, err
		}
		return len(ms) > 0, nil
	}
	var free []int
	for h := open; h < close_; h++ {
		s, e := day.Add(time.Duration(h)*time.Hour), day.Add(time.Duration(h+1)*time.Hour)
		busy, err := overlaps(s, e)
		if err != nil {
			return nil, err
		}
		if !busy {
			free = append(free, h)
		}
	}
	return free, nil
}

// --- bookings ---

type BookingInput struct {
	FacilityID, UserID, Purpose string
	Start, End                  time.Time
	Participants                int
}

// Create validates the full rule set then stores a pending booking.
func (u *Facilities) Book(ctx context.Context, in BookingInput) (*domain.Booking, error) {
	f, err := u.fac.ByID(ctx, in.FacilityID)
	if err != nil {
		return nil, err
	}
	ok, err := u.member.HasRole(ctx, in.UserID, f.BuildingID, "*")
	if err != nil || !ok {
		return nil, domain.ErrForbidden
	}
	if !f.IsActive {
		return nil, domain.ErrUnderMaintenance
	}
	if in.Participants < 1 || in.Participants > f.Capacity {
		return nil, domain.ErrOverCapacity
	}
	now := u.now()
	lead := in.Start.Sub(now)
	if lead < time.Duration(f.MinAdvanceH)*time.Hour {
		return nil, domain.ErrTooSoon
	}
	if lead > time.Duration(f.MaxAdvanceH)*time.Hour {
		return nil, domain.ErrTooFar
	}
	open, close_ := parseHour(f.OpeningTime, 8), parseHour(f.ClosingTime, 22)
	if in.Start.Hour() < open || in.End.Hour() > close_ {
		return nil, domain.ErrClosed
	}
	bs, err := u.books.Overlaps(ctx, f.ID, in.Start, in.End)
	if err != nil {
		return nil, err
	}
	for _, b := range bs {
		if b.Status == domain.BookingPending || b.Status == domain.BookingConfirmed {
			return nil, domain.ErrConflict
		}
	}
	ms, err := u.maint.Overlaps(ctx, f.ID, in.Start, in.End)
	if err != nil {
		return nil, err
	}
	if len(ms) > 0 {
		return nil, domain.ErrConflict
	}
	hours := int64(in.End.Sub(in.Start).Hours())
	b := &domain.Booking{
		ID: ids.New(), FacilityID: f.ID, BuildingID: f.BuildingID, UserID: in.UserID,
		Start: in.Start, End: in.End, Purpose: in.Purpose,
		Participants: in.Participants, Status: domain.BookingPending,
		TotalCost: hours * f.HourlyRate, CreatedAt: now.UTC(),
	}
	if err := u.books.Create(ctx, b); err != nil {
		return nil, err
	}
	u.publish(ctx, "booking.created", b.ID, map[string]any{"facility_id": f.ID, "user_id": in.UserID})
	return b, nil
}

func (u *Facilities) MyBookings(ctx context.Context, userID string) ([]domain.Booking, error) {
	return u.books.ByUser(ctx, userID)
}

// Booking fetches one booking for detail views.
func (u *Facilities) Booking(ctx context.Context, id string) (*domain.Booking, error) {
	return u.books.ByID(ctx, id)
}

// Decide applies approve/reject/cancel with state-machine rules.
func (u *Facilities) Decide(ctx context.Context, actorID, bookingID, action string) (*domain.Booking, error) {
	b, err := u.books.ByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	switch action {
	case "approve", "reject":
		if err := u.requireManager(ctx, actorID, b.BuildingID); err != nil {
			return nil, err
		}
		if b.Status != domain.BookingPending {
			return nil, domain.ErrInvalidState
		}
		if action == "approve" {
			b.Status = domain.BookingConfirmed
			b.ApprovedBy = actorID
		} else {
			b.Status = domain.BookingRejected
		}
	case "cancel":
		if b.UserID != actorID {
			if err := u.requireManager(ctx, actorID, b.BuildingID); err != nil {
				return nil, err
			}
		}
		if b.Status == domain.BookingCancelled || b.Status == domain.BookingCompleted {
			return nil, domain.ErrInvalidState
		}
		b.Status = domain.BookingCancelled
	default:
		return nil, domain.ErrInvalidState
	}
	if err := u.books.Update(ctx, b); err != nil {
		return nil, err
	}
	u.publish(ctx, "booking."+b.Status, b.ID, map[string]any{"user_id": b.UserID})
	return b, nil
}

// --- maintenance ---

func (u *Facilities) ScheduleMaintenance(ctx context.Context, actorID string, m *domain.Maintenance) (*domain.Maintenance, error) {
	f, err := u.fac.ByID(ctx, m.FacilityID)
	if err != nil {
		return nil, err
	}
	if err := u.requireManager(ctx, actorID, f.BuildingID); err != nil {
		return nil, err
	}
	if !m.ScheduledEnd.After(m.ScheduledStart) {
		return nil, domain.ErrInvalidState
	}
	m.ID = ids.New()
	m.Status = "scheduled"
	m.CreatedBy = actorID
	if err := u.maint.Create(ctx, m); err != nil {
		return nil, err
	}
	if m.AffectBookings {
		bs, err := u.books.Overlaps(ctx, f.ID, m.ScheduledStart, m.ScheduledEnd)
		if err != nil {
			return nil, err
		}
		for _, b := range bs {
			if b.Status == domain.BookingPending || b.Status == domain.BookingConfirmed {
				b.Status = domain.BookingCancelled
				_ = u.books.Update(ctx, &b)
			}
		}
	}
	u.publish(ctx, "facility.maintenance_scheduled", m.ID, map[string]any{"facility_id": f.ID})
	return m, nil
}

func (u *Facilities) MaintenanceAction(ctx context.Context, actorID, id, action string) (*domain.Maintenance, error) {
	m, err := u.maint.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	f, err := u.fac.ByID(ctx, m.FacilityID)
	if err != nil {
		return nil, err
	}
	if err := u.requireManager(ctx, actorID, f.BuildingID); err != nil {
		return nil, err
	}
	now := u.now().UTC()
	switch action {
	case "start":
		if m.Status != "scheduled" {
			return nil, domain.ErrInvalidState
		}
		m.Status = "in_progress"
		m.ActualStart = &now
	case "complete":
		if m.Status != "in_progress" {
			return nil, domain.ErrInvalidState
		}
		m.Status = "done"
		m.ActualEnd = &now
	default:
		return nil, domain.ErrInvalidState
	}
	if err := u.maint.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (u *Facilities) requireManager(ctx context.Context, userID, buildingID string) error {
	ok, err := u.member.AnyRole(ctx, userID, buildingID, "manager", "board_member")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (u *Facilities) publish(ctx context.Context, typ, subject string, data map[string]any) {
	_ = u.pub.Publish(ctx, events.New("facilities", typ, subject, data))
}

// parseHour extracts the hour from "HH:MM"; empty string falls back to def.
func parseHour(s string, def int) int {
	if s == "" {
		return def
	}
	if t, err := time.Parse("15:04", s); err == nil {
		return t.Hour()
	}
	return def
}
