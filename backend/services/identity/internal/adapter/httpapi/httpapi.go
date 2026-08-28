// Package httpapi exposes identity's REST surface (see docs/API_DESIGN.md).
// Handlers do parse → usecase → respond; no business logic lives here.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/identity/internal/domain"
	"unital/backend/services/identity/internal/usecase"
)

type API struct {
	auth    *usecase.Auth
	members *usecase.Memberships
	signer  *jwtx.Signer
	base    string // problem namespace
}

func New(auth *usecase.Auth, members *usecase.Memberships, signer *jwtx.Signer) *API {
	return &API{auth: auth, members: members, signer: signer, base: "identity"}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.login)
	mux.HandleFunc("POST /auth/refresh", a.refresh)
	mux.HandleFunc("POST /auth/logout", a.logout)
	mux.HandleFunc("POST /auth/verify", a.verify)
	mux.HandleFunc("POST /auth/password-reset", a.requestReset)
	mux.HandleFunc("POST /auth/password-reset/confirm", a.confirmReset)
	mux.HandleFunc("GET /me", a.me)
	mux.HandleFunc("PATCH /me", a.patchMe)
	mux.HandleFunc("GET /me/buildings", a.myBuildings)
	mux.HandleFunc("POST /buildings/{buildingID}/memberships", a.grantMembership)
	mux.HandleFunc("GET /buildings/{buildingID}/memberships", a.listMemberships)
	mux.HandleFunc("DELETE /buildings/{buildingID}/memberships/{membershipID}", a.revokeMembership)
	// internal (gateway-only in deployment; bound to a separate listener)
	mux.HandleFunc("GET /internal/users", a.searchUsers)
	mux.HandleFunc("POST /internal/users/invite", a.inviteUser)
	mux.HandleFunc("GET /internal/users/{userID}", a.internalUser)
	mux.HandleFunc("PATCH /internal/users/{userID}/platform-role", a.setPlatformRole)
	mux.HandleFunc("POST /internal/buildings/{buildingID}/bootstrap-manager", a.bootstrapManager)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { httpx.JSON(w, 200, map[string]string{"status": "ok"}) })
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := httpx.NewProblem(a.base, "INTERNAL", "Internal server error", http.StatusInternalServerError)
	switch {
	case errors.Is(err, domain.ErrEmailTaken):
		p = httpx.NewProblem(a.base, "EMAIL_TAKEN", "Email already registered", http.StatusConflict)
	case errors.Is(err, domain.ErrBadCredentials):
		p = httpx.NewProblem(a.base, "BAD_CREDENTIALS", "Invalid credentials", http.StatusUnauthorized)
	case errors.Is(err, domain.ErrEmailUnverified):
		p = httpx.NewProblem(a.base, "EMAIL_UNVERIFIED", "Email not verified", http.StatusForbidden)
	case errors.Is(err, domain.ErrNotFound):
		p = httpx.NewProblem(a.base, "NOT_FOUND", "Resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		p = httpx.NewProblem(a.base, "FORBIDDEN", "Not allowed", http.StatusForbidden)
	case errors.Is(err, domain.ErrInvalidRole), errors.Is(err, usecase.ErrWeakPassword):
		p = httpx.NewProblem(a.base, "INVALID_INPUT", err.Error(), http.StatusUnprocessableEntity)
	}
	httpx.WriteError(w, r, p)
}

type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
		Phone    string `json:"phone"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	u, err := a.auth.Register(r.Context(), usecase.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
		Phone:    req.Phone,
	})
	if err != nil {
		slog.Error("register failed", "err", err, "email", req.Email)
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoUser(u))
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Email == "" || req.Password == "" {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "BAD_CREDENTIALS", "Invalid credentials", http.StatusUnauthorized))
		return
	}
	u, err := a.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.writeTokens(w, r, u)
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.RefreshToken == "" {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "BAD_CREDENTIALS", "refresh_token required", http.StatusUnauthorized))
		return
	}
	uid, newTok, err := a.auth.Rotate(r.Context(), req.RefreshToken)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	u, err := a.auth.User(r.Context(), uid)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	a.writeTokens(w, r, u, newTok)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = httpx.Decode(w, r, &req) // missing token is a no-op logout
	if req.RefreshToken != "" {
		if err := a.auth.Logout(r.Context(), req.RefreshToken); err != nil {
			a.fail(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (a *API) verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Token == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "token", Message: "required"}))
		return
	}
	if err := a.auth.Verify(r.Context(), req.Token); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (a *API) requestReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Email == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "email", Message: "required"}))
		return
	}
	// always 200: never reveal whether the account exists
	_ = a.auth.RequestPasswordReset(r.Context(), req.Email)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "reset_sent"})
}

func (a *API) confirmReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Token == "" || req.Password == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "token", Message: "required"},
			httpx.Validation{Field: "password", Message: "required"}))
		return
	}
	if err := a.auth.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func (a *API) writeTokens(w http.ResponseWriter, r *http.Request, u *domain.User, refreshOverride ...string) {
	access, err := a.signer.Issue(u.ID, u.PlatformRole, u.EmailVerified)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	refresh := ""
	if len(refreshOverride) > 0 {
		refresh = refreshOverride[0]
	} else {
		refresh, _, err = a.auth.IssueRefresh(r.Context(), u.ID)
		if err != nil {
			a.fail(w, r, err)
			return
		}
	}
	httpx.JSON(w, http.StatusOK, tokenPair{
		AccessToken: access, RefreshToken: refresh,
		TokenType: "Bearer", ExpiresIn: int(a.signer.TTL().Seconds()),
	})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	u, err := a.auth.User(r.Context(), uid)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoUser(u))
}

func (a *API) patchMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		FullName     string `json:"full_name"`
		Phone        string `json:"phone"`
		NationalCode string `json:"national_code"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	u, err := a.auth.UpdateProfile(r.Context(), uid, req.FullName, req.Phone, req.NationalCode)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoUser(u))
}

