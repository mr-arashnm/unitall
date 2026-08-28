// Communications usecases: announcements (with inbox fan-out), meetings
// (RSVP + minutes), and support tickets (threaded responses).
package usecase

import (
	"context"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/ids"
	"unital/backend/services/notifications/internal/domain"
)

type Comms struct {
	announcements domain.AnnouncementStore
	meetings      domain.MeetingStore
	attendance    domain.AttendanceStore
	minutes       domain.MinuteStore
	tickets       domain.TicketStore
	responses     domain.TicketResponseStore
	inbox         domain.InboxStore
	resolver      domain.RecipientResolver
	membership    domain.MembershipChecker
	pub           events.Publisher
	now           func() time.Time
}

func NewComms(a domain.AnnouncementStore, m domain.MeetingStore, att domain.AttendanceStore,
	min domain.MinuteStore, t domain.TicketStore, tr domain.TicketResponseStore,
	inbox domain.InboxStore, r domain.RecipientResolver, mem domain.MembershipChecker,
	pub events.Publisher) *Comms {
	return &Comms{
		announcements: a, meetings: m, attendance: att, minutes: min,
		tickets: t, responses: tr, inbox: inbox, resolver: r, membership: mem,
		pub: pub, now: time.Now,
	}
}

// --- announcements ---

