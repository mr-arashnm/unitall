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
	"unital/backend/services/billing/internal/adapter/store/memory"
	"unital/backend/services/billing/internal/usecase"
)

const (
	managerID = "user-manager"
	ownerID   = "user-owner"
	outsider  = "user-outsider"
	building  = "bld-1"
	unitA     = "unit-a"
	unitB     = "unit-b"
)

func newTestServer(t *testing.T) (*httptest.Server, *usecase.Billing) {
	t.Helper()
	b := memory.New()
	b.Membership.Seed(managerID, building, "manager")
	b.Membership.Seed(ownerID, building, "owner")
	b.Directory.SeedUnits(building, unitA, unitB)
	billing := usecase.New(b.Templates, b.Charges, b.Txs, b.Invoices, b.Directory, b.Membership, events.LogPublisher{})
	signer := jwtx.NewSigner("test-secret", 15*time.Minute)
	ts := httptest.NewServer(NewServer(New(billing, signer)))
	t.Cleanup(ts.Close)
	return ts, billing
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

func TestChargeGenerationIsIdempotent(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, body := do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charge-templates", building), map[string]any{
		"name": "Monthly charge", "type": "monthly", "amount": 2_500_000,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("template: %d %v", resp.StatusCode, body)
	}

	resp, body = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-06", "due_in_days": 14,
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["created"].(float64) != 2 {
		t.Fatalf("generate: %d %v", resp.StatusCode, body)
	}

	// rerun: idempotent
	resp, body = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-06",
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["created"].(float64) != 0 {
		t.Fatalf("regenerate should create 0, got %d %v", resp.StatusCode, body)
	}

	// outsider cannot generate
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-06",
	}, outsider)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider generate should 403, got %d", resp.StatusCode)
	}
}

func TestPartialPaymentAndInvoice(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charge-templates", building), map[string]any{
		"name": "Monthly", "type": "monthly", "amount": 1_000_000,
	}, managerID)
	do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-06",
	}, managerID)

	resp, body := do(t, ts, "GET", fmt.Sprintf("/buildings/%s/charges?unit_id=%s", building, unitA), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("charges: %d", resp.StatusCode)
	}
	charge := body["data"].([]any)[0].(map[string]any)
	chargeID := charge["id"].(string)
	if charge["status"] != "pending" || charge["remaining"].(float64) != 1_000_000 {
		t.Fatalf("charge wrong: %v", charge)
	}

	// owner records cash payment of 400k; manager confirms
	resp, body = do(t, ts, "POST", fmt.Sprintf("/charges/%s/payments", chargeID), map[string]any{
		"amount": 400_000, "method": "cash",
	}, ownerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pay: %d %v", resp.StatusCode, body)
	}
	txID := body["id"].(string)
	if ref := body["reference"].(string); len(ref) < 3 || ref[:3] != "TX-" {
		t.Fatalf("reference wrong: %v", body)
	}

	// owner cannot confirm (staff action)
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/transactions/%s/confirm", txID), nil, ownerID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("owner confirm should 403, got %d", resp.StatusCode)
	}

	resp, _ = do(t, ts, "POST", fmt.Sprintf("/transactions/%s/confirm", txID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("confirm: %d", resp.StatusCode)
	}

	resp, body = do(t, ts, "GET", fmt.Sprintf("/buildings/%s/charges?unit_id=%s", building, unitA), nil, managerID)
	charge = body["data"].([]any)[0].(map[string]any)
	if charge["status"] != "partially_paid" || charge["paid"].(float64) != 400_000 || charge["remaining"].(float64) != 600_000 {
		t.Fatalf("charge after partial: %v", charge)
	}

	// invoice reflects totals
	resp, body = do(t, ts, "GET", fmt.Sprintf("/buildings/%s/invoices?unit_id=%s", building, unitA), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("invoices: %d", resp.StatusCode)
	}
	inv := body["data"].([]any)[0].(map[string]any)
	if inv["total"].(float64) != 1_000_000 || inv["paid"].(float64) != 400_000 || inv["is_paid"] != false {
		t.Fatalf("invoice wrong: %v", inv)
	}

	// financial report
	resp, body = do(t, ts, "GET", fmt.Sprintf("/buildings/%s/reports/financial?period=1405-06", building), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("report: %d", resp.StatusCode)
	}
	if body["total_billed"].(float64) != 2_000_000 || body["total_collected"].(float64) != 400_000 {
		t.Fatalf("report wrong: %v", body)
	}
}

