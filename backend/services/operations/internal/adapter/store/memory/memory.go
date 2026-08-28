// Package memory provides in-memory operations stores (dev/tests).
package memory

import (
	"context"
	"sort"
	"sync"

	"unital/backend/services/operations/internal/domain"
)

type core struct {
	mu    sync.Mutex
	teams map[string]*domain.Team
	tasks map[string]*domain.Task
	reqs  map[string]*domain.ServiceRequest
	comms []*domain.Comment
	roles map[string]string
}

// Bundle groups all adapters over one core.
type Bundle struct {
	Teams      *TeamStore
	Tasks      *TaskStore
	Requests   *RequestStore
	Comments   *CommentStore
	Membership *MembershipTable
}

func New() Bundle {
	c := &core{
		teams: map[string]*domain.Team{},
		tasks: map[string]*domain.Task{},
		reqs:  map[string]*domain.ServiceRequest{},
		roles: map[string]string{},
	}
	return Bundle{
		Teams:      &TeamStore{c},
		Tasks:      &TaskStore{c},
		Requests:   &RequestStore{c},
		Comments:   &CommentStore{c},
		Membership: &MembershipTable{c},
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

type TeamStore struct{ *core }

func (s *TeamStore) Create(_ context.Context, t *domain.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.teams[t.ID] = t
	return nil
}

func (s *TeamStore) ByID(_ context.Context, id string) (*domain.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.teams[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *TeamStore) Update(_ context.Context, t *domain.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.teams[t.ID]; !ok {
		return domain.ErrNotFound
	}
	s.teams[t.ID] = t
	return nil
}

func (s *TeamStore) ListByBuilding(_ context.Context, buildingID string) ([]domain.Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Team
	for _, t := range s.teams {
		if t.BuildingID == buildingID {
			out = append(out, *t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

type TaskStore struct{ *core }

func (s *TaskStore) Create(_ context.Context, t *domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
	return nil
}

func (s *TaskStore) ByID(_ context.Context, id string) (*domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t, nil
}

func (s *TaskStore) Update(_ context.Context, t *domain.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[t.ID]; !ok {
		return domain.ErrNotFound
	}
	s.tasks[t.ID] = t
	return nil
}

func (s *TaskStore) List(_ context.Context, f domain.TaskFilter) ([]domain.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Task
	for _, t := range s.tasks {
		if f.TeamID != "" && t.TeamID != f.TeamID {
			continue
		}
		if f.Assignee != "" && t.AssignedTo != f.Assignee {
			continue
		}
		if f.Status != "" && t.Status != f.Status {
			continue
		}
		if f.BuildingID != "" && t.BuildingID != f.BuildingID {
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

type RequestStore struct{ *core }

func (s *RequestStore) Create(_ context.Context, r *domain.ServiceRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs[r.ID] = r
	return nil
}

func (s *RequestStore) ByID(_ context.Context, id string) (*domain.ServiceRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.reqs[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return r, nil
}

func (s *RequestStore) Update(_ context.Context, r *domain.ServiceRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.reqs[r.ID]; !ok {
		return domain.ErrNotFound
	}
	s.reqs[r.ID] = r
	return nil
}

func (s *RequestStore) List(_ context.Context, f domain.RequestFilter) ([]domain.ServiceRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.ServiceRequest
	for _, r := range s.reqs {
		if f.BuildingID != "" && r.BuildingID != f.BuildingID {
			continue
		}
		if f.Status != "" && r.Status != f.Status {
			continue
		}
		if f.SubmittedBy != "" && r.SubmittedBy != f.SubmittedBy {
			continue
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SubmittedAt.Before(out[j].SubmittedAt) })
	return out, nil
}

type CommentStore struct{ *core }

func (s *CommentStore) Create(_ context.Context, c *domain.Comment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.comms = append(s.comms, c)
	return nil
}

func (s *CommentStore) ByTask(_ context.Context, taskID string) ([]domain.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Comment
	for _, c := range s.comms {
		if c.TaskID == taskID {
			out = append(out, *c)
		}
	}
	return out, nil
}