func (c *Comms) CreateAnnouncement(ctx context.Context, actorID string, a *domain.Announcement) (*domain.Announcement, error) {
	if err := c.requireManager(ctx, actorID, a.BuildingID); err != nil {
		return nil, err
	}
	if a.Title == "" {
		return nil, domain.ErrInvalidState
	}
	if a.Priority == "" {
		a.Priority = "normal"
	}
	if a.Target.Kind == "" {
		a.Target.Kind = domain.TargetAll
	}
	a.ID = ids.New()
	a.Status = domain.AnnDraft
	a.CreatedBy = actorID
	a.CreatedAt = c.now().UTC()
	if err := c.announcements.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (c *Comms) Announcement(ctx context.Context, id string) (*domain.Announcement, error) {
	return c.announcements.ByID(ctx, id)
}

func (c *Comms) PatchAnnouncement(ctx context.Context, actorID, id string, patch func(*domain.Announcement)) (*domain.Announcement, error) {
	a, err := c.announcements.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.requireManager(ctx, actorID, a.BuildingID); err != nil {
		return nil, err
	}
	if a.Status == domain.AnnPublished {
		return nil, domain.ErrInvalidState // published announcements are immutable
	}
	patch(a)
	if err := c.announcements.Update(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (c *Comms) DeleteAnnouncement(ctx context.Context, actorID, id string) error {
	a, err := c.announcements.ByID(ctx, id)
	if err != nil {
		return err
	}
	if err := c.requireManager(ctx, actorID, a.BuildingID); err != nil {
		return err
	}
	return c.announcements.Delete(ctx, id)
}

// Announcements lists what the actor may see: managers get everything
// (drafts included); members get published, unexpired, targeted ones.
func (c *Comms) Announcements(ctx context.Context, actorID, buildingID string) ([]domain.Announcement, error) {
	if err := c.requireMember(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	all, err := c.announcements.ListByBuilding(ctx, buildingID)
	if err != nil {
		return nil, err
	}
	isMgr, err := c.isStaff(ctx, actorID, buildingID)
	if err != nil {
		return nil, err
	}
	now := c.now()
	out := make([]domain.Announcement, 0, len(all))
	for _, a := range all {
		if isMgr {
			out = append(out, a)
			continue
		}
		if a.Status != domain.AnnPublished {
			continue
		}
		if a.ExpiresAt != nil && a.ExpiresAt.Before(now) {
			continue
		}
		if c.visibleTo(ctx, actorID, buildingID, a.Target) {
			out = append(out, a)
		}
	}
	return out, nil
}

// Publish flips a draft to published and pushes it to every targeted
// recipient's in-app inbox (email/SMS fan-out can piggyback on campaigns
// once the property internal API resolves unit targets).
func (c *Comms) Publish(ctx context.Context, actorID, id string) (*domain.Announcement, error) {
	a, err := c.announcements.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.requireManager(ctx, actorID, a.BuildingID); err != nil {
		return nil, err
	}
	if a.Status != domain.AnnDraft {
		return nil, domain.ErrInvalidState
	}
	recipients, err := c.resolver.Resolve(ctx, a.BuildingID, a.Target)
	if err != nil {
		return nil, err
	}
	now := c.now().UTC()
	for _, r := range recipients {
		_ = c.inbox.Push(ctx, &domain.InboxMessage{
			UserID: r.ID, Title: a.Title, Body: a.Content,
			CampaignID: a.ID, CreatedAt: now,
		})
	}
	a.Status = domain.AnnPublished
	a.PublishAt = &now
	a.DeliveredTo = len(recipients)
	if err := c.announcements.Update(ctx, a); err != nil {
		return nil, err
	}
	c.publish(ctx, "announcement.published", a.ID, map[string]any{
		"building_id": a.BuildingID, "recipients": a.DeliveredTo,
	})
	return a, nil
}

// visibleTo reports whether a target applies to the actor. Unit targets
// degrade to "all members" until the property internal API lands.
func (c *Comms) visibleTo(ctx context.Context, actorID, buildingID string, t domain.Target) bool {
	switch t.Kind {
	case domain.TargetAll, domain.TargetUnits, "":
		return true
	case domain.TargetRole:
		ok, err := c.membership.AnyRole(ctx, actorID, buildingID, t.Values...)
		return err == nil && ok
	case domain.TargetUsers:
		for _, v := range t.Values {
			if v == actorID {
				return true
			}
		}
	}
	return false
}

// --- meetings ---

func (c *Comms) CreateMeeting(ctx context.Context, actorID string, m *domain.Meeting) (*domain.Meeting, error) {
	if err := c.requireManager(ctx, actorID, m.BuildingID); err != nil {
		return nil, err
	}
	if m.Title == "" || m.ScheduledAt.IsZero() {
		return nil, domain.ErrInvalidState
	}
	if m.Type == "" {
		m.Type = "general"
	}
	if m.DurationMin == 0 {
		m.DurationMin = 60
	}
	if m.Target.Kind == "" {
		m.Target.Kind = domain.TargetAll
	}
	m.ID = ids.New()
	m.Status = domain.MeetScheduled
	m.CreatedBy = actorID
	m.CreatedAt = c.now().UTC()
	if err := c.meetings.Create(ctx, m); err != nil {
		return nil, err
	}
	c.publish(ctx, "meeting.scheduled", m.ID, map[string]any{"building_id": m.BuildingID})
	return m, nil
}

func (c *Comms) PatchMeeting(ctx context.Context, actorID, id string, patch func(*domain.Meeting)) (*domain.Meeting, error) {
	m, err := c.meetings.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.requireManager(ctx, actorID, m.BuildingID); err != nil {
		return nil, err
	}
	patch(m)
	if err := c.meetings.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Comms) Meeting(ctx context.Context, id string) (*domain.Meeting, error) {
	return c.meetings.ByID(ctx, id)
}

// Meetings lists what the actor may see: managers/staff see every
// meeting; other members only general and emergency sessions (board and
// committee meetings are private — mirrors the Django behaviour).
func (c *Comms) Meetings(ctx context.Context, actorID, buildingID string) ([]domain.Meeting, error) {
	if err := c.requireMember(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	all, err := c.meetings.ListByBuilding(ctx, buildingID)
	if err != nil {
		return nil, err
	}
	isStaff, err := c.isStaff(ctx, actorID, buildingID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Meeting, 0, len(all))
	for _, m := range all {
		if isStaff || m.Type == "general" || m.Type == "emergency" {
			out = append(out, m)
		}
	}
	return out, nil
}

// RSVP records (or updates) the actor's attendance status.
func (c *Comms) RSVP(ctx context.Context, actorID, meetingID, status string) (*domain.Attendance, error) {
	m, err := c.meetings.ByID(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	if err := c.requireMember(ctx, actorID, m.BuildingID); err != nil {
		return nil, err
	}
	switch status {
	case domain.RsvpConfirmed, domain.RsvpDeclined, domain.RsvpAttended, domain.RsvpAbsent:
	default:
		return nil, domain.ErrInvalidState
	}
	now := c.now().UTC()
	att := &domain.Attendance{ID: ids.New(), MeetingID: meetingID, UserID: actorID, Status: status, RespondedAt: &now, CreatedAt: now}
	if err := c.attendance.Upsert(ctx, att); err != nil {
		return nil, err
	}
	return att, nil
}

func (c *Comms) Attendance(ctx context.Context, actorID, meetingID string) ([]domain.Attendance, error) {
	m, err := c.meetings.ByID(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	if err := c.requireManager(ctx, actorID, m.BuildingID); err != nil {
		return nil, err
	}
	return c.attendance.ByMeeting(ctx, meetingID)
}

func (c *Comms) UpsertMinutes(ctx context.Context, actorID string, m *domain.Minute) (*domain.Minute, error) {
	meet, err := c.meetings.ByID(ctx, m.MeetingID)
	if err != nil {
		return nil, err
	}
	if err := c.requireManager(ctx, actorID, meet.BuildingID); err != nil {
		return nil, err
	}
	if m.Content == "" {
		return nil, domain.ErrInvalidState
	}
	now := c.now().UTC()
	if existing, err := c.minutes.ByMeeting(ctx, m.MeetingID); err == nil {
		m.ID = existing.ID
		m.CreatedBy = existing.CreatedBy
		m.CreatedAt = existing.CreatedAt
		m.SignedBy = existing.SignedBy
	} else {
		m.ID = ids.New()
		m.CreatedBy = actorID
		m.CreatedAt = now
		m.SignedBy = []string{}
	}
	m.UpdatedAt = now
	if err := c.minutes.Upsert(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *Comms) Minutes(ctx context.Context, meetingID string) (*domain.Minute, error) {
	return c.minutes.ByMeeting(ctx, meetingID)
}

// SignMinutes appends the actor's signature (idempotent per user).
func (c *Comms) SignMinutes(ctx context.Context, actorID, meetingID string) (*domain.Minute, error) {
	m, err := c.minutes.ByMeeting(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	meet, err := c.meetings.ByID(ctx, meetingID)
	if err != nil {
		return nil, err
	}
	if err := c.requireMember(ctx, actorID, meet.BuildingID); err != nil {
		return nil, err
	}
	for _, s := range m.SignedBy {
		if s == actorID {
			return m, nil
		}
	}
	m.SignedBy = append(m.SignedBy, actorID)
	m.UpdatedAt = c.now().UTC()
	if err := c.minutes.Upsert(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// --- support tickets ---

func (c *Comms) SubmitTicket(ctx context.Context, actorID string, t *domain.Ticket) (*domain.Ticket, error) {
	if err := c.requireMember(ctx, actorID, t.BuildingID); err != nil {
		return nil, err
	}
	if t.Title == "" || t.UnitID == "" {
		return nil, domain.ErrInvalidState
	}
	if t.Type == "" {
		t.Type = "general"
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	t.ID = ids.New()
	t.SubmittedBy = actorID
	t.Status = domain.TicketOpen
	t.SubmittedAt = c.now().UTC()
	if err := c.tickets.Create(ctx, t); err != nil {
		return nil, err
	}
	c.publish(ctx, "ticket.created", t.ID, map[string]any{"building_id": t.BuildingID})
	return t, nil
}

// Tickets scopes visibility: managers/staff see the building's tickets,
// everyone else only their own.
func (c *Comms) Tickets(ctx context.Context, actorID, buildingID string, f domain.TicketFilter) ([]domain.Ticket, error) {
	f.BuildingID = buildingID
	isStaff, err := c.isStaff(ctx, actorID, buildingID)
	if err != nil {
		return nil, err
	}
	if !isStaff {
		f.SubmittedBy = actorID
	}
	return c.tickets.List(ctx, f)
}

func (c *Comms) Ticket(ctx context.Context, actorID, id string) (*domain.Ticket, []domain.TicketResponse, error) {
	t, err := c.tickets.ByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	isStaff, err := c.isStaff(ctx, actorID, t.BuildingID)
	if err != nil {
		return nil, nil, err
	}
	if !isStaff && t.SubmittedBy != actorID {
		return nil, nil, domain.ErrForbidden
	}
	rs, err := c.responses.ByTicket(ctx, id)
	return t, rs, err
}

// Respond adds a thread reply. A staff reply to an open ticket also
// triages it: status in_progress and assignment to the responder.
func (c *Comms) Respond(ctx context.Context, actorID, ticketID, message string, internal bool) (*domain.TicketResponse, error) {
	t, err := c.tickets.ByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	isStaff, err := c.isStaff(ctx, actorID, t.BuildingID)
	if err != nil {
		return nil, err
	}
	if !isStaff && t.SubmittedBy != actorID {
		return nil, domain.ErrForbidden
	}
	if message == "" {
		return nil, domain.ErrInvalidState
	}
	now := c.now().UTC()
	resp := &domain.TicketResponse{ID: ids.New(), TicketID: ticketID, Author: actorID, Message: message, Internal: internal, CreatedAt: now}
	if err := c.responses.Create(ctx, resp); err != nil {
		return nil, err
	}
	if isStaff && t.Status == domain.TicketOpen {
		t.Status = domain.TicketInProgress
		t.AssignedTo = actorID
		_ = c.tickets.Update(ctx, t)
	}
	return resp, nil
}

func (c *Comms) Responses(ctx context.Context, actorID, ticketID string) ([]domain.TicketResponse, error) {
	if _, _, err := c.Ticket(ctx, actorID, ticketID); err != nil {
		return nil, err
	}
	return c.responses.ByTicket(ctx, ticketID)
}

// TicketAction applies resolve/close (staff only).
func (c *Comms) TicketAction(ctx context.Context, actorID, ticketID, action string) (*domain.Ticket, error) {
	t, err := c.tickets.ByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if err := c.requireManagerOrStaff(ctx, actorID, t.BuildingID); err != nil {
		return nil, err
	}
	now := c.now().UTC()
	event := ""
	switch action {
	case "resolve":
		if t.Status == domain.TicketResolved || t.Status == domain.TicketClosed {
			return nil, domain.ErrInvalidState
		}
		t.Status = domain.TicketResolved
		t.ResolvedAt = &now
		event = "ticket.resolved"
	case "close":
		if t.Status == domain.TicketClosed {
			return nil, domain.ErrInvalidState
		}
		t.Status = domain.TicketClosed
		t.ClosedAt = &now
		event = "ticket.closed"
	default:
		return nil, domain.ErrInvalidState
	}
	if err := c.tickets.Update(ctx, t); err != nil {
		return nil, err
	}
	c.publish(ctx, event, t.ID, map[string]any{"building_id": t.BuildingID})
	return t, nil
}

// --- shared helpers ---

func (c *Comms) isStaff(ctx context.Context, userID, buildingID string) (bool, error) {
	return c.membership.AnyRole(ctx, userID, buildingID, "manager", "board_member", "staff")
}

func (c *Comms) requireManagerOrStaff(ctx context.Context, userID, buildingID string) error {
	ok, err := c.isStaff(ctx, userID, buildingID)
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (c *Comms) requireManager(ctx context.Context, userID, buildingID string) error {
	ok, err := c.membership.AnyRole(ctx, userID, buildingID, "manager", "board_member")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (c *Comms) requireMember(ctx context.Context, userID, buildingID string) error {
	ok, err := c.membership.HasRole(ctx, userID, buildingID, "*")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (c *Comms) publish(ctx context.Context, typ, subject string, data map[string]any) {
	_ = c.pub.Publish(ctx, events.New("notifications", typ, subject, data))
}
