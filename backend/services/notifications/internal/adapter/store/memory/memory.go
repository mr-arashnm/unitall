// Package memory provides in-memory notification stores and a seeded
// recipient resolver + membership table (dev/tests; production uses
// identity/property internal APIs).
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"unital/backend/pkg/ids"
	"unital/backend/services/notifications/internal/domain"
)

type core struct {
	mu         sync.Mutex
	templates  map[string]*domain.Template
	campaigns  map[string]*domain.Campaign
	deliveries []*domain.Delivery
	inbox      []*domain.InboxMessage
	anns       map[string]*domain.Announcement
	meets      map[string]*domain.Meeting
	atts       map[string]*domain.Attendance // key: meetingID|userID
	mins       map[string]*domain.Minute     // key: meetingID
	ticks      map[string]*domain.Ticket
	tresps     []*domain.TicketResponse
}

// Bundle groups all adapters over one core.
type Bundle struct {
	Templates     *TemplateStore
	Campaigns     *CampaignStore
	Deliveries    *DeliveryStore
	Inbox         *InboxStore
	Directory     *Directory
	Announcements *AnnouncementStore
	Meetings      *MeetingStore
	Attendance    *AttendanceStore
	Minutes       *MinuteStore
	Tickets       *TicketStore
	TicketResp    *TicketResponseStore
}

func New() Bundle {
	c := &core{
		templates: map[string]*domain.Template{},
		campaigns: map[string]*domain.Campaign{},
		anns:      map[string]*domain.Announcement{},
		meets:     map[string]*domain.Meeting{},
		atts:      map[string]*domain.Attendance{},
		mins:      map[string]*domain.Minute{},
		ticks:     map[string]*domain.Ticket{},
	}
	inbox := &InboxStore{core: c}
	return Bundle{
		Templates:     &TemplateStore{core: c},
		Campaigns:     &CampaignStore{core: c},
		Deliveries:    &DeliveryStore{core: c},
		Inbox:         inbox,
		Directory:     &Directory{inbox: inbox, roles: map[string]string{}, users: map[string][]domain.Recipient{}},
		Announcements: &AnnouncementStore{c},
		Meetings:      &MeetingStore{c},
		Attendance:    &AttendanceStore{c},
		Minutes:       &MinuteStore{c},
		Tickets:       &TicketStore{c},
		TicketResp:    &TicketResponseStore{c},
	}
}

// --- TemplateStore ---

type TemplateStore struct{ *core }

func (s *TemplateStore) Upsert(_ context.Context, t *domain.Template) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templates[t.Name] = t
	return nil
}

func (s *TemplateStore) ByName(_ context.Context, name string) (*domain.Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.templates[name]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *TemplateStore) List(_ context.Context) ([]domain.Template, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Template, 0, len(s.templates))
	for _, t := range s.templates {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// --- CampaignStore ---

type CampaignStore struct{ *core }

func (s *CampaignStore) Create(_ context.Context, c *domain.Campaign) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.campaigns[c.ID] = c
	return nil
}