func TestOverpaymentRejected(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charge-templates", building), map[string]any{
		"name": "Monthly", "type": "monthly", "amount": 500_000,
	}, managerID)
	do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-06",
	}, managerID)
	_, body := do(t, ts, "GET", fmt.Sprintf("/buildings/%s/charges?unit_id=%s", building, unitA), nil, managerID)
	chargeID := body["data"].([]any)[0].(map[string]any)["id"].(string)

	_, body = do(t, ts, "POST", fmt.Sprintf("/charges/%s/payments", chargeID), map[string]any{
		"amount": 600_000, "method": "cash",
	}, managerID)
	txID := body["id"].(string)
	resp, _ := do(t, ts, "POST", fmt.Sprintf("/transactions/%s/confirm", txID), nil, managerID)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("overpayment confirm should 422, got %d", resp.StatusCode)
	}
}

func TestSweepOverdue(t *testing.T) {
	ts, billing := newTestServer(t)
	do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charge-templates", building), map[string]any{
		"name": "Monthly", "type": "monthly", "amount": 100_000,
	}, managerID)
	do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-05", "due_in_days": 0,
	}, managerID)

	resp, _ := do(t, ts, "POST", "/internal/sweep-overdue", nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sweep: %d", resp.StatusCode)
	}
	_ = billing
	resp, body := do(t, ts, "GET", fmt.Sprintf("/buildings/%s/charges?status=overdue", building), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list overdue: %d", resp.StatusCode)
	}
	if data := body["data"].([]any); len(data) != 2 {
		t.Fatalf("expected 2 overdue charges, got %v", data)
	}
}

func TestTemplatePatchAndDelete(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, body := do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charge-templates", building), map[string]any{
		"name": "Elevator", "type": "elevator", "amount": 300_000,
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create template: %d %v", resp.StatusCode, body)
	}
	tplID := body["id"].(string)

	// manager patches amount and deactivates
	resp, body = do(t, ts, "PATCH", fmt.Sprintf("/charge-templates/%s", tplID), map[string]any{
		"amount": 350_000, "is_active": false,
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["amount"].(float64) != 350_000 || body["is_active"] != false {
		t.Fatalf("patch template: %d %v", resp.StatusCode, body)
	}

	// owner (member, not manager) and outsider cannot patch or delete
	resp, _ = do(t, ts, "PATCH", fmt.Sprintf("/charge-templates/%s", tplID), map[string]any{"amount": 1}, ownerID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("owner patch template should 403, got %d", resp.StatusCode)
	}
	resp, _ = do(t, ts, "DELETE", fmt.Sprintf("/charge-templates/%s", tplID), nil, outsider)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider delete template should 403, got %d", resp.StatusCode)
	}

	// deactivated template is excluded from generation
	resp, body = do(t, ts, "POST", fmt.Sprintf("/buildings/%s/charges:generate", building), map[string]any{
		"period": "1405-08",
	}, managerID)
	if resp.StatusCode != http.StatusOK || body["created"].(float64) != 0 {
		t.Fatalf("inactive template must not generate: %d %v", resp.StatusCode, body)
	}

	// manager deletes; idempotent 404 afterwards
	resp, body = do(t, ts, "DELETE", fmt.Sprintf("/charge-templates/%s", tplID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete template: %d %v", resp.StatusCode, body)
	}
	resp, _ = do(t, ts, "PATCH", fmt.Sprintf("/charge-templates/%s", tplID), map[string]any{"amount": 1}, managerID)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("patch deleted template should 404, got %d", resp.StatusCode)
	}
}
