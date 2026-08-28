// Package httpapi exposes the notification service REST surface.
package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/notifications/internal/domain"
	"unital/backend/services/notifications/internal/usecase"
)

type API struct {
	notifier *usecase.Notifier
	comms    *usecase.Comms
	signer   *jwtx.Signer
	base     string
}

func New(n *usecase.Notifier, c *usecase.Comms, signer *jwtx.Signer) *API {
	return &API{notifier: n, comms: c, signer: signer, base: "notifications"}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /templates", a.upsertTemplate)
	mux.HandleFunc("GET /templates", a.listTemplates)
	mux.HandleFunc("POST /notifications:send", a.send)
	mux.HandleFunc("GET /notifications/{id}", a.getCampaign)
	mux.HandleFunc("GET /me/notifications", a.myInbox)
	mux.HandleFunc("POST /me/notifications/{id}/read", a.markRead)
	// communications: announcements
	mux.HandleFunc("POST /buildings/{buildingID}/announcements", a.createAnnouncement)
	mux.HandleFunc("GET /buildings/{buildingID}/announcements", a.listAnnouncements)
	mux.HandleFunc("GET /announcements/{announcementID}", a.getAnnouncement)
	mux.HandleFunc("PATCH /announcements/{announcementID}", a.patchAnnouncement)
	mux.HandleFunc("DELETE /announcements/{announcementID}", a.deleteAnnouncement)
	mux.HandleFunc("POST /announcements/{announcementID}/publish", a.publishAnnouncement)
	// communications: meetings
	mux.HandleFunc("POST /buildings/{buildingID}/meetings", a.createMeeting)
	mux.HandleFunc("GET /buildings/{buildingID}/meetings", a.listMeetings)
	mux.HandleFunc("GET /meetings/{meetingID}", a.getMeeting)
	mux.HandleFunc("PATCH /meetings/{meetingID}", a.patchMeeting)
	mux.HandleFunc("POST /meetings/{meetingID}/rsvp", a.rsvp)
	mux.HandleFunc("GET /meetings/{meetingID}/attendance", a.listAttendance)
	mux.HandleFunc("PUT /meetings/{meetingID}/minutes", a.upsertMinutes)
	mux.HandleFunc("GET /meetings/{meetingID}/minutes", a.getMinutes)
	mux.HandleFunc("POST /meetings/{meetingID}/minutes/sign", a.signMinutes)
	// communications: support tickets
	mux.HandleFunc("POST /buildings/{buildingID}/tickets", a.submitTicket)
	mux.HandleFunc("GET /buildings/{buildingID}/tickets", a.listTickets)
	mux.HandleFunc("GET /tickets/{ticketID}", a.getTicket)
	mux.HandleFunc("POST /tickets/{ticketID}/responses", a.respondTicket)
	mux.HandleFunc("GET /tickets/{ticketID}/responses", a.listTicketResponses)
	mux.HandleFunc("POST /tickets/{ticketID}/resolve", a.ticketAction("resolve"))
	mux.HandleFunc("POST /tickets/{ticketID}/close", a.ticketAction("close"))
	mux.HandleFunc("POST /internal/trigger", a.trigger)
	mux.HandleFunc("POST /internal/dispatch", a.dispatch)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, 200, map[string]string{"status": "ok"}) })
}

func (a *API) userID(r *http.Request) (string, bool) {
	if id := r.Header.Get("X-User-Id"); id != "" {
		return id, true
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	claims, err := a.signer.Parse(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		return "", false
	}
	return claims.Sub, true
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := httpx.NewProblem(a.base, "INTERNAL", "Internal server error", http.StatusInternalServerError)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		p = httpx.NewProblem(a.base, "NOT_FOUND", "Resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		p = httpx.NewProblem(a.base, "FORBIDDEN", "Not allowed for this building", http.StatusForbidden)
	case errors.Is(err, domain.ErrTemplateMissing):
		p = httpx.NewProblem(a.base, "TEMPLATE_MISSING", "Template not found", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrBadChannel):
		p = httpx.NewProblem(a.base, "BAD_CHANNEL", err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrBadTarget):
		p = httpx.NewProblem(a.base, "BAD_TARGET", "Invalid target", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrInvalidState):
		p = httpx.NewProblem(a.base, "INVALID_STATE", "Invalid state or input", http.StatusUnprocessableEntity)
	}
	httpx.WriteError(w, r, p)
}

func (a *API) upsertTemplate(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var t domain.Template
	if err := httpx.Decode(w, r, &t); err != nil || t.Name == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "name", Message: "required"}))
		return
	}
	out, err := a.notifier.UpsertTemplate(r.Context(), uid, &t)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) listTemplates(w http.ResponseWriter, r *http.Request) {
	ts, err := a.notifier.Templates(r.Context())
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if ts == nil {
		ts = []domain.Template{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": ts})
}

func (a *API) send(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		BuildingID string            `json:"building_id"`
		Template   string            `json:"template"`
		Target     domain.Target     `json:"target"`
		Vars       map[string]string `json:"vars"`
		Channels   []string          `json:"channels"`
		ScheduleAt *string           `json:"schedule_at"` // RFC3339
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.BuildingID == "" || req.Template == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "building_id", Message: "required"},
			httpx.Validation{Field: "template", Message: "required"}))
		return
	}
	in := usecase.SendInput{
		ActorID: uid, BuildingID: req.BuildingID, Template: req.Template,
		Target: req.Target, Vars: req.Vars, Channels: req.Channels,
	}
	if req.ScheduleAt != nil {
		if t, err := time.Parse(time.RFC3339, *req.ScheduleAt); err == nil {
			in.ScheduleAt = &t
		}
	}
	camp, err := a.notifier.Send(r.Context(), in)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, camp)
}

