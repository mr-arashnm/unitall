// Package httpapi exposes the operations REST surface.
package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/operations/internal/domain"
	"unital/backend/services/operations/internal/usecase"
)

type API struct {
	ops    *usecase.Ops
	signer *jwtx.Signer
	base   string
}

func New(o *usecase.Ops, signer *jwtx.Signer) *API {
	return &API{ops: o, signer: signer, base: "operations"}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /buildings/{buildingID}/teams", a.createTeam)
	mux.HandleFunc("GET /buildings/{buildingID}/teams", a.listTeams)
	mux.HandleFunc("GET /teams/{teamID}", a.getTeam)
	mux.HandleFunc("POST /teams/{teamID}/members", a.addMember)
	mux.HandleFunc("DELETE /teams/{teamID}/members/{userID}", a.removeMember)
	mux.HandleFunc("GET /teams/{teamID}/performance", a.teamPerformance)
	mux.HandleFunc("POST /teams/{teamID}/tasks", a.createTask)
	mux.HandleFunc("GET /buildings/{buildingID}/tasks", a.listTasks)
	mux.HandleFunc("GET /tasks/{taskID}", a.getTask)
	mux.HandleFunc("POST /tasks/{taskID}/assign", a.assignTask)
	mux.HandleFunc("POST /tasks/{taskID}/start", a.taskAction("start"))
	mux.HandleFunc("POST /tasks/{taskID}/complete", a.completeTask)
	mux.HandleFunc("POST /tasks/{taskID}/hold", a.taskAction("hold"))
	mux.HandleFunc("POST /tasks/{taskID}/cancel", a.taskAction("cancel"))
	mux.HandleFunc("POST /tasks/{taskID}/comments", a.addComment)
	mux.HandleFunc("GET /tasks/{taskID}/comments", a.listComments)
	mux.HandleFunc("POST /buildings/{buildingID}/service-requests", a.submitRequest)
	mux.HandleFunc("GET /buildings/{buildingID}/service-requests", a.listRequests)
	mux.HandleFunc("GET /service-requests/{requestID}", a.getRequest)
	mux.HandleFunc("POST /service-requests/{requestID}/assign", a.assignRequest)
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

func (a *API) unauthorized(w http.ResponseWriter, r *http.Request) {
	httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := httpx.NewProblem(a.base, "INTERNAL", "Internal server error", http.StatusInternalServerError)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		p = httpx.NewProblem(a.base, "NOT_FOUND", "Resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		p = httpx.NewProblem(a.base, "FORBIDDEN", "Not allowed for this building", http.StatusForbidden)
	case errors.Is(err, domain.ErrInvalidState):
		p = httpx.NewProblem(a.base, "INVALID_STATE", "Invalid state or input", http.StatusUnprocessableEntity)
	}
	httpx.WriteError(w, r, p)
}

// --- teams ---

func (a *API) createTeam(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var t domain.Team
	if err := httpx.Decode(w, r, &t); err != nil || t.Name == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "name", Message: "required"}))
		return
	}
	t.BuildingID = r.PathValue("buildingID")
	out, err := a.ops.CreateTeam(r.Context(), uid, &t)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listTeams(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	ts, err := a.ops.Teams(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if ts == nil {
		ts = []domain.Team{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": ts})
}

func (a *API) getTeam(w http.ResponseWriter, r *http.Request) {
	t, err := a.ops.Team(r.Context(), r.PathValue("teamID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (a *API) addMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.UserID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "user_id", Message: "required"}))
		return
	}
	t, err := a.ops.AddMember(r.Context(), uid, r.PathValue("teamID"), req.UserID, false)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (a *API) removeMember(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	t, err := a.ops.AddMember(r.Context(), uid, r.PathValue("teamID"), r.PathValue("userID"), true)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (a *API) teamPerformance(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	p, err := a.ops.TeamPerformance(r.Context(), uid, r.PathValue("teamID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// --- tasks ---

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		Title          string     `json:"title"`
		Description    string     `json:"description"`
		Priority       string     `json:"priority"`
		DueDate        *time.Time `json:"due_date"`
		EstimatedHours int        `json:"estimated_hours"`
		RelatedUnit    string     `json:"related_unit"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Title == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "title", Message: "required"}))
		return
	}
	t := &domain.Task{
		TeamID: r.PathValue("teamID"), Title: req.Title, Description: req.Description,
		Priority: req.Priority, EstimatedHours: req.EstimatedHours, RelatedUnit: req.RelatedUnit,
	}
	if req.DueDate != nil {
		t.DueDate = *req.DueDate
	}
	out, err := a.ops.CreateTask(r.Context(), uid, t)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	q := r.URL.Query()
	ts, err := a.ops.TasksFor(r.Context(), uid, r.PathValue("buildingID"), domain.TaskFilter{
		TeamID: q.Get("team_id"), Status: q.Get("status"),
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if ts == nil {
		ts = []domain.Task{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": ts})
}

func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	t, err := a.ops.Task(r.Context(), r.PathValue("taskID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (a *API) assignTask(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		StaffID string `json:"staff_id"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.StaffID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "staff_id", Message: "required"}))
		return
	}
	t, err := a.ops.TaskAction(r.Context(), uid, r.PathValue("taskID"), "assign", req.StaffID, 0)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (a *API) taskAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := a.userID(r)
		if !ok {
			a.unauthorized(w, r)
			return
		}
		t, err := a.ops.TaskAction(r.Context(), uid, r.PathValue("taskID"), action, "", 0)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, t)
	}
}

func (a *API) completeTask(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		ActualHours int `json:"actual_hours"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		req.ActualHours = 0 // empty body is fine
	}
	t, err := a.ops.TaskAction(r.Context(), uid, r.PathValue("taskID"), "complete", "", req.ActualHours)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

// --- comments ---

func (a *API) addComment(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		Comment string `json:"comment"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Comment == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "comment", Message: "required"}))
		return
	}
	c, err := a.ops.AddComment(r.Context(), r.PathValue("taskID"), uid, req.Comment)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, c)
}

