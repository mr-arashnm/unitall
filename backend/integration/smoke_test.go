// Package integration runs end-to-end smoke tests against a live Unital
// stack (gateway + all services). The test is opt-in: it only runs when
// the UNITAL_INTEGRATION_BASE_URL env var is set, so it never breaks
// local `go test ./...` runs.
//
// Run from the repo root:
//
//	UNITAL_INTEGRATION_BASE_URL=http://127.0.0.1:18080 \
//	  go test ./integration/... -run TestSmoke -v
//
// The test exercises every endpoint the frontend actually calls, in the
// order it would call them during a building lifecycle. Each step
// fails fast on the first 4xx/5xx so contract mismatches surface
// immediately.
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const testUser = "sara@example.com"
const testPassword = "secret12"

type api struct {
	base string
	hc   *http.Client
	tok  string
}

func newAPI(t *testing.T) *api {
	t.Helper()
	base := os.Getenv("UNITAL_INTEGRATION_BASE_URL")
	if base == "" {
		t.Skip("UNITAL_INTEGRATION_BASE_URL not set; skipping integration smoke test")
	}
	base = strings.TrimRight(base, "/")
	a := &api{base: base, hc: &http.Client{Timeout: 10 * time.Second}}

	// Register the test user if they don't exist yet (DB may be fresh).
	// Registration no longer accepts a role field. Use do() so 409 (already
	// exists) doesn't fail the test.
	regResp, _ := a.do(t, http.MethodPost, "/auth/register", map[string]string{
		"email": testUser, "password": testPassword,
	}, false)
	if regResp.StatusCode >= 400 && regResp.StatusCode != http.StatusConflict {
		t.Fatalf("register: %d %v", regResp.StatusCode, regResp.StatusCode)
	}

	loginResp := a.post(t, "/auth/login", map[string]string{
		"email": testUser, "password": testPassword,
	}, false)
	tok, ok := loginResp["access_token"].(string)
	if !ok || tok == "" {
		t.Fatalf("login: no access_token in %v", loginResp)
	}
	a.tok = tok
	return a
}

func (a *api) do(t *testing.T, method, path string, body any, withAuth bool) (*http.Response, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, a.base+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if withAuth {
		req.Header.Set("Authorization", "Bearer "+a.tok)
	}
	resp, err := a.hc.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func (a *api) get(t *testing.T, path string) (*http.Response, map[string]any) {
	return a.do(t, http.MethodGet, path, nil, true)
}

func (a *api) post(t *testing.T, path string, body any, withAuth bool) map[string]any {
	t.Helper()
	resp, out := a.do(t, http.MethodPost, path, body, withAuth)
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s → %d: %v", path, resp.StatusCode, out)
	}
	return out
}

func (a *api) patch(t *testing.T, path string, body any) map[string]any {
	t.Helper()
	resp, out := a.do(t, http.MethodPatch, path, body, true)
	if resp.StatusCode >= 400 {
		t.Fatalf("PATCH %s → %d: %v", path, resp.StatusCode, out)
	}
	return out
}

func (a *api) delete(t *testing.T, path string) {
	t.Helper()
	resp, _ := a.do(t, http.MethodDelete, path, nil, true)
	if resp.StatusCode >= 400 {
		t.Fatalf("DELETE %s → %d", path, resp.StatusCode)
	}
}

func mustString(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("expected %q to be a string, got %T (%v)", key, m[key], m)
	}
	return v
}

func dataSlice(t *testing.T, m map[string]any) []any {
	t.Helper()
	d, ok := m["data"].([]any)
	if !ok {
		t.Fatalf("expected data array, got %T (%v)", m["data"], m)
	}
	return d
}

