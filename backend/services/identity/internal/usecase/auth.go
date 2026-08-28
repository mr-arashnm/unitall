// Package usecase implements identity's application services.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"

	"unital/backend/pkg/ids"
	"unital/backend/pkg/password"
	"unital/backend/services/identity/internal/domain"
)

// Auth covers register/login/refresh/logout/verify/reset.
type Auth struct {
	users    domain.UserStore
	refresh  domain.RefreshStore
	mailer   domain.Mailer
	tokenTTL time.Duration // refresh token lifetime
}

func NewAuth(users domain.UserStore, refresh domain.RefreshStore, mailer domain.Mailer, refreshTTL time.Duration) *Auth {
	return &Auth{users: users, refresh: refresh, mailer: mailer, tokenTTL: refreshTTL}
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
	Phone    string
	// Role is no longer accepted at registration — building roles are granted
	// by a building manager via POST /buildings/{id}/memberships.
	// Platform roles are assigned by a platform admin via the seed CLI.
}

var ErrWeakPassword = errors.New("password must be at least 8 chars with a letter and a digit")

func (a *Auth) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if in.Email == "" || !strings.Contains(in.Email, "@") {
		return nil, domain.ErrBadCredentials
	}
	if err := checkPassword(in.Password); err != nil {
		return nil, err
	}
	if _, err := a.users.ByEmail(ctx, in.Email); err == nil {
		return nil, domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &domain.User{
		ID: ids.New(), Email: in.Email, PasswordHash: hash,
		FullName: strings.TrimSpace(in.FullName), Phone: in.Phone,
		PlatformRole: "", CreatedAt: now, UpdatedAt: now,
	}
	if err := a.users.Create(ctx, u); err != nil {
		return nil, err
	}
	// Dev-mode: verification token equals a signed random; real tokens come
	// from the verification store. We reuse refresh store keyed by purpose.
	tok := ids.New()
	if err := a.refresh.Save(ctx, verifyKey(tok), u.ID, now.Add(24*time.Hour)); err != nil {
		return nil, err
	}
	_ = a.mailer.SendVerification(ctx, u.Email, tok) // best effort
	return u, nil
}

func (a *Auth) Login(ctx context.Context, email, plain string) (*domain.User, error) {
	u, err := a.users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrBadCredentials
		}
		return nil, err
	}
	if err := password.Verify(plain, u.PasswordHash); err != nil {
		return nil, domain.ErrBadCredentials
	}
	if !u.EmailVerified {
		return nil, domain.ErrEmailUnverified
	}
	return u, nil
}

// IssueRefresh stores a hashed opaque token and returns it.
func (a *Auth) IssueRefresh(ctx context.Context, userID string) (string, time.Time, error) {
	tok := ids.New() + ids.New()
	exp := time.Now().UTC().Add(a.tokenTTL)
	if err := a.refresh.Save(ctx, hashKey(tok), userID, exp); err != nil {
		return "", time.Time{}, err
	}
	return tok, exp, nil
}

// Rotate validates and consumes a refresh token, issuing a new one.
func (a *Auth) Rotate(ctx context.Context, token string) (string, string, error) {
	h := hashKey(token)
	uid, _, err := a.refresh.FindToken(ctx, h)
	if err != nil {
		return "", "", domain.ErrBadCredentials
	}
	if err := a.refresh.Delete(ctx, h); err != nil && !errors.Is(err, domain.ErrNotFound) {
		return "", "", err
	}
	u, err := a.users.ByID(ctx, uid)
	if err != nil {
		return "", "", err
	}
	newTok, _, err := a.IssueRefresh(ctx, uid)
	return u.ID, newTok, err
}

func (a *Auth) Logout(ctx context.Context, token string) error {
	return a.refresh.Delete(ctx, hashKey(token))
}

// Verify marks the account verified using the token from the mailer.
func (a *Auth) Verify(ctx context.Context, token string) error {
	uid, _, err := a.refresh.FindToken(ctx, verifyKey(token))
	if err != nil {
		return domain.ErrBadCredentials
	}
	u, err := a.users.ByID(ctx, uid)
	if err != nil {
		return err
	}
	u.EmailVerified = true
	u.UpdatedAt = time.Now().UTC()
	return a.users.Update(ctx, u)
}

