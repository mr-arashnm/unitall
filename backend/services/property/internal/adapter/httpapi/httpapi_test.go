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
	"unital/backend/services/property/internal/adapter/store/memory"
	"unital/backend/services/property/internal/usecase"
)

const (
	managerID = "user-manager"
	boardID   = "user-board"
	ownerA    = "user-owner-a" // seller
	ownerB    = "user-owner-b" // buyer
	outsider  = "user-outsider"
)

func newTestServer(t *testing.T) (*httptest.Server, memory.Bundle) {
	t.Helper()
	bundle := memory.New()
	// seed roles: manager + board manage the building; ownerA owns a unit.
	prop := usecase.New(bundle.Buildings, bundle.Units, bundle.Assets, bundle.Parties, bundle.Contracts, bundle.Membership, events.LogPublisher{})
	signer := jwtx.NewSigner("test-secret", 15*time.Minute)
	ts := httptest.NewServer(NewServer(New(prop, signer)))
	t.Cleanup(ts.Close)
	return ts, bundle
}

func do(t *testing.T, ts *httptest.Server, method, path string, body any, userID string) (*http.Response, map[string]any) {
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
	if userID != "" {
		req.Header.Set("X-User-Id", userID) // gateway normally injects this
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// setup creates a building + one unit, seeding memberships, and returns ids.
func setup(t *testing.T, ts *httptest.Server, b memory.Bundle) (buildingID, unitID string) {
	t.Helper()
	resp, body := do(t, ts, "POST", "/buildings", map[string]any{
		"name": "Towers", "code": "TWR-1", "type": "residential",
		"address": "Street 1", "floors": 10, "features": []string{"billing"},
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create building: %d %v", resp.StatusCode, body)
	}
	buildingID = body["id"].(string)
	b.Membership.Seed(managerID, buildingID, "manager")
	b.Membership.Seed(boardID, buildingID, "board_member")
	b.Membership.Seed(ownerA, buildingID, "owner")

	resp, body = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/units", buildingID), map[string]any{
		"floor": 3, "number": "301", "area_m2": 120.5, "rooms": 3,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create unit: %d %v", resp.StatusCode, body)
	}
	unitID = body["id"].(string)
	return buildingID, unitID
}

func TestBuildingUnitCreationAndRBAC(t *testing.T) {
	ts, b := newTestServer(t)
	buildingID, _ := setup(t, ts, b)

	// outsider cannot create units
	resp, body := do(t, ts, "POST", fmt.Sprintf("/buildings/%s/units", buildingID), map[string]any{
		"floor": 1, "number": "101", "area_m2": 80, "rooms": 2,
	}, outsider)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider unit create should 403, got %d %v", resp.StatusCode, body)
	}

	// board member can list units
	resp, body = do(t, ts, "GET", fmt.Sprintf("/buildings/%s/units", buildingID), nil, boardID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list units: %d %v", resp.StatusCode, body)
	}
	if data := body["data"].([]any); len(data) != 1 {
		t.Fatalf("expected 1 unit, got %v", data)
	}
}

func TestAssetAssignmentRules(t *testing.T) {
	ts, b := newTestServer(t)
	buildingID, unitID := setup(t, ts, b)

	resp, body := do(t, ts, "POST", fmt.Sprintf("/buildings/%s/assets", buildingID), map[string]any{
		"kind": "parking", "code": "P-01", "floor": -1,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create asset: %d %v", resp.StatusCode, body)
	}

	// duplicate code rejected
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/assets", buildingID), map[string]any{
		"kind": "parking", "code": "P-01", "floor": -1,
	}, managerID)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate asset should 409, got %d", resp.StatusCode)
	}

	// assign → ok
	resp, body = do(t, ts, "POST", fmt.Sprintf("/units/%s/assets", unitID), map[string]any{
		"kind": "parking", "code": "P-01",
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["available"] != false {
		t.Fatalf("assign: %d %v", resp.StatusCode, body)
	}

	// double assign → 409
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/units/%s/assets", unitID), map[string]any{
		"kind": "parking", "code": "P-01",
	}, managerID)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("double assign should 409, got %d", resp.StatusCode)
	}

	// release → available again
	resp, _ = do(t, ts, "DELETE", fmt.Sprintf("/units/%s/assets/P-01?kind=parking", unitID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("release failed: %d", resp.StatusCode)
	}
	resp, body = do(t, ts, "GET", fmt.Sprintf("/buildings/%s/assets?kind=parking&status=available", buildingID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list assets: %d", resp.StatusCode)
	}
	if data := body["data"].([]any); len(data) != 1 || data[0].(map[string]any)["available"] != true {
		t.Fatalf("expected released asset available, got %v", data)
	}
}

func TestContractActivationChangesOwnership(t *testing.T) {
	ts, b := newTestServer(t)
	_, unitID := setup(t, ts, b)

	// initial owner = ownerA
	resp, _ := do(t, ts, "POST", fmt.Sprintf("/units/%s/ownership-changes", unitID), map[string]any{
		"new_user_id": ownerA,
	}, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set initial owner: %d", resp.StatusCode)
	}

	// draft purchase contract: seller ownerA → buyer ownerB
	resp, body := do(t, ts, "POST", "/contracts", map[string]any{
		"type": "purchase", "unit_id": unitID,
		"first_party_id": ownerA, "second_party_id": ownerB,
		"title": "Sale of unit 301", "amount": 5_000_000_000,
		"deposit_amount": 200_000_000, "start_date": "2026-09-01", "duration_months": 1,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create contract: %d %v", resp.StatusCode, body)
	}
	contractID := body["id"].(string)
	if num := body["number"].(string); len(num) == 0 {
		t.Fatalf("contract number missing: %v", body)
	}

	// activation before signatures → 409
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/contracts/%s/activate", contractID), nil, managerID)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("unsigned activate should 409, got %d", resp.StatusCode)
	}

	// parties sign (signer identity checked against contract parties)
	do(t, ts, "POST", fmt.Sprintf("/contracts/%s/sign", contractID), nil, ownerA)
	do(t, ts, "POST", fmt.Sprintf("/contracts/%s/sign", contractID), nil, ownerB)

	// outsider cannot sign (not a party)
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/contracts/%s/sign", contractID), nil, outsider)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("outsider sign should be 422, got %d", resp.StatusCode)
	}

	resp, body = do(t, ts, "POST", fmt.Sprintf("/contracts/%s/activate", contractID), nil, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "active" {
		t.Fatalf("activate: %d %v", resp.StatusCode, body)
	}

	// ownership moved to buyer; history has two records
	resp, body = do(t, ts, "GET", fmt.Sprintf("/units/%s/parties", unitID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("parties: %d", resp.StatusCode)
	}
	cur := body["current"].(map[string]any)["owner"].(map[string]any)
	if cur["user_id"] != ownerB {
		t.Fatalf("owner should be buyer, got %v", cur)
	}

	resp, body = do(t, ts, "GET", fmt.Sprintf("/units/%s/transfer-history", unitID), nil, boardID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history: %d", resp.StatusCode)
	}
	rows := body["data"].([]any)
	if len(rows) != 2 {
		t.Fatalf("expected 2 transfer records, got %v", rows)
	}
	last := rows[1].(map[string]any)
	if last["new_user_id"] != ownerB || last["previous_user_id"] != ownerA {
		t.Fatalf("transfer record wrong: %v", last)
	}
}

func TestRentalContractChangesResidencyOnly(t *testing.T) {
	ts, b := newTestServer(t)
	_, unitID := setup(t, ts, b)

	do(t, ts, "POST", fmt.Sprintf("/units/%s/ownership-changes", unitID), map[string]any{"new_user_id": ownerA}, managerID)
	resp, body := do(t, ts, "POST", "/contracts", map[string]any{
		"type": "rental", "unit_id": unitID,
		"first_party_id": ownerA, "second_party_id": ownerB,
		"title": "Rent 301", "amount": 500_000_000, "duration_months": 12,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create rental: %d %v", resp.StatusCode, body)
	}
	contractID := body["id"].(string)
	do(t, ts, "POST", fmt.Sprintf("/contracts/%s/sign", contractID), nil, ownerA)
	do(t, ts, "POST", fmt.Sprintf("/contracts/%s/sign", contractID), nil, ownerB)
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/contracts/%s/activate", contractID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("activate rental: %d", resp.StatusCode)
	}

	resp, body = do(t, ts, "GET", fmt.Sprintf("/units/%s/parties", unitID), nil, managerID)
	current := body["current"].(map[string]any)
	if current["resident"].(map[string]any)["user_id"] != ownerB {
		t.Fatalf("resident should be tenant, got %v", current)
	}
	if current["owner"].(map[string]any)["user_id"] != ownerA {
		t.Fatalf("owner must stay landlord, got %v", current)
	}
}

func TestPatchBuildingAndUnit(t *testing.T) {
	ts, b := newTestServer(t)
	buildingID, unitID := setup(t, ts, b)

	// manager renames the building and bumps floors
	resp, body := do(t, ts, "PATCH", fmt.Sprintf("/buildings/%s", buildingID), map[string]any{
		"name": "Towers Renamed", "floors": 12,
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["name"] != "Towers Renamed" {
		t.Fatalf("patch building: %d %v", resp.StatusCode, body)
	}
	if body["floors"].(float64) != 12 {
		t.Fatalf("floors not patched: %v", body["floors"])
	}

	// board member (also manager-grade) can patch; plain owner cannot
	resp, _ = do(t, ts, "PATCH", fmt.Sprintf("/buildings/%s", buildingID), map[string]any{"address": "New Street"}, boardID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("board patch building: %d", resp.StatusCode)
	}
	resp, _ = do(t, ts, "PATCH", fmt.Sprintf("/buildings/%s", buildingID), map[string]any{"name": "Hacked"}, ownerA)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("owner patch building should 403, got %d", resp.StatusCode)
	}

	// unit status lifecycle: manager flips vacant → occupied
	resp, body = do(t, ts, "PATCH", fmt.Sprintf("/units/%s", unitID), map[string]any{
		"status": "occupied", "rooms": 4,
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "occupied" || body["rooms"].(float64) != 4 {
		t.Fatalf("patch unit: %d %v", resp.StatusCode, body)
	}

	// unknown ids → 404
	resp, _ = do(t, ts, "PATCH", "/buildings/none", map[string]any{"name": "x"}, managerID)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("patch missing building should 404, got %d", resp.StatusCode)
	}
	resp, _ = do(t, ts, "PATCH", "/units/none", map[string]any{"status": "occupied"}, managerID)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("patch missing unit should 404, got %d", resp.StatusCode)
	}
}
