// Package memory provides in-memory identity stores (dev & tests).
// The postgres adapter implements the same domain ports.
package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"unital/backend/services/identity/internal/domain"
)

// Users implements domain.UserStore and domain.RefreshStore.
type Users struct {
	mu     sync.RWMutex
	users  map[string]*domain.User
	byMail map[string]string
	tokens map[string]tokenRec
}

type tokenRec struct {
	userID  string
	expires time.Time
}

func NewUsers() *Users {
	return &Users{users: map[string]*domain.User{}, byMail: map[string]string{}, tokens: map[string]tokenRec{}}
}

func (s *Users) Create(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byMail[u.Email]; ok {
		return domain.ErrEmailTaken
	}
	s.users[u.ID] = u
	s.byMail[u.Email] = u.ID
	return nil
}

func (s *Users) ByEmail(_ context.Context, email string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byMail[email]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s.users[id], nil
}

func (s *Users) ByID(_ context.Context, id string) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

func (s *Users) Update(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.ID]; !ok {
		return domain.ErrNotFound
	}
	s.users[u.ID] = u
	return nil
}

func (s *Users) SearchByPrefix(_ context.Context, prefix string) ([]*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" {
		return nil, nil
	}
	var out []*domain.User
	for _, u := range s.users {
		if strings.HasPrefix(strings.ToLower(u.Email), prefix) {
			out = append(out, u)
			if len(out) >= 20 {
				break
			}
		}
	}
	return out, nil
}

func (s *Users) Save(_ context.Context, hash, userID string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[hash] = tokenRec{userID: userID, expires: expires}
	return nil
}

func (s *Users) FindToken(_ context.Context, hash string) (string, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.tokens[hash]
	if !ok || time.Now().After(rec.expires) {
		return "", time.Time{}, domain.ErrNotFound
	}
	return rec.userID, rec.expires, nil
}

func (s *Users) Delete(_ context.Context, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, hash)
	return nil
}

// Memberships implements domain.MembershipStore.
type Memberships struct {
	mu   sync.RWMutex
	byID map[string]*domain.Membership
}

func NewMemberships() *Memberships { return &Memberships{byID: map[string]*domain.Membership{}} }

func (s *Memberships) Grant(_ context.Context, m *domain.Membership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[m.ID] = m
	return nil
}

func (s *Memberships) Revoke(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.byID, id)
	return nil
}

func (s *Memberships) ByUser(_ context.Context, userID string) ([]domain.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Membership
	for _, m := range s.byID {
		if m.UserID == userID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (s *Memberships) ByBuilding(_ context.Context, buildingID string) ([]domain.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []domain.Membership
	for _, m := range s.byID {
		if m.BuildingID == buildingID {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (s *Memberships) Find(_ context.Context, userID, buildingID string) (*domain.Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.byID {
		if m.UserID == userID && m.BuildingID == buildingID {
			cp := *m
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

// LogMailer satisfies domain.Mailer for dev: drops mail on the floor.
type LogMailer struct{}

func (LogMailer) SendVerification(_ context.Context, to, token string) error { return nil }

func (LogMailer) SendPasswordReset(_ context.Context, to, token string) error { return nil }