func (a *API) myBuildings(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	ms, err := a.members.Mine(r.Context(), uid)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(ms))
	for _, m := range ms {
		out = append(out, map[string]any{"building_id": m.BuildingID, "role": m.Role, "member_since": m.From})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) grantMembership(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.UserID == "" || req.Role == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "user_id", Message: "required"},
			httpx.Validation{Field: "role", Message: "required"}))
		return
	}
	m, err := a.members.Grant(r.Context(), uid, r.PathValue("buildingID"), req.UserID, req.Role)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoMembership(m))
}

func (a *API) listMemberships(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	ms, err := a.members.ByBuilding(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]domain.Membership, 0, len(ms))
	out = append(out, ms...)
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) revokeMembership(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if err := a.members.Revoke(r.Context(), uid, r.PathValue("buildingID"), r.PathValue("membershipID")); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (a *API) internalUser(w http.ResponseWriter, r *http.Request) {
	u, err := a.auth.User(r.Context(), r.PathValue("userID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoUser(u))
}

func (a *API) bootstrapManager(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.UserID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "user_id", Message: "required"}))
		return
	}
	m, err := a.members.Bootstrap(r.Context(), r.PathValue("buildingID"), req.UserID, domain.RoleManager)
	if err != nil {
		slog.Error("bootstrap manager failed", "err", err, "building_id", r.PathValue("buildingID"), "user_id", req.UserID)
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoMembership(m))
}

// setPlatformRole assigns or revokes a platform role for a user.
// Called by the seed CLI or other internal tools via the gateway internal token.
func (a *API) setPlatformRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlatformRole string `json:"platform_role"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	u, err := a.auth.UpdatePlatformRole(r.Context(), r.PathValue("userID"), req.PlatformRole)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoUser(u))
}

// searchUsers looks up users by email prefix. The property service uses
// this when assigning an owner/resident to a unit.
func (a *API) searchUsers(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("email")
	users, err := a.auth.SearchUsers(r.Context(), prefix)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, dtoUser(u))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// inviteUser creates a stub user (or returns the existing one) and fires
// a verification email. Used by the property service when a manager
// assigns a unit to a person who isn't registered yet.
func (a *API) inviteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	u, err := a.auth.InviteByEmail(r.Context(), req.Email)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoUser(u))
}

// userID extracts the caller from the gateway-injected header or a
// direct bearer token (service standalone mode).
func (a *API) userID(r *http.Request) (string, bool) {
	if id := r.Header.Get("X-User-Id"); id != "" {
		return id, true
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	claims, err := a.signer.Parse(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		return "", false
	}
	return claims.Sub, true
}

func dtoUser(u *domain.User) map[string]any {
	return map[string]any{
		"id": u.ID, "email": u.Email, "full_name": u.FullName,
		"phone": u.Phone, "national_code": u.NationalCode,
		"platform_role": u.PlatformRole, "email_verified": u.EmailVerified,
		"created_at": u.CreatedAt,
	}
}

func dtoMembership(m *domain.Membership) map[string]any {
	return map[string]any{"id": m.ID, "user_id": m.UserID, "building_id": m.BuildingID, "role": m.Role, "from": m.From}
}

// NewServer wires middleware around the mux (used by main and tests).
func NewServer(api *API) http.Handler {
	mux := http.NewServeMux()
	api.Routes(mux)
	return httpx.Chain(mux, httpx.RequestID, httpx.AccessLog, httpx.Recover)
}
