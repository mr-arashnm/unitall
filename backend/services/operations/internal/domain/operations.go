// Package domain holds operations' core model and ports.
package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("not allowed for this building")
	ErrInvalidState = errors.New("invalid state or input")
)

// Task lifecycle.
const (
	TaskPending    = "pending"
	TaskAssigned   = "assigned"
	TaskInProgress = "in_progress"
	TaskCompleted  = "completed"
	TaskCancelled  = "cancelled"
	TaskOnHold     = "on_hold"
)

// ServiceRequest lifecycle.
const (
	ReqSubmitted   = "submitted"
	ReqUnderReview = "under_review"
	ReqAssigned    = "assigned"
	ReqInProgress  = "in_progress"
	ReqCompleted   = "completed"
	ReqCancelled   = "cancelled"
)

type Team struct {
	ID         string    `json:"id"`
	BuildingID string    `json:"building_id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"` // security|maintenance|cleaning|landscaping|pool|gym|other
	Supervisor string    `json:"supervisor,omitempty"`
	Members    []string  `json:"members"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

type Task struct {
	ID             string     `json:"id"`
	TeamID         string     `json:"team_id"`
	BuildingID     string     `json:"building_id"`
	Title          string     `json:"title"`
	Description    string     `json:"description,omitempty"`
	AssignedTo     string     `json:"assigned_to,omitempty"`
	Priority       string     `json:"priority"` // low|medium|high|urgent
	Status         string     `json:"status"`
	DueDate        time.Time  `json:"due_date"`
	EstimatedHours int        `json:"estimated_hours,omitempty"`
	ActualHours    int        `json:"actual_hours,omitempty"`
	RelatedUnit    string     `json:"related_unit,omitempty"`
	CreatedBy      string     `json:"created_by"`
	AssignedAt     *time.Time `json:"assigned_at,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Comment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Author    string    `json:"author"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type ServiceRequest struct {
	ID           string     `json:"id"`
	BuildingID   string     `json:"building_id"`
	UnitID       string     `json:"unit_id"`
	SubmittedBy  string     `json:"submitted_by"`
	Title        string     `json:"title"`
	Description  string     `json:"description,omitempty"`
	Type         string     `json:"type"` // maintenance|cleaning|security|complaint|suggestion|other
	Priority     string     `json:"priority"`
	Status       string     `json:"status"`
	AssignedTeam string     `json:"assigned_team,omitempty"`
	RelatedTask  string     `json:"related_task,omitempty"`
	SubmittedAt  time.Time  `json:"submitted_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type Ports interface{}

type TeamStore interface {
	Create(ctx context.Context, t *Team) error
	ByID(ctx context.Context, id string) (*Team, error)
	Update(ctx context.Context, t *Team) error
	ListByBuilding(ctx context.Context, buildingID string) ([]Team, error)
}

type TaskStore interface {
	Create(ctx context.Context, t *Task) error
	ByID(ctx context.Context, id string) (*Task, error)
	Update(ctx context.Context, t *Task) error
	List(ctx context.Context, filter TaskFilter) ([]Task, error)
}

type TaskFilter struct {
	TeamID, Assignee, Status, BuildingID string
}

type CommentStore interface {
	Create(ctx context.Context, c *Comment) error
	ByTask(ctx context.Context, taskID string) ([]Comment, error)
}

type RequestStore interface {
	Create(ctx context.Context, r *ServiceRequest) error
	ByID(ctx context.Context, id string) (*ServiceRequest, error)
	Update(ctx context.Context, r *ServiceRequest) error
	List(ctx context.Context, filter RequestFilter) ([]ServiceRequest, error)
}

type RequestFilter struct {
	BuildingID, Status, SubmittedBy string
}

type MembershipChecker interface {
	HasRole(ctx context.Context, userID, buildingID, role string) (bool, error)
	AnyRole(ctx context.Context, userID, buildingID string, roles ...string) (bool, error)
}
