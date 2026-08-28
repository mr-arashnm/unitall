// Package usecase implements operations workflows: teams, tasks,
// service requests (auto-task creation), and team performance.
package usecase

import (
	"context"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/ids"
	"unital/backend/services/operations/internal/domain"
)

type Ops struct {
	teams    domain.TeamStore
	tasks    domain.TaskStore
	comments domain.CommentStore
	reqs     domain.RequestStore
	member   domain.MembershipChecker
	pub      events.Publisher
	now      func() time.Time
}

func New(t domain.TeamStore, tk domain.TaskStore, c domain.CommentStore,
	r domain.RequestStore, m domain.MembershipChecker, pub events.Publisher) *Ops {
	return &Ops{teams: t, tasks: tk, comments: c, reqs: r, member: m, pub: pub, now: time.Now}
}

// --- teams ---

func (o *Ops) CreateTeam(ctx context.Context, actorID string, t *domain.Team) (*domain.Team, error) {
	if err := o.requireManager(ctx, actorID, t.BuildingID); err != nil {
		return nil, err
	}
	if t.Name == "" {
		return nil, domain.ErrInvalidState
	}
	t.ID = ids.New()
	t.IsActive = true
	t.CreatedAt = o.now().UTC()
	if err := o.teams.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (o *Ops) Team(ctx context.Context, id string) (*domain.Team, error) {
	return o.teams.ByID(ctx, id)
}

func (o *Ops) Teams(ctx context.Context, actorID, buildingID string) ([]domain.Team, error) {
	ok, err := o.member.HasRole(ctx, actorID, buildingID, "*")
	if err != nil || !ok {
		return nil, domain.ErrForbidden
	}
	return o.teams.ListByBuilding(ctx, buildingID)
}

// AddMember adds (remove=true removes) a staff member to a team.
func (o *Ops) AddMember(ctx context.Context, actorID, teamID, userID string, remove bool) (*domain.Team, error) {
	t, err := o.teams.ByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if err := o.requireManager(ctx, actorID, t.BuildingID); err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, m := range t.Members {
		set[m] = true
	}
	if remove {
		delete(set, userID)
	} else {
		set[userID] = true
	}
	t.Members = t.Members[:0]
	for m := range set {
		t.Members = append(t.Members, m)
	}
	if err := o.teams.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// --- tasks ---

func (o *Ops) CreateTask(ctx context.Context, actorID string, t *domain.Task) (*domain.Task, error) {
	team, err := o.teams.ByID(ctx, t.TeamID)
	if err != nil {
		return nil, err
	}
	if err := o.requireManager(ctx, actorID, team.BuildingID); err != nil {
		return nil, err
	}
	if t.Title == "" {
		return nil, domain.ErrInvalidState
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	t.ID = ids.New()
	t.BuildingID = team.BuildingID
	t.Status = domain.TaskPending
	t.CreatedBy = actorID
	t.CreatedAt = o.now().UTC()
	if err := o.tasks.Create(ctx, t); err != nil {
		return nil, err
	}
	o.publish(ctx, "task.created", t.ID, map[string]any{"team_id": t.TeamID})
	return t, nil
}

func (o *Ops) Task(ctx context.Context, id string) (*domain.Task, error) {
	return o.tasks.ByID(ctx, id)
}

func (o *Ops) Tasks(ctx context.Context, f domain.TaskFilter) ([]domain.Task, error) {
	return o.tasks.List(ctx, f)
}

// TasksFor returns the tasks visible to one actor in a building:
// managers/board see everything, staff see tasks of teams they belong
// to (or supervise) plus tasks assigned to them, others see nothing.
func (o *Ops) TasksFor(ctx context.Context, actorID, buildingID string, f domain.TaskFilter) ([]domain.Task, error) {
	f.BuildingID = buildingID
	isMgr, err := o.member.AnyRole(ctx, actorID, buildingID, "manager", "board_member")
	if err != nil {
		return nil, err
	}
	if isMgr {
		return o.tasks.List(ctx, f)
	}
	teams, err := o.teams.ListByBuilding(ctx, buildingID)
	if err != nil {
		return nil, err
	}
	visibleTeams := map[string]bool{}
	for _, t := range teams {
		if t.Supervisor == actorID {
			visibleTeams[t.ID] = true
			continue
		}
		for _, m := range t.Members {
			if m == actorID {
				visibleTeams[t.ID] = true
				break
			}
		}
	}
	all, err := o.tasks.List(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Task, 0, len(all))
	for _, t := range all {
		if visibleTeams[t.TeamID] || t.AssignedTo == actorID {
			out = append(out, t)
		}
	}
	return out, nil
}

// TaskAction applies assign/start/complete/hold/cancel with transitions.
// actualHours is recorded when completing (0 = unset).
func (o *Ops) TaskAction(ctx context.Context, actorID, taskID, action, assignee string, actualHours int) (*domain.Task, error) {
	t, err := o.tasks.ByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	// managers/board may do anything; assigned staff may start/complete their own
	isMgr, _ := o.member.AnyRole(ctx, actorID, t.BuildingID, "manager", "board_member")
	now := o.now().UTC()
	switch action {
	case "assign":
		if !isMgr {
			return nil, domain.ErrForbidden
		}
		if t.Status != domain.TaskPending && t.Status != domain.TaskAssigned {
			return nil, domain.ErrInvalidState
		}
		t.AssignedTo = assignee
		t.Status = domain.TaskAssigned
		t.AssignedAt = &now
	case "start":
		if !isMgr && t.AssignedTo != actorID {
			return nil, domain.ErrForbidden
		}
		if t.Status != domain.TaskAssigned && t.Status != domain.TaskPending {
			return nil, domain.ErrInvalidState
		}
		t.Status = domain.TaskInProgress
		t.StartedAt = &now
	case "complete":
		if !isMgr && t.AssignedTo != actorID {
			return nil, domain.ErrForbidden
		}
		if t.Status != domain.TaskInProgress {
			return nil, domain.ErrInvalidState
		}
		t.Status = domain.TaskCompleted
		t.CompletedAt = &now
		if actualHours > 0 {
			t.ActualHours = actualHours
		}
		o.publish(ctx, "task.completed", t.ID, map[string]any{"team_id": t.TeamID})
	case "hold":
		if !isMgr {
			return nil, domain.ErrForbidden
		}
		if t.Status != domain.TaskInProgress && t.Status != domain.TaskAssigned {
			return nil, domain.ErrInvalidState
		}
		t.Status = domain.TaskOnHold
	case "cancel":
		if !isMgr {
			return nil, domain.ErrForbidden
		}
		if t.Status == domain.TaskCompleted {
			return nil, domain.ErrInvalidState
		}
		t.Status = domain.TaskCancelled
	default:
		return nil, domain.ErrInvalidState
	}
	if err := o.tasks.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// --- comments ---

func (o *Ops) AddComment(ctx context.Context, taskID, author, text string) (*domain.Comment, error) {
	if _, err := o.tasks.ByID(ctx, taskID); err != nil {
		return nil, err
	}
	if text == "" {
		return nil, domain.ErrInvalidState
	}
	c := &domain.Comment{ID: ids.New(), TaskID: taskID, Author: author, Comment: text, CreatedAt: o.now().UTC()}
	if err := o.comments.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (o *Ops) Comments(ctx context.Context, taskID string) ([]domain.Comment, error) {
	return o.comments.ByTask(ctx, taskID)
}

// --- service requests ---

// SubmitRequest is the resident intake (any building member).
func (o *Ops) SubmitRequest(ctx context.Context, r *domain.ServiceRequest) (*domain.ServiceRequest, error) {
	ok, err := o.member.HasRole(ctx, r.SubmittedBy, r.BuildingID, "*")
	if err != nil || !ok {
		return nil, domain.ErrForbidden
	}
	if r.Title == "" || r.UnitID == "" {
		return nil, domain.ErrInvalidState
	}
	if r.Type == "" {
		r.Type = "maintenance"
	}
	if r.Priority == "" {
		r.Priority = "medium"
	}
	r.ID = ids.New()
	r.Status = domain.ReqSubmitted
	r.SubmittedAt = o.now().UTC()
	if err := o.reqs.Create(ctx, r); err != nil {
		return nil, err
	}
	o.publish(ctx, "servicerequest.submitted", r.ID, map[string]any{"building_id": r.BuildingID})
	return r, nil
}

func (o *Ops) Requests(ctx context.Context, f domain.RequestFilter) ([]domain.ServiceRequest, error) {
	return o.reqs.List(ctx, f)
}

// RequestsFor scopes request visibility: managers/board/staff see the
// building's requests, everyone else only their own submissions.
func (o *Ops) RequestsFor(ctx context.Context, actorID, buildingID string, f domain.RequestFilter) ([]domain.ServiceRequest, error) {
	f.BuildingID = buildingID
	isStaff, err := o.member.AnyRole(ctx, actorID, buildingID, "manager", "board_member", "staff")
	if err != nil {
		return nil, err
	}
	if !isStaff {
		f.SubmittedBy = actorID
	}
	return o.reqs.List(ctx, f)
}

// IsStaff reports whether the actor may manage building requests
// (manager, board member, or staff).
func (o *Ops) IsStaff(ctx context.Context, userID, buildingID string) (bool, error) {
	return o.member.AnyRole(ctx, userID, buildingID, "manager", "board_member", "staff")
}

func (o *Ops) Request(ctx context.Context, id string) (*domain.ServiceRequest, error) {
	return o.reqs.ByID(ctx, id)
}

// AssignToTeam triages a request: assigns the team, creates the linked
// task, and updates both statuses.
func (o *Ops) AssignToTeam(ctx context.Context, actorID, requestID, teamID string) (*domain.ServiceRequest, error) {
	req, err := o.reqs.ByID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if err := o.requireManager(ctx, actorID, req.BuildingID); err != nil {
		return nil, err
	}
	team, err := o.teams.ByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team.BuildingID != req.BuildingID {
		return nil, domain.ErrInvalidState
	}
	now := o.now().UTC()
	task := &domain.Task{
		ID: ids.New(), TeamID: teamID, BuildingID: req.BuildingID,
		Title: req.Title, Description: req.Description,
		Priority: req.Priority, Status: domain.TaskPending,
		DueDate: now.AddDate(0, 0, 3), RelatedUnit: req.UnitID,
		CreatedBy: actorID, CreatedAt: now,
	}
	if err := o.tasks.Create(ctx, task); err != nil {
		return nil, err
	}
	req.AssignedTeam = teamID
	req.RelatedTask = task.ID
	req.Status = domain.ReqAssigned
	if err := o.reqs.Update(ctx, req); err != nil {
		return nil, err
	}
	o.publish(ctx, "task.created", task.ID, map[string]any{"team_id": teamID, "from_request": req.ID})
	return req, nil
}

// --- performance ---

type Performance struct {
	TeamID         string  `json:"team_id"`
	Total          int     `json:"total_tasks"`
	Completed      int     `json:"completed_tasks"`
	Overdue        int     `json:"overdue_tasks"`
	CompletionRate float64 `json:"completion_rate"`
}

// TeamPerformance computes KPIs from live task data (the Django version
// stored monthly snapshots; this computes on read — snapshots arrive with
// the postgres adapter).
func (o *Ops) TeamPerformance(ctx context.Context, actorID, teamID string) (*Performance, error) {
	team, err := o.teams.ByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if err := o.requireManager(ctx, actorID, team.BuildingID); err != nil {
		return nil, err
	}
	tasks, err := o.tasks.List(ctx, domain.TaskFilter{TeamID: teamID})
	if err != nil {
		return nil, err
	}
	p := &Performance{TeamID: teamID}
	now := o.now()
	for _, t := range tasks {
		p.Total++
		if t.Status == domain.TaskCompleted {
			p.Completed++
		}
		if t.Status != domain.TaskCompleted && t.Status != domain.TaskCancelled && t.DueDate.Before(now) {
			p.Overdue++
		}
	}
	if p.Total > 0 {
		p.CompletionRate = float64(p.Completed) / float64(p.Total)
	}
	return p, nil
}

func (o *Ops) requireManager(ctx context.Context, userID, buildingID string) error {
	ok, err := o.member.AnyRole(ctx, userID, buildingID, "manager", "board_member")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (o *Ops) publish(ctx context.Context, typ, subject string, data map[string]any) {
	_ = o.pub.Publish(ctx, events.New("operations", typ, subject, data))
}
