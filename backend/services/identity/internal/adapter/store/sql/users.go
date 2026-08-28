package sql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"unital/backend/services/identity/internal/domain"
)

// Users implements domain.UserStore and domain.RefreshStore using *sql.DB.
type Users struct {
	db *sql.DB
}

// NewUsers wraps an open *sql.DB connection.
func NewUsers(db *sql.DB) *Users { return &Users{db: db} }

// ── domain.UserStore ──────────────────────────────────────────

// Create inserts a new user row. It respects u.ID if non-empty (used by
// in-process callers that already have an ID), otherwise inserts with a
// server-generated UUID.
func (s *Users) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, full_name, phone,
		                   national_code, platform_role, email_verified,
		                   created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (email) DO NOTHING
	`
	id := u.ID
	if id == "" {
		id = s.genID(ctx)
	}
	_, err := s.db.ExecContext(ctx, query,
		id, u.Email, u.PasswordHash, u.FullName, u.Phone,
		u.NationalCode, u.PlatformRole, u.EmailVerified,
		u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return err
	}
	// ON CONFLICT DO NOTHING doesn't tell us if a row was inserted.
	// Check by looking up the user.
	if _, err := s.ByEmail(ctx, u.Email); err != nil {
		return domain.ErrEmailTaken
	}
	u.ID = id
	return nil
}

func (s *Users) ByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id,email,password_hash,full_name,phone,national_code,
		       platform_role,email_verified,created_at,updated_at
		FROM users WHERE email=$1
	`
	var u domain.User
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Phone,
		&u.NationalCode, &u.PlatformRole, &u.EmailVerified,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Users) ByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id,email,password_hash,full_name,phone,national_code,
		       platform_role,email_verified,created_at,updated_at
		FROM users WHERE id=$1
	`
	var u domain.User
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Phone,
		&u.NationalCode, &u.PlatformRole, &u.EmailVerified,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Users) Update(ctx context.Context, u *domain.User) error {
	query := `
		UPDATE users SET
			full_name=$2, phone=$3, national_code=$4,
			platform_role=$5, email_verified=$6, updated_at=$7
		WHERE id=$1
	`
	u.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, query,
		u.ID, u.FullName, u.Phone, u.NationalCode,
		u.PlatformRole, u.EmailVerified, u.UpdatedAt,
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

func (s *Users) SearchByPrefix(ctx context.Context, prefix string) ([]*domain.User, error) {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, password_hash, full_name, phone, national_code,
		       platform_role, email_verified, created_at, updated_at
		FROM users
		WHERE LOWER(email) LIKE $1
		ORDER BY email
		LIMIT 20
	`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Phone,
			&u.NationalCode, &u.PlatformRole, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &u)
	}
	return out, rows.Err()
}

// ── domain.RefreshStore ───────────────────────────────────────

func (s *Users) Save(ctx context.Context, tokenHash, userID string, expires time.Time) error {
	query := `INSERT INTO refresh_tokens (token_hash, user_id, expires_at) VALUES ($1,$2,$3)`
	_, err := s.db.ExecContext(ctx, query, tokenHash, userID, expires)
	return err
}

func (s *Users) FindToken(ctx context.Context, tokenHash string) (string, time.Time, error) {
	query := `SELECT user_id,expires_at FROM refresh_tokens WHERE token_hash=$1`
	var userID string
	var expires time.Time
	err := s.db.QueryRowContext(ctx, query, tokenHash).Scan(&userID, &expires)
	if err == sql.ErrNoRows {
		return "", time.Time{}, domain.ErrNotFound
	}
	if err != nil {
		return "", time.Time{}, err
	}
	if time.Now().After(expires) {
		_ = s.Delete(ctx, tokenHash)
		return "", time.Time{}, domain.ErrNotFound
	}
	return userID, expires, nil
}

func (s *Users) Delete(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1`, tokenHash)
	return err
}

// ── helpers ────────────────────────────────────────────────────

// genID returns a new server-generated UUID. Callers that already have an
// ID (from the nanoid-based ids.New()) use that instead.
func (s *Users) genID(ctx context.Context) string {
	var id string
	_ = s.db.QueryRowContext(ctx, `SELECT gen_random_uuid()::text`).Scan(&id)
	return id
}

// UpdatePlatformRole is a convenience method used by the internal admin handler.
func (s *Users) UpdatePlatformRole(ctx context.Context, id, role string) error {
	query := `UPDATE users SET platform_role=$2, updated_at=NOW() WHERE id=$1`
	res, err := s.db.ExecContext(ctx, query, id, role)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ByEmailForSeed looks up a user by email without returning domain errors
// so callers can distinguish "not found" from other DB errors.
func (s *Users) ByEmailForSeed(ctx context.Context, email string) (id string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
