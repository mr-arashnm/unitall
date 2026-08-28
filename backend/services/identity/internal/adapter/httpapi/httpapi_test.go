package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"unital/backend/pkg/jwtx"
	"unital/backend/services/identity/internal/adapter/store/memory"
	"unital/backend/services/identity/internal/usecase"
)

// capturingMailer records verification tokens per email.
type capturingMailer struct {
	mu     sync.Mutex
	tokens map[string]string
}

func newCapturingMailer() *capturingMailer { return &capturingMailer{tokens: map[string]string{}} }

func (c *capturingMailer) SendVerification(_ context.Context, to, token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[to] = token
	return nil
}

func (c *capturingMailer) SendPasswordReset(_ context.Context, to, token string) error { return nil }

func (c *capturingMailer) tokenFor(email string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens[email]
}

func newTestServer(t *testing.T) (*httptest.Server, *jwtx.Signer, *capturingMailer) {
	t.Helper()
	users := memory.NewUsers()
	mships := memory.NewMemberships()
	mailer := newCapturingMailer()
	auth := usecase.NewAuth(users, users, mailer, time.Hour)
	members := usecase.NewMemberships(mships, users)
	signer := jwtx.NewSigner("test-secret", 15*time.Minute)
	ts := httptest.NewServer(NewServer(New(auth, members, signer)))
	t.Cleanup(ts.Close)
	return ts, signer, mailer
}

func do(t *testing.T, ts *httptest.Server, method, path string, body any, token string) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, ts.URL+path, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// registerAndVerify creates an account, completes verification, and logs in.
func registerAndVerify(t *testing.T, ts *httptest.Server, mailer *capturingMailer, email string) (string, string) {
	t.Helper()
	resp, regBody := do(t, ts, "POST", "/auth/register", map[string]string{
		"email": email, "password": "secret12",
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register %s: %d %v", email, resp.StatusCode, regBody)
	}
	// Confirm the new contract: registration does not accept role.
	if _, ok := regBody["platform_role"]; ok {
		// The field is present but should be empty.
		if pr := regBody["platform_role"].(string); pr != "" {
			t.Errorf("expected empty platform_role, got %q", pr)
		}
	}
	resp, body := do(t, ts, "POST", "/auth/verify", map[string]string{"token": mailer.tokenFor(email)}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("verify %s: %d %v", email, resp.StatusCode, body)
	}
	resp, body = do(t, ts, "POST", "/auth/login", map[string]string{"email": email, "password": "secret12"}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login %s: %d %v", email, resp.StatusCode, body)
	}
	return regBody["id"].(string), body["access_token"].(string)
}

func TestRegisterLoginMeFlow(t *testing.T) {
	ts, signer, mailer := newTestServer(t)
	uid, access := registerAndVerify(t, ts, mailer, "ali@example.com")

	resp, body := do(t, ts, "GET", "/me", nil, access)
	if resp.StatusCode != http.StatusOK || body["email"] != "ali@example.com" {
		t.Fatalf("me: %d %v", resp.StatusCode, body)
	}
	if body["id"] != uid {
		t.Fatalf("me id mismatch: %v", body)
	}
	if _, err := signer.Parse(access); err != nil {
		t.Fatalf("signer roundtrip: %v", err)
	}
}

func TestUnverifiedLoginRejected(t *testing.T) {
	ts, _, _ := newTestServer(t)
	do(t, ts, "POST", "/auth/register", map[string]string{"email": "a@b.c", "password": "secret12"}, "")
	resp, _ := do(t, ts, "POST", "/auth/login", map[string]string{"email": "a@b.c", "password": "secret12"}, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for unverified login, got %d", resp.StatusCode)
	}
}

func TestRegisterValidation(t *testing.T) {
	ts, _, _ := newTestServer(t)
	do(t, ts, "POST", "/auth/register", map[string]string{"email": "a@b.c", "password": "secret12"}, "")
	cases := []struct {
		name string
		body map[string]string
	}{
		{"weak password", map[string]string{"email": "x@b.c", "password": "short"}},
		{"duplicate email", map[string]string{"email": "a@b.c", "password": "secret12"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := do(t, ts, "POST", "/auth/register", tc.body, "")
			if resp.StatusCode != http.StatusUnprocessableEntity && resp.StatusCode != http.StatusConflict {
				t.Fatalf("got %d", resp.StatusCode)
			}
		})
	}
}

// TestSetPlatformRole verifies the internal admin endpoint for assigning
// a platform role. There is no auth on the test server for /internal/*,
// matching production behavior where the gateway enforces the token.
func TestSetPlatformRole(t *testing.T) {
	ts, _, mailer := newTestServer(t)
	uid, _ := registerAndVerify(t, ts, mailer, "admin@example.com")

	// Initial: platform_role is empty.
	resp, body := do(t, ts, "GET", "/internal/users/"+uid, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get user: %d %v", resp.StatusCode, body)
	}
	if pr, _ := body["platform_role"].(string); pr != "" {
		t.Errorf("expected empty platform_role, got %q", pr)
	}

	// Assign system_admin.
	resp, body = do(t, ts, "PATCH", "/internal/users/"+uid+"/platform-role", map[string]string{
		"platform_role": "system_admin",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set role: %d %v", resp.StatusCode, body)
	}
	if pr := body["platform_role"].(string); pr != "system_admin" {
		t.Errorf("expected system_admin, got %q", pr)
	}

	// Invalid role rejected.
	resp, _ = do(t, ts, "PATCH", "/internal/users/"+uid+"/platform-role", map[string]string{
		"platform_role": "owner",
	}, "")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid role, got %d", resp.StatusCode)
	}

	// Revoke by sending empty string.
	resp, body = do(t, ts, "PATCH", "/internal/users/"+uid+"/platform-role", map[string]string{
		"platform_role": "",
	}, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke role: %d %v", resp.StatusCode, body)
	}
	if pr, _ := body["platform_role"].(string); pr != "" {
		t.Errorf("expected empty after revoke, got %q", pr)
	}
}

func TestMembershipGrantRequiresManager(t *testing.T) {
	ts, _, mailer := newTestServer(t)
	mgrID, mgrTok := registerAndVerify(t, ts, mailer, "mgr@example.com")
	resID, resTok := registerAndVerify(t, ts, mailer, "res@example.com")

	buildingID := "bld-1"
	resp, body := do(t, ts, "POST", fmt.Sprintf("/internal/buildings/%s/bootstrap-manager", buildingID), map[string]string{"user_id": mgrID}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("bootstrap: %d %v", resp.StatusCode, body)
	}

	resp, body = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/memberships", buildingID), map[string]string{"user_id": resID, "role": "owner"}, resTok)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("self-grant should 403, got %d %v", resp.StatusCode, body)
	}

	resp, body = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/memberships", buildingID), map[string]string{"user_id": resID, "role": "owner"}, mgrTok)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("grant: %d %v", resp.StatusCode, body)
	}

	resp, body = do(t, ts, "GET", "/me/buildings", nil, resTok)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me/buildings: %d", resp.StatusCode)
	}
	if data := body["data"].([]any); len(data) != 1 {
		t.Fatalf("expected 1 membership, got %v", data)
	}
}
