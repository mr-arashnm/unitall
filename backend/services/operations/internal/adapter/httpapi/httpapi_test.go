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
	"unital/backend/services/operations/internal/adapter/store/memory"
	"unital/backend/services/operations/internal/usecase"
)

const (
	managerID = "user-manager"
	staffID   = "user-staff"
	resident  = "user-resident"
	outsider  = "user-outsider"
	building  = "bld-1"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	b := memory.New()
	b.Membership.Seed(managerID, building, "manager")
	b.Membership.Seed(staffID, building, "staff")
	b.Membership.Seed(resident, building, "resident")
	ops := usecase.New(b.Teams, b.Tasks, b.Comments, b.Requests, b.Membership, events.LogPublisher{})
	ts := httptest.NewServer(NewServer(New(ops, jwtx.NewSigner("t", time.Minute))))
	t.Cleanup(ts.Close)
	return ts
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

func setupTeam(t *testing.T, ts *httptest.Server) string {
	resp, body := do(t, ts, "POST", "/buildings/"+building+"/teams", map[string]any{
		"name": "تیم نگهداری", "type": "maintenance",
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create team: %d %v", resp.StatusCode, body)
	}
	return body["id"].(string)
}

func TestTeamManagement(t *testing.T) {
	ts := newTestServer(t)

	// resident cannot create a team
	resp, _ := do(t, ts, "POST", "/buildings/"+building+"/teams", map[string]any{"name": "x"}, resident)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("resident create team should 403, got %d", resp.StatusCode)
	}

	teamID := setupTeam(t, ts)

	// add a staff member
	resp, body := do(t, ts, "POST", fmt.Sprintf("/teams/%s/members", teamID), map[string]any{"user_id": staffID}, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add member: %d %v", resp.StatusCode, body)
	}
	if len(body["members"].([]any)) != 1 {
		t.Fatalf("member not added: %v", body["members"])
	}

	// remove the member again
	resp, body = do(t, ts, "DELETE", fmt.Sprintf("/teams/%s/members/%s", teamID, staffID), nil, managerID)
	if resp.StatusCode != http.StatusOK || len(body["members"].([]any)) != 0 {
		t.Fatalf("remove member: %d %v", resp.StatusCode, body)
	}

	// non-member outsider cannot list teams
	resp, _ = do(t, ts, "GET", "/buildings/"+building+"/teams", nil, outsider)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider list teams should 403, got %d", resp.StatusCode)
	}
}

func TestTaskLifecycle(t *testing.T) {
	ts := newTestServer(t)
	teamID := setupTeam(t, ts)
	_, _ = do(t, ts, "POST", fmt.Sprintf("/teams/%s/members", teamID), map[string]any{"user_id": staffID}, managerID)

	resp, body := do(t, ts, "POST", fmt.Sprintf("/teams/%s/tasks", teamID), map[string]any{
		"title": "بازدید آسانسور", "priority": "high", "estimated_hours": 2,
		"due_date": time.Now().UTC().AddDate(0, 0, 2).Format(time.RFC3339),
	}, managerID)
	if resp.StatusCode != http.StatusCreated || body["status"] != "pending" {
		t.Fatalf("create task: %d %v", resp.StatusCode, body)
	}
	taskID := body["id"].(string)

	// staff self-view: team member sees the task
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/tasks", nil, staffID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("staff should see team task: %d %v", resp.StatusCode, body)
	}
	// resident sees nothing
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/tasks", nil, resident)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 0 {
		t.Fatalf("resident should see no tasks: %d %v", resp.StatusCode, body)
	}

	// assign → start → complete (with actual hours)
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tasks/%s/assign", taskID), map[string]any{"staff_id": staffID}, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "assigned" || body["assigned_to"] != staffID {
		t.Fatalf("assign: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tasks/%s/start", taskID), nil, staffID)
	if resp.StatusCode != http.StatusOK || body["status"] != "in_progress" {
		t.Fatalf("start: %d %v", resp.StatusCode, body)
	}
	// invalid: complete by outsider
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/tasks/%s/complete", taskID), map[string]any{"actual_hours": 3}, resident)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider complete should 403, got %d", resp.StatusCode)
	}
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tasks/%s/complete", taskID), map[string]any{"actual_hours": 3}, staffID)
	if resp.StatusCode != http.StatusOK || body["status"] != "completed" {
		t.Fatalf("complete: %d %v", resp.StatusCode, body)
	}
	if body["actual_hours"].(float64) != 3 {
		t.Fatalf("actual_hours not recorded: %v", body["actual_hours"])
	}
	// completed task cannot be completed again
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/tasks/%s/complete", taskID), nil, staffID)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("re-complete should 422, got %d", resp.StatusCode)
	}

	// comments
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tasks/%s/comments", taskID), map[string]any{"comment": "تمام شد"}, staffID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("comment: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "GET", fmt.Sprintf("/tasks/%s/comments", taskID), nil, managerID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("list comments: %d %v", resp.StatusCode, body)
	}

	// performance reflects the completed task
	resp, body = do(t, ts, "GET", fmt.Sprintf("/teams/%s/performance", teamID), nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("performance: %d %v", resp.StatusCode, body)
	}
	if body["total_tasks"].(float64) != 1 || body["completed_tasks"].(float64) != 1 || body["completion_rate"].(float64) != 1 {
		t.Fatalf("performance numbers wrong: %v", body)
	}
}

func TestServiceRequestFlow(t *testing.T) {
	ts := newTestServer(t)
	teamID := setupTeam(t, ts)

	// resident submits a maintenance request for their unit
	resp, body := do(t, ts, "POST", "/buildings/"+building+"/service-requests", map[string]any{
		"unit_id": "unit-1", "title": "نشت آب", "type": "maintenance",
	}, resident)
	if resp.StatusCode != http.StatusCreated || body["status"] != "submitted" {
		t.Fatalf("submit: %d %v", resp.StatusCode, body)
	}
	reqID := body["id"].(string)

	// outsider of the building cannot submit
	resp, _ = do(t, ts, "POST", "/buildings/"+building+"/service-requests", map[string]any{
		"unit_id": "unit-1", "title": "x",
	}, outsider)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider submit should 403, got %d", resp.StatusCode)
	}

	// manager assigns to team → linked task is created
	resp, body = do(t, ts, "POST", fmt.Sprintf("/service-requests/%s/assign", reqID), map[string]any{"team_id": teamID}, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "assigned" {
		t.Fatalf("assign: %d %v", resp.StatusCode, body)
	}
	related := body["related_task"].(string)
	if related == "" {
		t.Fatalf("linked task missing: %v", body)
	}
	resp, body = do(t, ts, "GET", fmt.Sprintf("/tasks/%s", related), nil, managerID)
	if resp.StatusCode != http.StatusOK || body["title"] != "نشت آب" {
		t.Fatalf("linked task lookup: %d %v", resp.StatusCode, body)
	}

	// visibility: resident sees only their own request, staff sees all
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/service-requests", nil, resident)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("resident should see own request: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/service-requests", nil, staffID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("staff should see building requests: %d %v", resp.StatusCode, body)
	}
	// detail: outsider denied
	resp, _ = do(t, ts, "GET", fmt.Sprintf("/service-requests/%s", reqID), nil, outsider)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("outsider detail should 403, got %d", resp.StatusCode)
	}
}