func (a *API) listComments(w http.ResponseWriter, r *http.Request) {
	cs, err := a.ops.Comments(r.Context(), r.PathValue("taskID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if cs == nil {
		cs = []domain.Comment{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": cs})
}

// --- service requests ---

func (a *API) submitRequest(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		UnitID      string `json:"unit_id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Priority    string `json:"priority"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Title == "" || req.UnitID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "title", Message: "required"},
			httpx.Validation{Field: "unit_id", Message: "required"}))
		return
	}
	sr := &domain.ServiceRequest{
		BuildingID: r.PathValue("buildingID"), UnitID: req.UnitID, SubmittedBy: uid,
		Title: req.Title, Description: req.Description, Type: req.Type, Priority: req.Priority,
	}
	out, err := a.ops.SubmitRequest(r.Context(), sr)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listRequests(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	rs, err := a.ops.RequestsFor(r.Context(), uid, r.PathValue("buildingID"), domain.RequestFilter{
		Status: r.URL.Query().Get("status"),
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if rs == nil {
		rs = []domain.ServiceRequest{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": rs})
}

func (a *API) getRequest(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	sr, err := a.ops.Request(r.Context(), r.PathValue("requestID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	isStaff, err := a.ops.IsStaff(r.Context(), uid, sr.BuildingID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if !isStaff && sr.SubmittedBy != uid {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "FORBIDDEN", "Not allowed for this building", http.StatusForbidden))
		return
	}
	httpx.JSON(w, http.StatusOK, sr)
}

func (a *API) assignRequest(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		a.unauthorized(w, r)
		return
	}
	var req struct {
		TeamID string `json:"team_id"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.TeamID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "team_id", Message: "required"}))
		return
	}
	sr, err := a.ops.AssignToTeam(r.Context(), uid, r.PathValue("requestID"), req.TeamID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, sr)
}

// NewServer wires middleware (used by main and tests).
func NewServer(api *API) http.Handler {
	mux := http.NewServeMux()
	api.Routes(mux)
	return httpx.Chain(mux, httpx.RequestID, httpx.AccessLog, httpx.Recover)
}