func (s *CampaignStore) ByID(_ context.Context, id string) (*domain.Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.campaigns[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (s *CampaignStore) Due(_ context.Context, now time.Time) ([]domain.Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Campaign
	for _, c := range s.campaigns {
		if c.Status == domain.CampScheduled && c.ScheduleAt != nil && !c.ScheduleAt.After(now) {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (s *CampaignStore) MarkSending(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.campaigns[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.Status = domain.CampSending
	return nil
}

// --- DeliveryStore ---

type DeliveryStore struct{ *core }

func (s *DeliveryStore) Create(_ context.Context, d *domain.Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, d)
	return nil
}

// ClaimDue returns pending deliveries whose retry time has arrived
// (the postgres adapter uses FOR UPDATE SKIP LOCKED).
func (s *DeliveryStore) ClaimDue(_ context.Context, now time.Time, limit int) ([]domain.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.Delivery, 0, limit)
	for _, d := range s.deliveries {
		if len(out) >= limit {
			break
		}
		if d.Status == domain.DelivPending && !d.NextRetryAt.After(now) {
			out = append(out, *d)
		}
	}
	return out, nil
}

func (s *DeliveryStore) Update(_ context.Context, d *domain.Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.deliveries {
		if e.ID == d.ID {
			s.deliveries[i] = d
			return nil
		}
	}
	return domain.ErrNotFound
}

func (s *DeliveryStore) ByCampaign(_ context.Context, campaignID string) ([]domain.Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Delivery
	for _, d := range s.deliveries {
		if d.CampaignID == campaignID {
			out = append(out, *d)
		}
	}
	return out, nil
}

// --- InboxStore ---

type InboxStore struct{ *core }

func (s *InboxStore) Push(_ context.Context, m *domain.InboxMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = ids.New()
	}
	s.inbox = append(s.inbox, m)
	return nil
}

func (s *InboxStore) ByUser(_ context.Context, userID string, unreadOnly bool) ([]domain.InboxMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.InboxMessage
	for i := len(s.inbox) - 1; i >= 0; i-- { // newest first
		m := s.inbox[i]
		if m.UserID == userID && (!unreadOnly || !m.Read) {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (s *InboxStore) MarkRead(_ context.Context, id, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.inbox {
		if m.ID == id && m.UserID == userID {
			m.Read = true
			return nil
		}
	}
	return domain.ErrNotFound
}

// --- Directory: RecipientResolver + MembershipChecker ---

type Directory struct {
	inbox *InboxStore
	mu    sync.RWMutex
	roles map[string]string             // "user|building|role" set marker → simpler: "user|building" -> role
	users map[string][]domain.Recipient // buildingID -> recipients
}

func (d *Directory) Seed(buildingID string, rs ...domain.Recipient) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.users[buildingID] = append(d.users[buildingID], rs...)
}

func (d *Directory) SeedMembership(userID, buildingID, role string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.roles[userAndBuilding(userID, buildingID)] = role
}

func userAndBuilding(u, b string) string { return u + "|" + b }

func (d *Directory) HasRole(_ context.Context, userID, buildingID, role string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	got, ok := d.roles[userAndBuilding(userID, buildingID)]
	if !ok {
		return false, nil
	}
	return role == "*" || got == role, nil
}

func (d *Directory) AnyRole(_ context.Context, userID, buildingID string, roles ...string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	got, ok := d.roles[userAndBuilding(userID, buildingID)]
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

// Resolve expands targets over the seeded directory.
func (d *Directory) Resolve(_ context.Context, buildingID string, t domain.Target) ([]domain.Recipient, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	all := d.users[buildingID]
	switch t.Kind {
	case domain.TargetAll:
		return append([]domain.Recipient(nil), all...), nil
	case domain.TargetRole:
		var out []domain.Recipient
		for _, r := range all {
			role := d.roles[userAndBuilding(r.ID, buildingID)]
			for _, want := range t.Values {
				if role == want {
					out = append(out, r)
					break
				}
			}
		}
		return out, nil
	case domain.TargetUsers:
		want := map[string]bool{}
		for _, v := range t.Values {
			want[v] = true
		}
		var out []domain.Recipient
		for _, r := range all {
			if want[r.ID] {
				out = append(out, r)
			}
		}
		return out, nil
	case domain.TargetUnits:
		return nil, domain.ErrBadTarget // unit-party resolution arrives with property internal API
	default:
		return nil, domain.ErrBadTarget
	}
}

// --- communications stores ---

type AnnouncementStore struct{ *core }

func (s *AnnouncementStore) Create(_ context.Context, a *domain.Announcement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.anns[a.ID] = a
	return nil
}

func (s *AnnouncementStore) ByID(_ context.Context, id string) (*domain.Announcement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.anns[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (s *AnnouncementStore) Update(_ context.Context, a *domain.Announcement) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.anns[a.ID]; !ok {
		return domain.ErrNotFound
	}
	s.anns[a.ID] = a
	return nil
}

func (s *AnnouncementStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.anns, id)
	return nil
}

func (s *AnnouncementStore) ListByBuilding(_ context.Context, buildingID string) ([]domain.Announcement, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Announcement
	for _, a := range s.anns {
		if a.BuildingID == buildingID {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

type MeetingStore struct{ *core }

func (s *MeetingStore) Create(_ context.Context, m *domain.Meeting) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meets[m.ID] = m
	return nil
}

func (s *MeetingStore) ByID(_ context.Context, id string) (*domain.Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.meets[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m, nil
}

func (s *MeetingStore) Update(_ context.Context, m *domain.Meeting) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.meets[m.ID]; !ok {
		return domain.ErrNotFound
	}
	s.meets[m.ID] = m
	return nil
}

func (s *MeetingStore) ListByBuilding(_ context.Context, buildingID string) ([]domain.Meeting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Meeting
	for _, m := range s.meets {
		if m.BuildingID == buildingID {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ScheduledAt.Before(out[j].ScheduledAt) })
	return out, nil
}

type AttendanceStore struct{ *core }

func attKey(meetingID, userID string) string { return meetingID + "|" + userID }

func (s *AttendanceStore) Upsert(_ context.Context, a *domain.Attendance) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if prev, ok := s.atts[attKey(a.MeetingID, a.UserID)]; ok {
		a.ID = prev.ID // keep the original row identity
		a.CreatedAt = prev.CreatedAt
	}
	s.atts[attKey(a.MeetingID, a.UserID)] = a
	return nil
}

func (s *AttendanceStore) ByMeeting(_ context.Context, meetingID string) ([]domain.Attendance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Attendance
	for k, a := range s.atts {
		if strings.HasPrefix(k, meetingID+"|") {
			out = append(out, *a)
		}
	}
	return out, nil
}

type MinuteStore struct{ *core }

func (s *MinuteStore) Upsert(_ context.Context, m *domain.Minute) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mins[m.MeetingID] = m
	return nil
}

func (s *MinuteStore) ByMeeting(_ context.Context, meetingID string) (*domain.Minute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.mins[meetingID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return m, nil
}

type TicketStore struct{ *core }

func (s *TicketStore) Create(_ context.Context, t *domain.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ticks[t.ID] = t
	return nil
}

func (s *TicketStore) ByID(_ context.Context, id string) (*domain.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.ticks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *TicketStore) Update(_ context.Context, t *domain.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ticks[t.ID]; !ok {
		return domain.ErrNotFound
	}
	s.ticks[t.ID] = t
	return nil
}

func (s *TicketStore) List(_ context.Context, f domain.TicketFilter) ([]domain.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Ticket
	for _, t := range s.ticks {
		if f.BuildingID != "" && t.BuildingID != f.BuildingID {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.SubmittedBy != "" && t.SubmittedBy != f.SubmittedBy {
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.Before(out[j].SubmittedAt) })
	return out, nil
}

type TicketResponseStore struct{ *core }

func (s *TicketResponseStore) Create(_ context.Context, r *domain.TicketResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tresps = append(s.tresps, r)
	return nil
}

func (s *TicketResponseStore) ByTicket(_ context.Context, ticketID string) ([]domain.TicketResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.TicketResponse
	for _, r := range s.tresps {
		if r.TicketID == ticketID {
			out = append(out, *r)
		}
	}
	return out, nil
}