func (a *API) getCampaign(w http.ResponseWriter, r *http.Request) {
	camp, dels, err := a.notifier.Campaign(r.Context(), r.PathValue("id"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	counters := map[string]int{"pending": 0, "sent": 0, "failed": 0}
	for _, d := range dels {
		counters[d.Status]++
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"campaign": camp, "counters": counters, "deliveries": dels})
}

func (a *API) myInbox(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	unread := r.URL.Query().Get("unread") == "true"
	msgs, err := a.notifier.Inbox(r.Context(), uid, unread)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if msgs == nil {
		msgs = []domain.InboxMessage{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": msgs})
}

func (a *API) markRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if err := a.notifier.MarkRead(r.Context(), r.PathValue("id"), uid); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "read"})
}

// trigger feeds a domain event through the configured mapping — the
// entrypoint the NATS subscriber will call in production.
func (a *API) trigger(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type       string            `json:"type"`
		BuildingID string            `json:"building_id"`
		Data       map[string]string `json:"data"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Type == "" || req.BuildingID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "type", Message: "required"},
			httpx.Validation{Field: "building_id", Message: "required"}))
		return
	}
	binding, ok := eventBindings[req.Type]
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "EVENT_UNMAPPED", "No template bound for this event", http.StatusUnprocessableEntity))
		return
	}
	camp, err := a.notifier.HandleEvent(r.Context(), binding, req.BuildingID, req.Data)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, camp)
}

func (a *API) dispatch(w http.ResponseWriter, r *http.Request) {
	n, err := a.notifier.Dispatch(r.Context(), 100)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"dispatched": n})
}

// eventBindings is the event→template mapping (NOTIFICATION_DESIGN §5);
// moves to config when the NATS subscriber lands.
var eventBindings = map[string]usecase.EventBinding{
	"charge.overdue":    {Template: "charge.overdue.reminder", TargetKind: domain.TargetUsers, RecipientKey: "user_id"},
	"payment.completed": {Template: "payment.receipt", TargetKind: domain.TargetUsers, RecipientKey: "user_id"},
	"booking.confirmed": {Template: "booking.confirmed.reminder", TargetKind: domain.TargetUsers, RecipientKey: "user_id"},
	"charges.generated": {Template: "charges.generated.summary", TargetKind: domain.TargetRole, RecipientKey: ""},
}

// --- communications: announcements ---

func (a *API) createAnnouncement(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var ann domain.Announcement
	if err := httpx.Decode(w, r, &ann); err != nil || ann.Title == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "title", Message: "required"}))
		return
	}
	ann.BuildingID = r.PathValue("buildingID")
	out, err := a.comms.CreateAnnouncement(r.Context(), uid, &ann)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	anns, err := a.comms.Announcements(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": anns})
}

func (a *API) getAnnouncement(w http.ResponseWriter, r *http.Request) {
	ann, err := a.comms.Announcement(r.Context(), r.PathValue("announcementID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ann)
}

func (a *API) patchAnnouncement(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var patch struct {
		Title     string         `json:"title"`
		Content   string         `json:"content"`
		Priority  string         `json:"priority"`
		Target    *domain.Target `json:"target"`
		ExpiresAt *string        `json:"expires_at"` // RFC3339
	}
	if err := httpx.Decode(w, r, &patch); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	out, err := a.comms.PatchAnnouncement(r.Context(), uid, r.PathValue("announcementID"), func(ann *domain.Announcement) {
		if patch.Title != "" {
			ann.Title = patch.Title
		}
		if patch.Content != "" {
			ann.Content = patch.Content
		}
		if patch.Priority != "" {
			ann.Priority = patch.Priority
		}
		if patch.Target != nil {
			ann.Target = *patch.Target
		}
		if patch.ExpiresAt != nil {
			if t, err := time.Parse(time.RFC3339, *patch.ExpiresAt); err == nil {
				ann.ExpiresAt = &t
			}
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	if err := a.comms.DeleteAnnouncement(r.Context(), uid, r.PathValue("announcementID")); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) publishAnnouncement(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	ann, err := a.comms.Publish(r.Context(), uid, r.PathValue("announcementID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, ann)
}

// --- communications: meetings ---

func (a *API) createMeeting(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		Title       string         `json:"title"`
		Type        string         `json:"type"`
		Description string         `json:"description"`
		Agenda      string         `json:"agenda"`
		ScheduledAt string         `json:"scheduled_at"`
		Location    string         `json:"location"`
		DurationMin int            `json:"duration_min"`
		Target      *domain.Target `json:"target"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Title == "" || req.ScheduledAt == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "title", Message: "required"},
			httpx.Validation{Field: "scheduled_at", Message: "required (RFC3339)"}))
		return
	}
	when, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "scheduled_at", Message: "must be RFC3339"}))
		return
	}
	m := &domain.Meeting{
		Title: req.Title, Type: req.Type, Description: req.Description,
		Agenda: req.Agenda, ScheduledAt: when, Location: req.Location,
		DurationMin: req.DurationMin,
	}
	if req.Target != nil {
		m.Target = *req.Target
	}
	m.BuildingID = r.PathValue("buildingID")
	out, err := a.comms.CreateMeeting(r.Context(), uid, m)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listMeetings(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	ms, err := a.comms.Meetings(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": ms})
}

func (a *API) getMeeting(w http.ResponseWriter, r *http.Request) {
	m, err := a.comms.Meeting(r.Context(), r.PathValue("meetingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

func (a *API) patchMeeting(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var patch struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Agenda      string `json:"agenda"`
		Location    string `json:"location"`
		Status      string `json:"status"`
	}
	if err := httpx.Decode(w, r, &patch); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	out, err := a.comms.PatchMeeting(r.Context(), uid, r.PathValue("meetingID"), func(m *domain.Meeting) {
		if patch.Title != "" {
			m.Title = patch.Title
		}
		if patch.Description != "" {
			m.Description = patch.Description
		}
		if patch.Agenda != "" {
			m.Agenda = patch.Agenda
		}
		if patch.Location != "" {
			m.Location = patch.Location
		}
		if patch.Status != "" {
			m.Status = patch.Status
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) rsvp(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Status == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "status", Message: "required: confirmed|declined|attended|absent"}))
		return
	}
	att, err := a.comms.RSVP(r.Context(), uid, r.PathValue("meetingID"), req.Status)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, att)
}

func (a *API) listAttendance(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	atts, err := a.comms.Attendance(r.Context(), uid, r.PathValue("meetingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if atts == nil {
		atts = []domain.Attendance{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": atts})
}

func (a *API) upsertMinutes(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var m domain.Minute
	if err := httpx.Decode(w, r, &m); err != nil || m.Content == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "content", Message: "required"}))
		return
	}
	m.MeetingID = r.PathValue("meetingID")
	out, err := a.comms.UpsertMinutes(r.Context(), uid, &m)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) getMinutes(w http.ResponseWriter, r *http.Request) {
	m, err := a.comms.Minutes(r.Context(), r.PathValue("meetingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

func (a *API) signMinutes(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	m, err := a.comms.SignMinutes(r.Context(), uid, r.PathValue("meetingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

// --- communications: support tickets ---

func (a *API) submitTicket(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var t domain.Ticket
	if err := httpx.Decode(w, r, &t); err != nil || t.Title == "" || t.UnitID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "title", Message: "required"},
			httpx.Validation{Field: "unit_id", Message: "required"}))
		return
	}
	t.BuildingID = r.PathValue("buildingID")
	out, err := a.comms.SubmitTicket(r.Context(), uid, &t)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listTickets(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	ts, err := a.comms.Tickets(r.Context(), uid, r.PathValue("buildingID"), domain.TicketFilter{
		Status: r.URL.Query().Get("status"),
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if ts == nil {
		ts = []domain.Ticket{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": ts})
}

func (a *API) getTicket(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	t, rs, err := a.comms.Ticket(r.Context(), uid, r.PathValue("ticketID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if rs == nil {
		rs = []domain.TicketResponse{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"ticket": t, "responses": rs})
}

func (a *API) respondTicket(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		Message  string `json:"message"`
		Internal bool   `json:"internal"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Message == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "message", Message: "required"}))
		return
	}
	resp, err := a.comms.Respond(r.Context(), uid, r.PathValue("ticketID"), req.Message, req.Internal)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

func (a *API) listTicketResponses(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	rs, err := a.comms.Responses(r.Context(), uid, r.PathValue("ticketID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if rs == nil {
		rs = []domain.TicketResponse{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": rs})
}

func (a *API) ticketAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := a.userID(r)
		if !ok {
			a.unauthorized(w, r)
			return
		}
		t, err := a.comms.TicketAction(r.Context(), uid, r.PathValue("ticketID"), action)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, t)
	}
}

func (a *API) unauthorized(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
}

// NewServer wires middleware around the mux (used by main and tests).
func NewServer(api *API) http.Handler {
	mux := http.NewServeMux()
	api.Routes(mux)
	return httpx.Chain(mux, httpx.RequestID, httpx.AccessLog, httpx.Recover)
}
