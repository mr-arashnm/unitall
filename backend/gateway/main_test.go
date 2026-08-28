package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPickService locks the gateway routing table: every route family
// resolves to its owning service, with the right public/authenticated
// flag. Precedence bugs here silently 404 whole services.
func TestPickService(t *testing.T) {
	cases := []struct {
		path     string
		wantSvc  string
		wantOpen bool
	}{
		// service-to-service (authenticated via shared token, not user JWT)
		{"/api/v1/internal/users/u1", "identity", false},
		{"/api/v1/internal/buildings/b1/bootstrap-manager", "identity", false},
		// auth (public)
		{"/api/v1/auth/login", "identity", true},
		{"/api/v1/auth/register", "identity", true},
		{"/api/v1/auth/refresh", "identity", true},
		{"/api/v1/auth/password-reset", "identity", true},
		{"/api/v1/auth/password-reset/confirm", "identity", true},
		// identity: me + memberships
		{"/api/v1/me", "identity", false},
		{"/api/v1/me/buildings", "identity", false},
		{"/api/v1/buildings/b1/memberships", "identity", false},
		{"/api/v1/buildings/b1/memberships/m9", "identity", false},
		// /me/notifications must beat /me
		{"/api/v1/me/notifications", "notif", false},
		{"/api/v1/me/notifications/i1/read", "notif", false},
		// notifications core + comms
		{"/api/v1/templates", "notif", false},
		{"/api/v1/notifications:send", "notif", false},
		{"/api/v1/notifications/c1", "notif", false},
		{"/api/v1/buildings/b1/announcements", "notif", false},
		{"/api/v1/announcements/a1/publish", "notif", false},
		{"/api/v1/buildings/b1/meetings", "notif", false},
		{"/api/v1/meetings/m1/rsvp", "notif", false},
		{"/api/v1/buildings/b1/tickets", "notif", false},
		{"/api/v1/tickets/t1/responses", "notif", false},
		{"/api/v1/tickets/t1/resolve", "notif", false},
		// billing
		{"/api/v1/buildings/b1/charge-templates", "billing", false},
		{"/api/v1/charge-templates/t1", "billing", false},
		{"/api/v1/buildings/b1/charges:generate", "billing", false},
		{"/api/v1/buildings/b1/charges", "billing", false},
		{"/api/v1/charges/c1/payments", "billing", false},
		{"/api/v1/transactions/x1/confirm", "billing", false},
		{"/api/v1/buildings/b1/invoices", "billing", false},
		{"/api/v1/buildings/b1/reports/financial", "billing", false},
		// facilities (flat and building-scoped)
		{"/api/v1/facilities", "facilities", false},
		{"/api/v1/facilities/f1", "facilities", false},
		{"/api/v1/facilities/f1/availability", "facilities", false},
		{"/api/v1/buildings/b1/facilities", "facilities", false},
		{"/api/v1/facilities/f1/bookings", "facilities", false},
		{"/api/v1/bookings?mine=true", "facilities", false},
		{"/api/v1/bookings/b1/approve", "facilities", false},
		{"/api/v1/facilities/f1/maintenance-windows", "facilities", false},
		{"/api/v1/maintenance-windows/m1/start", "facilities", false},
		// operations
		{"/api/v1/buildings/b1/teams", "operations", false},
		{"/api/v1/teams/t1", "operations", false},
		{"/api/v1/teams/t1/members", "operations", false},
		{"/api/v1/teams/t1/tasks", "operations", false},
		{"/api/v1/buildings/b1/tasks", "operations", false},
		{"/api/v1/tasks/k1/complete", "operations", false},
		{"/api/v1/tasks/k1/comments", "operations", false},
		{"/api/v1/buildings/b1/service-requests", "operations", false},
		{"/api/v1/service-requests/r1/assign", "operations", false},
		// property (top-level and nested detail routes)
		{"/api/v1/buildings", "property", false},
		{"/api/v1/buildings/b1", "property", false},
		{"/api/v1/buildings/b1/units", "property", false},
		{"/api/v1/units/u1", "property", false},
		{"/api/v1/units/u1/parties", "property", false},
		{"/api/v1/units/u1/ownership-changes", "property", false},
		{"/api/v1/buildings/b1/assets", "property", false},
		{"/api/v1/contracts", "property", false},
		{"/api/v1/contracts/c1/activate", "property", false},
		{"/api/v1/parkings", "property", false},
		{"/api/v1/warehouses", "property", false},
		// unknown
		{"/api/v1/nope", "", false},
		{"/api/v1/", "", false},
	}
	for _, tc := range cases {
		got, open := pickService(tc.path)
		if got != tc.wantSvc || open != tc.wantOpen {
			t.Errorf("pickService(%q) = (%q,%v), want (%q,%v)", tc.path, got, open, tc.wantSvc, tc.wantOpen)
		}
	}
}

func TestCORSMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := cors(next)

	// preflight short-circuits with 204 + allow headers
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/buildings", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want 204", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("preflight missing Access-Control-Allow-Origin")
	}

	// normal cross-origin request passes through with CORS headers
	req = httptest.NewRequest(http.MethodGet, "/api/v1/buildings", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("passthrough = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("missing Allow-Origin on response")
	}

	// same-origin/no-Origin requests get no CORS headers
	req = httptest.NewRequest(http.MethodGet, "/api/v1/buildings", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS headers must not be set without an Origin header")
	}
}
