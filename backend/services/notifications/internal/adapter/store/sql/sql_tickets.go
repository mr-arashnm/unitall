// Package sql provides a PostgreSQL adapter for notifications stores.
package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"unital/backend/services/notifications/internal/domain"
)

// ── TicketStore ────────────────────────────────────────────────

type TicketStore struct {
	db *sql.DB
}

func NewTicketStore(db *sql.DB) *TicketStore { return &TicketStore{db: db} }

func (s *TicketStore) Create(ctx context.Context, t *domain.Ticket) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO tickets (id,building_id,unit_id,submitted_by,title,description,type,priority,status,assigned_to,team_id,submitted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.BuildingID, t.UnitID, t.SubmittedBy, t.Title, t.Description,
		t.Type, t.Priority, t.Status, t.AssignedTo, t.TeamID, t.SubmittedAt,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *TicketStore) ByID(ctx context.Context, id string) (*domain.Ticket, error) {
	var t domain.Ticket
	var unitID, submittedBy, assignedTo, teamID, desc sql.NullString
	var resolvedAt, closedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id,building_id,unit_id,submitted_by,title,description,type,priority,status,assigned_to,team_id,submitted_at,resolved_at,closed_at
		FROM tickets WHERE id=$1`,
		id,
	).Scan(&t.ID, &t.BuildingID, &unitID, &submittedBy, &t.Title, &desc, &t.Type, &t.Priority, &t.Status, &assignedTo, &teamID, &t.SubmittedAt, &resolvedAt, &closedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.UnitID = unitID.String
	t.SubmittedBy = submittedBy.String
	t.AssignedTo = assignedTo.String
	t.TeamID = teamID.String
	t.Description = desc.String
	if resolvedAt.Valid {
		t.ResolvedAt = &resolvedAt.Time
	}
	if closedAt.Valid {
		t.ClosedAt = &closedAt.Time
	}
	return &t, nil
}

func (s *TicketStore) Update(ctx context.Context, t *domain.Ticket) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE tickets SET
			unit_id=$2, title=$3, description=$4, type=$5, priority=$6,
			status=$7, assigned_to=$8, team_id=$9, resolved_at=$10, closed_at=$11
		WHERE id=$1`,
		t.ID, t.UnitID, t.Title, t.Description, t.Type, t.Priority,
		t.Status, t.AssignedTo, t.TeamID, t.ResolvedAt, t.ClosedAt,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *TicketStore) List(ctx context.Context, f domain.TicketFilter) ([]domain.Ticket, error) {
	query := `SELECT id,building_id,unit_id,submitted_by,title,description,type,priority,status,assigned_to,team_id,submitted_at,resolved_at,closed_at FROM tickets WHERE 1=1`
	args := []any{}
	arg := 1

	if f.BuildingID != "" {
		query += ` AND building_id = $` + itoa(arg)
		args = append(args, f.BuildingID)
		arg++
	}
	if f.Status != "" {
		query += ` AND status = $` + itoa(arg)
		args = append(args, f.Status)
		arg++
	}
	if f.SubmittedBy != "" {
		query += ` AND submitted_by = $` + itoa(arg)
		args = append(args, f.SubmittedBy)
		arg++
	}
	query += ` ORDER BY submitted_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Ticket
	for rows.Next() {
		var t domain.Ticket
		var unitID, submittedBy, assignedTo, teamID, desc sql.NullString
		var resolvedAt, closedAt sql.NullTime
		if err := rows.Scan(&t.ID, &t.BuildingID, &unitID, &submittedBy, &t.Title, &desc, &t.Type, &t.Priority, &t.Status, &assignedTo, &teamID, &t.SubmittedAt, &resolvedAt, &closedAt); err != nil {
			return nil, err
		}
		t.UnitID = unitID.String
		t.SubmittedBy = submittedBy.String
		t.AssignedTo = assignedTo.String
		t.TeamID = teamID.String
		t.Description = desc.String
		if resolvedAt.Valid {
			t.ResolvedAt = &resolvedAt.Time
		}
		if closedAt.Valid {
			t.ClosedAt = &closedAt.Time
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ── TicketResponseStore ────────────────────────────────────────

type TicketResponseStore struct {
	db *sql.DB
}

func NewTicketResponseStore(db *sql.DB) *TicketResponseStore { return &TicketResponseStore{db: db} }

func (s *TicketResponseStore) Create(ctx context.Context, r *domain.TicketResponse) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ticket_responses (id,ticket_id,author,message,internal,created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		r.ID, r.TicketID, r.Author, r.Message, r.Internal, r.CreatedAt,
	)
	return err
}

func (s *TicketResponseStore) ByTicket(ctx context.Context, ticketID string) ([]domain.TicketResponse, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,ticket_id,author,message,internal,created_at
		FROM ticket_responses WHERE ticket_id=$1 ORDER BY created_at ASC`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.TicketResponse
	for rows.Next() {
		var r domain.TicketResponse
		if err := rows.Scan(&r.ID, &r.TicketID, &r.Author, &r.Message, &r.Internal, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// itoa formats an integer for use in a query placeholder index.
func itoa(i int) string { return fmt.Sprintf("%d", i) }

// SafeIsNotFound returns true if err is sql.ErrNoRows.
func SafeIsNotFound(err error) bool {
	return strings.Contains(err.Error(), "no rows")
}