// TestSmoke walks every frontend endpoint end-to-end. If any of these
// start returning 4xx/5xx, or a response shape the frontend can't
// consume, the smoke test will fail and the contract drift is caught
// before it ships.
func TestSmoke(t *testing.T) {
	a := newAPI(t)

	t.Run("identity", func(t *testing.T) {
		// /me
		resp, body := a.get(t, "/me")
		if resp.StatusCode != 200 {
			t.Fatalf("/me: %d", resp.StatusCode)
		}
		if _, ok := body["email"]; !ok {
			t.Fatalf("/me missing email: %v", body)
		}
		if _, ok := body["id"]; !ok {
			t.Fatalf("/me missing id: %v", body)
		}

		// PATCH /me (updateProfile) — restore after
		originalName := body["full_name"]
		updated := a.patch(t, "/me", map[string]any{"phone": "+1 555 0001"})
		if updated["phone"] != "+1 555 0001" {
			t.Fatalf("PATCH /me: phone not updated, got %v", updated["phone"])
		}
		if originalName != nil {
			_ = a.patch(t, "/me", map[string]any{"full_name": originalName})
		}
	})

	t.Run("buildings", func(t *testing.T) {
		// /buildings — should return at least the ones sara created.
		// The frontend uses this for both fetchBuildings and
		// fetchMyBuildings, so the response must contain full
		// Building objects (id, name, code, type, address).
		resp, body := a.get(t, "/buildings")
		if resp.StatusCode != 200 {
			t.Fatalf("/buildings: %d", resp.StatusCode)
		}
		_ = dataSlice(t, body) // shape check: must be {data: []}
		for _, item := range body["data"].([]any) {
			row := item.(map[string]any)
			for _, key := range []string{"id", "name", "code", "type", "address"} {
				if _, ok := row[key]; !ok {
					t.Fatalf("/buildings data row missing %q: %v", key, row)
				}
			}
		}

		// POST /buildings (createBuilding)
		code := fmt.Sprintf("SMK-%d", time.Now().UnixNano()%100000)
		newB := a.post(t, "/buildings", map[string]any{
			"name": "Smoke Tower", "code": code, "type": "residential",
			"address": "1 Test Way", "floors": 8, "features": []string{"billing"},
		}, true)
		bid := mustString(t, newB, "id")
		t.Cleanup(func() { /* buildings are not deletable through the API */ })

		// /me/buildings — should now list the newly created building
		// (because the property service bootstraps the creator as a
		// manager in identity's membership table on create).
		_, mships := a.get(t, "/me/buildings")
		_ = dataSlice(t, mships) // shape check
		found := false
		for _, item := range mships["data"].([]any) {
			if row := item.(map[string]any); row["building_id"] == bid {
				found = true
				if row["role"] != "manager" {
					t.Errorf("expected manager role for bootstrap, got %v", row["role"])
				}
			}
		}
		if !found {
			t.Errorf("expected %s in /me/buildings memberships, got %v", bid, mships)
		}

		// GET /buildings/{id}
		_, got := a.get(t, "/buildings/"+bid)
		if mustString(t, got, "id") != bid {
			t.Fatalf("/buildings/{id} id mismatch")
		}

		// POST /buildings/{id}/units
		newU := a.post(t, "/buildings/"+bid+"/units", map[string]any{
			"floor": 1, "number": "101", "area_m2": 80, "rooms": 2,
		}, true)
		uid := mustString(t, newU, "id")

		// GET /buildings/{id}/units
		_, ulist := a.get(t, "/buildings/"+bid+"/units")
		if len(dataSlice(t, ulist)) < 1 {
			t.Errorf("expected ≥1 unit, got %d", len(ulist["data"].([]any)))
		}

		// GET /units/{id}
		_, u := a.get(t, "/units/"+uid)
		if mustString(t, u, "id") != uid {
			t.Errorf("unit id mismatch")
		}

		// POST /buildings/{id}/assets (parking)
		_ = a.post(t, "/buildings/"+bid+"/assets", map[string]any{
			"kind": "parking", "code": "P-SMK-1", "floor": -1, "area_m2": 12,
		}, true)
		// GET /buildings/{id}/assets?kind=parking
		_, assets := a.get(t, "/buildings/"+bid+"/assets?kind=parking")
		_ = dataSlice(t, assets)

		// Memberships
		_, mlist := a.get(t, "/buildings/"+bid+"/memberships")
		_ = dataSlice(t, mlist)
	})

	t.Run("billing", func(t *testing.T) {
		// Use a building sara manages (bootstrap test building)
		_, body := a.get(t, "/buildings")
		buildings := body["data"].([]any)
		if len(buildings) == 0 {
			t.Skip("no buildings to bill against")
		}
		bid := buildings[0].(map[string]any)["id"].(string)

		// POST /buildings/{id}/charge-templates
		t1 := a.post(t, "/buildings/"+bid+"/charge-templates", map[string]any{
			"name": "Smoke Maintenance", "type": "maintenance", "amount": 100000, "description": "test",
		}, true)
		tid := mustString(t, t1, "id")

		// GET /buildings/{id}/charge-templates
		_, tpl := a.get(t, "/buildings/"+bid+"/charge-templates")
		tplData := dataSlice(t, tpl)
		found := false
		for _, item := range tplData {
			if row := item.(map[string]any); row["id"] == tid {
				found = true
			}
		}
		if !found {
			t.Errorf("created template %s not in list", tid)
		}

		// POST /buildings/{id}/charges:generate
		gen := a.post(t, "/buildings/"+bid+"/charges:generate", map[string]any{
			"period": "1404-07", "due_in_days": 14,
		}, true)
		if _, ok := gen["created"]; !ok {
			t.Errorf("generate response missing 'created': %v", gen)
		}

		// GET /buildings/{id}/charges
		_, charges := a.get(t, "/buildings/"+bid+"/charges")
		_ = dataSlice(t, charges)

		// POST /charges/{id}/payments (payCharge) — only if there's a charge
		if chargesData, ok := charges["data"].([]any); ok && len(chargesData) > 0 {
			first := chargesData[0].(map[string]any)
			cid := first["id"].(string)
			pay := a.post(t, "/charges/"+cid+"/payments", map[string]any{
				"amount": first["amount"], "method": "online",
			}, true)
			if _, ok := pay["id"]; !ok {
				t.Errorf("payment response missing id: %v", pay)
			}
		}

		// GET /buildings/{id}/invoices
		_, inv := a.get(t, "/buildings/"+bid+"/invoices")
		_ = dataSlice(t, inv)

		// GET /buildings/{id}/reports/financial
		_, report := a.get(t, "/buildings/"+bid+"/reports/financial?period=1404-07")
		for _, key := range []string{"period", "total_billed", "total_collected", "outstanding"} {
			if _, ok := report[key]; !ok {
				t.Errorf("financial report missing %q: %v", key, report)
			}
		}
	})

	t.Run("facilities", func(t *testing.T) {
		_, body := a.get(t, "/buildings")
		buildings := body["data"].([]any)
		if len(buildings) == 0 {
			t.Skip("no buildings for facilities test")
		}
		bid := buildings[0].(map[string]any)["id"].(string)

		// POST /facilities (top-level)
		fac := a.post(t, "/facilities", map[string]any{
			"building_id": bid, "name": "Smoke Gym", "type": "gym",
			"capacity": 10, "opening_time": "08:00", "closing_time": "22:00",
			"hourly_rate": 0, "is_active": true,
		}, true)
		fid := mustString(t, fac, "id")

		// GET /buildings/{id}/facilities
		_, list := a.get(t, "/buildings/"+bid+"/facilities")
		_ = dataSlice(t, list)

		// GET /facilities/{id}/availability
		_, avail := a.get(t, "/facilities/"+fid+"/availability?date=2026-09-01")
		if _, ok := avail["free_hours"]; !ok {
			t.Errorf("availability missing free_hours: %v", avail)
		}

		// POST /facilities/{id}/bookings
		booking := a.post(t, "/facilities/"+fid+"/bookings", map[string]any{
			"start": "2026-09-01T10:00:00Z", "end": "2026-09-01T11:00:00Z",
			"purpose": "smoke", "participants": 2,
		}, true)
		bkID := mustString(t, booking, "id")

		// GET /bookings?mine=true (fetchMyBookings)
		_, mine := a.get(t, "/bookings?mine=true")
		_ = dataSlice(t, mine)

		// POST /bookings/{id}/cancel
		resp, _ := a.do(t, http.MethodPost, "/bookings/"+bkID+"/cancel", nil, true)
		if resp.StatusCode >= 400 {
			t.Errorf("cancel booking: %d", resp.StatusCode)
		}
	})

	t.Run("operations", func(t *testing.T) {
		_, body := a.get(t, "/buildings")
		buildings := body["data"].([]any)
		if len(buildings) == 0 {
			t.Skip("no buildings for operations test")
		}
		bid := buildings[0].(map[string]any)["id"].(string)

		// POST /buildings/{id}/teams
		team := a.post(t, "/buildings/"+bid+"/teams", map[string]any{
			"name": "Smoke Team", "type": "maintenance", "description": "test",
		}, true)
		tid := mustString(t, team, "id")

		// GET /buildings/{id}/teams
		_, list := a.get(t, "/buildings/"+bid+"/teams")
		_ = dataSlice(t, list)

		// POST /teams/{id}/tasks
		task := a.post(t, "/teams/"+tid+"/tasks", map[string]any{
			"title": "Smoke Task", "priority": "medium", "building_id": bid,
		}, true)
		taskID := mustString(t, task, "id")

		// GET /buildings/{id}/tasks
		_, tasks := a.get(t, "/buildings/"+bid+"/tasks")
		_ = dataSlice(t, tasks)

		// POST /tasks/{id}/start, then /complete (the usecase enforces
		// pending→in_progress→completed state transitions; the test
		// must walk them in order).
		if resp, _ := a.do(t, http.MethodPost, "/tasks/"+taskID+"/start", nil, true); resp.StatusCode >= 400 {
			t.Errorf("start task: %d", resp.StatusCode)
		}
		if resp, _ := a.do(t, http.MethodPost, "/tasks/"+taskID+"/complete", map[string]any{"actual_hours": 1}, true); resp.StatusCode >= 400 {
			t.Errorf("complete task: %d", resp.StatusCode)
		}

		// POST /buildings/{id}/service-requests
		// (needs a unit id; pull from property)
		_, unitsResp := a.get(t, "/buildings/"+bid+"/units")
		units := unitsResp["data"].([]any)
		if len(units) > 0 {
			uid := units[0].(map[string]any)["id"].(string)
			req := a.post(t, "/buildings/"+bid+"/service-requests", map[string]any{
				"unit_id": uid, "title": "Smoke Request", "type": "maintenance",
				"priority": "medium", "description": "test",
			}, true)
			if _, ok := req["id"]; !ok {
				t.Errorf("service-request response missing id: %v", req)
			}
		}
		// GET /buildings/{id}/service-requests
		_, srs := a.get(t, "/buildings/"+bid+"/service-requests")
		_ = dataSlice(t, srs)
	})

	t.Run("comms", func(t *testing.T) {
		_, body := a.get(t, "/buildings")
		buildings := body["data"].([]any)
		if len(buildings) == 0 {
			t.Skip("no buildings for comms test")
		}
		bid := buildings[0].(map[string]any)["id"].(string)

		// POST /buildings/{id}/announcements
		ann := a.post(t, "/buildings/"+bid+"/announcements", map[string]any{
			"title": "Smoke", "body": "smoke body", "priority": "normal",
		}, true)
		_ = ann

		// GET /buildings/{id}/announcements
		_, list := a.get(t, "/buildings/"+bid+"/announcements")
		_ = dataSlice(t, list)

		// POST /buildings/{id}/meetings
		meet := a.post(t, "/buildings/"+bid+"/meetings", map[string]any{
			"title": "Smoke Meeting", "type": "general",
			"scheduled_at": "2026-09-15T18:00:00Z",
			"duration_min": 60, "description": "smoke",
		}, true)
		_ = meet
		_, mlist := a.get(t, "/buildings/"+bid+"/meetings")
		_ = dataSlice(t, mlist)

		// POST /buildings/{id}/tickets (needs unit)
		_, unitsResp := a.get(t, "/buildings/"+bid+"/units")
		units := unitsResp["data"].([]any)
		if len(units) > 0 {
			uid := units[0].(map[string]any)["id"].(string)
			tk := a.post(t, "/buildings/"+bid+"/tickets", map[string]any{
				"unit_id": uid, "title": "Smoke Ticket", "type": "general",
				"priority": "low", "description": "test",
			}, true)
			_ = tk
		}
		_, tlist := a.get(t, "/buildings/"+bid+"/tickets")
		_ = dataSlice(t, tlist)

		// GET /me/notifications (fetchInbox)
		_, inbox := a.get(t, "/me/notifications")
		_ = dataSlice(t, inbox)
	})

	t.Run("notification_templates", func(t *testing.T) {
		// POST /templates (upsert)
		tpl := a.post(t, "/templates", map[string]any{
			"name": "smoke_tmpl", "severity": "normal",
			"channels": []string{"inapp"},
			"variants": map[string]any{"default": map[string]any{"title": "T", "body": "B"}},
		}, true)
		if _, ok := tpl["name"]; !ok {
			t.Errorf("template upsert: missing name in response: %v", tpl)
		}
		// GET /templates
		_, list := a.get(t, "/templates")
		_ = dataSlice(t, list)
	})

	t.Run("gateway_routing", func(t *testing.T) {
		// /internal/* must route to identity (smoke check via 401 with
		// no internal token — proves the route exists and goes to
		// identity, not property).
		req, _ := http.NewRequest(http.MethodPost, a.base+"/api/v1/internal/buildings/x/bootstrap-manager", bytes.NewReader([]byte("{}")))
		req.Header.Set("Content-Type", "application/json")
		resp, err := a.hc.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		// 401 (no token) or 404 (no such building) both prove the
		// route reached identity. What we must NOT see is 502
		// (property can't reach it) or any non-401/404/422 status.
		if resp.StatusCode != 401 && resp.StatusCode != 404 && resp.StatusCode != 422 {
			t.Errorf("internal bootstrap returned %d, expected 401/404/422", resp.StatusCode)
		}
	})
}
