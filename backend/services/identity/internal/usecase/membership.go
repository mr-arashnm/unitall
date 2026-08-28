package usecase

import (
	"context"
	"time"

	"unital/backend/pkg/ids"
	"unital/backend/services/identity/internal/domain"
)

// Memberships manages user↔building roles. Public grants require the
// caller to already be a manager member of the building; Bootstrap is
// the internal path used by property at building creation.
type Memberships struct {
	members domain.MembershipStore
	users   domain.UserStore
}

func NewMemberships(m domain.MembershipStore, u domain.UserStore) *Memberships {
	return &Memberships{members: m, users: u}
}

func (m *Memberships) Grant(ctx context.Context, actorUserID, buildingID, targetUserID, role string) (*domain.Membership, error) {
	mm, err := m.members.Find(ctx, actorUserID, buildingID)
	if err != nil {
		return nil, domain.ErrForbidden
	}
	if mm.Role != domain.RoleManager {
		return nil, domain.ErrForbidden
	}
	return m.grant(ctx, buildingID, targetUserID, role)
}

// Bootstrap grants a membership without an existing manager — called
// only from the gateway-protected internal route at building creation.
func (m *Memberships) Bootstrap(ctx context.Context, buildingID, targetUserID, role string) (*domain.Membership, error) {
	return m.grant(ctx, buildingID, targetUserID, role)
}

func (m *Memberships) grant(ctx context.Context, buildingID, targetUserID, role string) (*domain.Membership, error) {
	if !domain.ValidRole(role) {
		return nil, domain.ErrInvalidRole
	}
	if _, err := m.users.ByID(ctx, targetUserID); err != nil {
		return nil, err
	}
	if existing, err := m.members.Find(ctx, targetUserID, buildingID); err == nil {
		return existing, nil // idempotent
	}
	mem := &domain.Membership{ID: ids.New(), UserID: targetUserID, BuildingID: buildingID, Role: role, From: time.Now().UTC()}
	if err := m.members.Grant(ctx, mem); err != nil {
		return nil, err
	}
	return mem, nil
}

func (m *Memberships) Revoke(ctx context.Context, actorUserID, buildingID, membershipID string) error {
	mm, err := m.members.Find(ctx, actorUserID, buildingID)
	if err != nil {
		return domain.ErrForbidden
	}
	if mm.Role != domain.RoleManager {
		return domain.ErrForbidden
	}
	return m.members.Revoke(ctx, membershipID)
}

func (m *Memberships) Mine(ctx context.Context, userID string) ([]domain.Membership, error) {
	return m.members.ByUser(ctx, userID)
}

func (m *Memberships) ByBuilding(ctx context.Context, actorUserID, buildingID string) ([]domain.Membership, error) {
	if _, err := m.members.Find(ctx, actorUserID, buildingID); err != nil {
		return nil, domain.ErrForbidden
	}
	return m.members.ByBuilding(ctx, buildingID)
}
