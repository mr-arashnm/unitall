// Package domain holds identity's core model: users, building
// memberships, and the ports the usecases depend on. No framework
// imports allowed here.
package domain

import (
	"context"
	"errors"
	"time"
)

// Platform roles.
const (
	RoleManager     = "manager"
	RoleBoardMember = "board_member"
	RoleStaff       = "staff"
	RoleOwner       = "owner"
	RoleResident    = "resident"
)

func ValidRole(r string) bool {
	switch r {
	case RoleManager, RoleBoardMember, RoleStaff, RoleOwner, RoleResident:
		return true
	}
	return false
}

// User is an account. Password is stored as an opaque pbkdf2 string.
type User struct {
	ID            string
	Email         string
	PasswordHash  string
	FullName      string
	Phone         string
	NationalCode  string
	PlatformRole  string
	EmailVerified bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Membership links a user to a building with a role (validity ranges
// arrive in the persistence adapter; current memberships have To=nil).
type Membership struct {
	ID         string
	UserID     string
	BuildingID string
	Role       string // per-building role: manager|board_member|staff|owner|resident
	From       time.Time
}

var (
	ErrEmailTaken      = errors.New("email already registered")
	ErrNotFound        = errors.New("not found")
	ErrBadCredentials  = errors.New("invalid email or password")
	ErrEmailUnverified = errors.New("email not verified")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidRole     = errors.New("invalid role")
)

// UserStore persists users.
type UserStore interface {
	Create(ctx context.Context, u *User) error
	ByEmail(ctx context.Context, email string) (*User, error)
	ByID(ctx context.Context, id string) (*User, error)
	Update(ctx context.Context, u *User) error
	// SearchByPrefix returns up to 20 users whose email starts with prefix
	// (case-insensitive). Used by the property service to find a user when
	// assigning ownership/residency.
	SearchByPrefix(ctx context.Context, prefix string) ([]*User, error)
}

// RefreshStore stores hashed refresh tokens for rotation/revocation.
type RefreshStore interface {
	Save(ctx context.Context, tokenHash, userID string, expires time.Time) error
	FindToken(ctx context.Context, tokenHash string) (userID string, expires time.Time, err error)
	Delete(ctx context.Context, tokenHash string) error
}

// MembershipStore persists building memberships.
type MembershipStore interface {
	Grant(ctx context.Context, m *Membership) error
	Revoke(ctx context.Context, id string) error
	ByUser(ctx context.Context, userID string) ([]Membership, error)
	ByBuilding(ctx context.Context, buildingID string) ([]Membership, error)
	Find(ctx context.Context, userID, buildingID string) (*Membership, error)
}

// Mailer sends transactional email (verification, reset). Dev adapter logs.
type Mailer interface {
	SendVerification(ctx context.Context, to, token string) error
	SendPasswordReset(ctx context.Context, to, token string) error
}
