package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/facilities/internal/adapter/store/memory"
	"unital/backend/services/facilities/internal/usecase"
)

const (
	managerID = "user-manager"
	resident  = "user-resident"
	building  = "bld-1"
)

func newTestServer(t *testing.T) (*httptest.Server, *usecase.Facilities) {
	t.Helper()
	b := memory.New()
	b.Membership.Seed(managerID, building, "manager")
	b.Membership.Seed(resident, building, "resident")
	fac := usecase.New(b.Facilities, b.Bookings, b.Maintenance, b.Membership, events.LogPublisher{})
	ts := httptest.NewServer(NewServer(New(fac, jwtx.NewSigner("t", time.Minute))))
	t.Cleanup(ts.Close)
	return ts, fac
}

func do(t *testing.T, ts *httptest.Server, method, path string, body any, userID string) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-Id", userID)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func tomorrow(hour int) string {
	return time.Now().UTC().AddDate(0, 0, 1).Truncate(24 * time.Hour).Add(time.Duration(hour) * time.Hour).Format(time.RFC3339)
}

func setupFacility(t *testing.T, ts *httptest.Server) string {
	resp, body := do(t, ts, "POST", "/facilities", map[string]any{
		"building_id": building, "name": "سالن اجتماعات", "type": "party_hall",
		"capacity": 40, "opening_time": "08:00", "closing_time": "22:00",
		"hourly_rate": 500000, "min_advance_hours": 2, "max_advance_hours": 720,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create facility: %d %v", resp.StatusCode, body)
	}
	return body["id"].(string)
}

func TestBookingLifecycleAndRules(t *testing.T) {
	ts, _ := newTestServer(t)
	facID := setupFacility(t, ts)

	// resident books 18:00–20:00 tomorrow
	resp, body := do(t, ts, "POST", fmt.Sprintf("/facilities/%s/bookings", facID), map[string]any{
		"start": tomorrow(18), "end": tomorrow(20), "purpose": "تولد", "participants": 30,
	}, resident)
	if resp.StatusCode != http.StatusCreated || body["status"] != "pending" {
		t.Fatalf("book: %d %v", resp.StatusCode, body)
	}
	if body["total_cost"].(float64) != 1_000_000 {
		t.Fatalf("cost should be 2h × 500k, got %v", body["total_cost"])
	}
	bookingID := body["id"].(string)

	// overlapping booking by same or other user → 409
	resp, body = do(t, ts, "POST", fmt.Sprintf("/facilities/%s/bookings", facID), map[string]any{
		"start": tomorrow(19), "end": tomorrow(21), "purpose": "x", "participants": 5,
	}, resident)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("overlap should 409, got %d %v", resp.StatusCode, body)
	}

	// outside closing hours → 422
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/facilities/%s/bookings", facID), map[string]any{
		"start": tomorrow(21), "end": tomorrow(23), "purpose": "x", "participants": 5,
	}, resident)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("after-hours should 422, got %d", resp.StatusCode)
	}

	// over capacity → 422
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/facilities/%s/bookings", facID), map[string]any{
		"start": tomorrow(10), "end": tomorrow(11), "purpose": "x", "participants": 100,
	}, resident)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("over capacity should 422, got %d", resp.StatusCode)
	}

	// resident cannot approve; manager can
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/bookings/%s/approve", bookingID), nil, resident)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("resident approve should 403, got %d", resp.StatusCode)
	}
	resp, body = do(t, ts, "POST", fmt.Sprintf("/bookings/%s/approve", bookingID), nil, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "confirmed" {
		t.Fatalf("approve: %d %v", resp.StatusCode, body)
	}

	// availability reflects the confirmed block
	day := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	resp, body = do(t, ts, "GET", fmt.Sprintf("/facilities/%s/availability?date=%s", facID, day), nil, resident)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("availability: %d", resp.StatusCode)
	}
	free := map[int]bool{}
	for _, h := range body["free_hours"].([]any) {
		free[int(h.(float64))] = true
	}
	if free[18] || free[19] || !free[17] || !free[20] {
		t.Fatalf("free hours wrong: %v", free)
	}
}

func TestMaintenanceCancelsBookings(t *testing.T) {
	ts, _ := newTestServer(t)
	facID := setupFacility(t, ts)

	_, body := do(t, ts, "POST", fmt.Sprintf("/facilities/%s/bookings", facID), map[string]any{
		"start": tomorrow(18), "end": tomorrow(20), "purpose": "جلسه", "participants": 10,
	}, resident)
	bookingID := body["id"].(string)
	do(t, ts, "POST", fmt.Sprintf("/bookings/%s/approve", bookingID), nil, managerID)

	// maintenance over the same window cancels the booking
	resp, mbody := do(t, ts, "POST", fmt.Sprintf("/facilities/%s/maintenance-windows", facID), map[string]any{
		"title": "سرویس دوره‌ای", "type": "cleaning", "priority": "high",
		"scheduled_start": tomorrow(17), "scheduled_end": tomorrow(21), "affect_bookings": true,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("maintenance: %d %v", resp.StatusCode, mbody)
	}
	maintID := mbody["id"].(string)

	_, b2 := do(t, ts, "GET", fmt.Sprintf("/bookings/%s", bookingID), nil, resident)
	if b2["status"] != "cancelled" {
		t.Fatalf("booking should be cancelled by maintenance, got %v", b2["status"])
	}

	// start → complete lifecycle
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/maintenance-windows/%s/start", maintID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("maint start failed: %d", resp.StatusCode)
	}
	resp, body3 := do(t, ts, "POST", fmt.Sprintf("/maintenance-windows/%s/complete", maintID), nil, managerID)
	if resp.StatusCode != http.StatusOK || body3["status"] != "done" {
		t.Fatalf("maint complete: %v", body3)
	}
}
