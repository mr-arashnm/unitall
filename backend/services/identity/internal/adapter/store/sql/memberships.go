package sql

import (
	"context"
	"database/sql"

	"unital/backend/services/identity/internal/domain"
)

// Memberships implements domain.MembershipStore using *sql.DB.
type Memberships struct {
	db *sql.DB
}

// NewMemberships wraps an open *sql.DB connection.
func NewMemberships(db *sql.DB) *Memberships { return &Memberships{db: db} }

func (s *Memberships) Grant(ctx context.Context, m *domain.Membership) error {
	query := `
		INSERT INTO memberships (id, user_id, building_id, role, created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (user_id, building_id)
		DO UPDATE SET role=EXCLUDED.role
	`
	id := m.ID
	if id == "" {
		_ = s.db.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&id)
	}
	_, err := s.db.ExecContext(ctx, query, id, m.UserID, m.BuildingID, m.Role, m.From)
	if err != nil {
		return err
	}
	m.ID = id
	return nil
}

func (s *Memberships) Revoke(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memberships WHERE id=$1`, id)
	return err
}

func (s *Memberships) ByUser(ctx context.Context, userID string) ([]domain.Membership, error) {
	query := `
		SELECT id,user_id,building_id,role,created_at
		FROM memberships WHERE user_id=$1 ORDER BY created_at
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		var m domain.Membership
		if err := rows.Scan(&m.ID, &m.UserID, &m.BuildingID, &m.Role, &m.From); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Memberships) ByBuilding(ctx context.Context, buildingID string) ([]domain.Membership, error) {
	query := `
		SELECT id,user_id,building_id,role,created_at
		FROM memberships WHERE building_id=$1 ORDER BY created_at
	`
	rows, err := s.db.QueryContext(ctx, query, buildingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		var m domain.Membership
		if err := rows.Scan(&m.ID, &m.UserID, &m.BuildingID, &m.Role, &m.From); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Memberships) Find(ctx context.Context, userID, buildingID string) (*domain.Membership, error) {
	query := `
		SELECT id,user_id,building_id,role,created_at
		FROM memberships WHERE user_id=$1 AND building_id=$2
	`
	var m domain.Membership
	err := s.db.QueryRowContext(ctx, query, userID, buildingID).Scan(
		&m.ID, &m.UserID, &m.BuildingID, &m.Role, &m.From,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