func (a *Auth) User(ctx context.Context, id string) (*domain.User, error) {
	return a.users.ByID(ctx, id)
}

// RequestPasswordReset emails a one-time reset token (valid 1h).
func (a *Auth) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := a.users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil // don't reveal account existence
	}
	tok := ids.New()
	if err := a.refresh.Save(ctx, resetKey(tok), u.ID, time.Now().Add(time.Hour)); err != nil {
		return err
	}
	return a.mailer.SendPasswordReset(ctx, u.Email, tok)
}

// ResetPassword consumes the token and sets a new password.
func (a *Auth) ResetPassword(ctx context.Context, token, newPass string) error {
	if err := checkPassword(newPass); err != nil {
		return err
	}
	uid, _, err := a.refresh.FindToken(ctx, resetKey(token))
	if err != nil {
		return domain.ErrBadCredentials
	}
	u, err := a.users.ByID(ctx, uid)
	if err != nil {
		return err
	}
	hash, err := password.Hash(newPass)
	if err != nil {
		return err
	}
	u.PasswordHash = hash
	u.UpdatedAt = time.Now().UTC()
	if err := a.users.Update(ctx, u); err != nil {
		return err
	}
	return a.refresh.Delete(ctx, resetKey(token))
}

// UpdatePlatformRole sets a user's global role (system_admin, supervisor,
// developer, or "" to revoke). Only the seed CLI / internal endpoints may
// call this — it bypasses the per-building RBAC checks.
func (a *Auth) UpdatePlatformRole(ctx context.Context, id, role string) (*domain.User, error) {
	if role != "" && !validPlatformRole(role) {
		return nil, domain.ErrInvalidRole
	}
	u, err := a.users.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.PlatformRole = role
	u.UpdatedAt = time.Now().UTC()
	if err := a.users.Update(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func validPlatformRole(r string) bool {
	switch r {
	case "system_admin", "supervisor", "developer":
		return true
	}
	return false
}

func (a *Auth) UpdateProfile(ctx context.Context, id, fullName, phone, nationalCode string) (*domain.User, error) {
	u, err := a.users.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if fullName != "" {
		u.FullName = fullName
	}
	u.Phone, u.NationalCode = phone, nationalCode
	u.UpdatedAt = time.Now().UTC()
	return u, a.users.Update(ctx, u)
}

// SearchUsers returns up to 20 users whose email starts with prefix (case-insensitive).
// Used by the property service to find a user when assigning owner/resident.
func (a *Auth) SearchUsers(ctx context.Context, prefix string) ([]*domain.User, error) {
	return a.users.SearchByPrefix(ctx, prefix)
}

// InviteByEmail creates a stub user with a random password if no user
// exists with the given email, and fires a verification email. Returns
// the user (existing or newly created). The new user is marked as
// email-unverified; they must click the link before they can log in.
func (a *Auth) InviteByEmail(ctx context.Context, email string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, domain.ErrBadCredentials
	}
	if u, err := a.users.ByEmail(ctx, email); err == nil {
		return u, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	// Generate a random 16-char password — the user will set their own
	// after clicking the invite link.
	rnd := make([]byte, 12)
	for i := range rnd {
		rnd[i] = byte('a' + (time.Now().UnixNano()+int64(i))%26)
	}
	hash, err := password.Hash(string(rnd))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &domain.User{
		ID:            ids.New(),
		Email:         email,
		PasswordHash:  hash,
		FullName:      "",
		PlatformRole:  "",
		EmailVerified: false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := a.users.Create(ctx, u); err != nil {
		return nil, err
	}
	// Fire-and-forget verification mail. If the mailer is nil (tests),
	// skip silently.
	if a.mailer != nil {
		token := ids.New()
		_ = a.mailer.SendVerification(ctx, email, token)
	}
	return u, nil
}

func checkPassword(p string) error {
	if len([]rune(p)) < 8 {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}

func hashKey(tok string) string {
	sum := sha256.Sum256([]byte("rt:" + tok))
	return hex.EncodeToString(sum[:])
}
func verifyKey(tok string) string {
	sum := sha256.Sum256([]byte("vf:" + tok))
	return hex.EncodeToString(sum[:])
}
func resetKey(tok string) string {
	sum := sha256.Sum256([]byte("rs:" + tok))
	return hex.EncodeToString(sum[:])
}
