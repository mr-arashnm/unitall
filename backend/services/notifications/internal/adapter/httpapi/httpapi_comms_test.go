package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAnnouncementPublishFansOutToInbox(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// resident cannot author announcements
	resp, _ := do(t, ts, "POST", "/buildings/"+building+"/announcements",
		map[string]any{"title": "اطلاعیه", "content": "سلام"}, tenantID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("resident author should 403, got %d", resp.StatusCode)
	}

	resp, body := do(t, ts, "POST", "/buildings/"+building+"/announcements",
		map[string]any{"title": "قطعی آب", "content": "پنج‌شنبه آب قطع است", "priority": "high"}, managerID)
	if resp.StatusCode != http.StatusCreated || body["status"] != "draft" {
		t.Fatalf("create announcement: %d %v", resp.StatusCode, body)
	}
	annID := body["id"].(string)

	// drafts are invisible to residents
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/announcements", nil, tenantID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 0 {
		t.Fatalf("resident should not see drafts: %d %v", resp.StatusCode, body)
	}

	// resident cannot publish
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/announcements/%s/publish", annID), nil, tenantID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("resident publish should 403, got %d", resp.StatusCode)
	}

	// publish → every seeded member (owner + tenant) gets an inbox message
	resp, body = do(t, ts, "POST", fmt.Sprintf("/announcements/%s/publish", annID), nil, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "published" || body["delivered_to"].(float64) != 2 {
		t.Fatalf("publish: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "GET", "/me/notifications", nil, tenantID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("tenant inbox should hold the announcement: %d %v", resp.StatusCode, body)
	}
	if body["data"].([]any)[0].(map[string]any)["title"] != "قطعی آب" {
		t.Fatalf("inbox message mismatch: %v", body["data"])
	}

	// published announcements are immutable
	resp, _ = do(t, ts, "PATCH", fmt.Sprintf("/announcements/%s", annID),
		map[string]any{"title": "ویرایش"}, managerID)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("patch published should 422, got %d", resp.StatusCode)
	}

	// role-targeted announcement reaches only owners
	resp, body = do(t, ts, "POST", "/buildings/"+building+"/announcements", map[string]any{
		"title": "هیئت‌مدیره", "content": "جلسه بعدی",
		"target": map[string]any{"kind": "role", "values": []string{"owner"}},
	}, managerID)
	roleAnn := body["id"].(string)
	resp, body = do(t, ts, "POST", fmt.Sprintf("/announcements/%s/publish", roleAnn), nil, managerID)
	if body["delivered_to"].(float64) != 1 {
		t.Fatalf("role target should reach 1 owner, got %v", body["delivered_to"])
	}
	resp, body = do(t, ts, "GET", "/me/notifications", nil, ownerID)
	if len(body["data"].([]any)) != 2 {
		t.Fatalf("owner should see both announcements: %v", body["data"])
	}
	resp, body = do(t, ts, "GET", "/me/notifications", nil, tenantID)
	if len(body["data"].([]any)) != 1 {
		t.Fatalf("tenant should still see one: %v", body["data"])
	}
}

func TestMeetingRSVPAndMinutes(t *testing.T) {
	ts, _, _ := newTestServer(t)
	when := time.Now().UTC().AddDate(0, 0, 7).Format(time.RFC3339)

	// a board meeting (private) and a general assembly
	resp, body := do(t, ts, "POST", "/buildings/"+building+"/meetings", map[string]any{
		"title": "جلسه هیئت‌مدیره", "type": "board", "scheduled_at": when, "location": "دفتر",
	}, managerID)
	if resp.StatusCode != http.StatusCreated || body["status"] != "scheduled" {
		t.Fatalf("create board meeting: %d %v", resp.StatusCode, body)
	}
	boardID := body["id"].(string)
	resp, body = do(t, ts, "POST", "/buildings/"+building+"/meetings", map[string]any{
		"title": "مجمع عمومی", "type": "general", "scheduled_at": when,
	}, managerID)
	generalID := body["id"].(string)

	// residents see only the general assembly
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/meetings", nil, tenantID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("resident should see only general meeting: %d %v", resp.StatusCode, body)
	}
	if body["data"].([]any)[0].(map[string]any)["type"] != "general" {
		t.Fatalf("wrong meeting visible: %v", body["data"])
	}

	// RSVP upserts: latest status wins
	resp, body = do(t, ts, "POST", fmt.Sprintf("/meetings/%s/rsvp", generalID),
		map[string]any{"status": "declined"}, tenantID)
	if resp.StatusCode != http.StatusOK || body["status"] != "declined" {
		t.Fatalf("rsvp: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "POST", fmt.Sprintf("/meetings/%s/rsvp", generalID),
		map[string]any{"status": "confirmed"}, tenantID)
	if resp.StatusCode != http.StatusOK || body["status"] != "confirmed" {
		t.Fatalf("rsvp update: %d %v", resp.StatusCode, body)
	}
	// invalid RSVP status
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/meetings/%s/rsvp", generalID),
		map[string]any{"status": "maybe"}, tenantID)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad rsvp should 422, got %d", resp.StatusCode)
	}

	// attendance roster is manager-only
	resp, body = do(t, ts, "GET", fmt.Sprintf("/meetings/%s/attendance", generalID), nil, managerID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 1 {
		t.Fatalf("attendance: %d %v", resp.StatusCode, body)
	}
	resp, _ = do(t, ts, "GET", fmt.Sprintf("/meetings/%s/attendance", generalID), nil, tenantID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("tenant attendance should 403, got %d", resp.StatusCode)
	}

	// minutes upsert + signatures
	resp, _ = do(t, ts, "PUT", fmt.Sprintf("/meetings/%s/minutes", boardID), map[string]any{
		"content": "متن صورتجلسه", "decisions": "تصمیم ۱", "action_items": "اقدام ۱",
	}, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upsert minutes: %d", resp.StatusCode)
	}
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/meetings/%s/minutes/sign", boardID), nil, "user-outsider")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-member sign should 403, got %d", resp.StatusCode)
	}
	resp, body = do(t, ts, "POST", fmt.Sprintf("/meetings/%s/minutes/sign", boardID), nil, ownerID)
	if resp.StatusCode != http.StatusOK || len(body["signed_by"].([]any)) != 1 {
		t.Fatalf("sign: %d %v", resp.StatusCode, body)
	}
	// idempotent double sign
	resp, body = do(t, ts, "POST", fmt.Sprintf("/meetings/%s/minutes/sign", boardID), nil, ownerID)
	if len(body["signed_by"].([]any)) != 1 {
		t.Fatalf("double sign should be idempotent: %v", body["signed_by"])
	}
}

func TestSupportTicketThread(t *testing.T) {
	ts, _, _ := newTestServer(t)

	resp, body := do(t, ts, "POST", "/buildings/"+building+"/tickets", map[string]any{
		"unit_id": "unit-9", "title": "آسانسور صدادار است", "type": "technical",
	}, tenantID)
	if resp.StatusCode != http.StatusCreated || body["status"] != "open" {
		t.Fatalf("submit ticket: %d %v", resp.StatusCode, body)
	}
	ticketID := body["id"].(string)

	// missing unit rejected
	resp, _ = do(t, ts, "POST", "/buildings/"+building+"/tickets", map[string]any{"title": "x"}, tenantID)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("ticket without unit should 422, got %d", resp.StatusCode)
	}

	// staff reply triages an open ticket: in_progress + assigned
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tickets/%s/responses", ticketID),
		map[string]any{"message": "بررسی می‌شود"}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("respond: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "GET", fmt.Sprintf("/tickets/%s", ticketID), nil, tenantID)
	ticket := body["ticket"].(map[string]any)
	if ticket["status"] != "in_progress" || ticket["assigned_to"] != managerID {
		t.Fatalf("ticket not triaged: %v", ticket)
	}
	if len(body["responses"].([]any)) != 1 {
		t.Fatalf("responses missing: %v", body["responses"])
	}

	// submitter may reply on their own thread
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/tickets/%s/responses", ticketID),
		map[string]any{"message": "ممنون"}, tenantID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("submitter reply failed: %d", resp.StatusCode)
	}

	// resolve then close; double-resolve rejected
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tickets/%s/resolve", ticketID), nil, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "resolved" {
		t.Fatalf("resolve: %d %v", resp.StatusCode, body)
	}
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/tickets/%s/resolve", ticketID), nil, managerID)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("double resolve should 422, got %d", resp.StatusCode)
	}
	resp, body = do(t, ts, "POST", fmt.Sprintf("/tickets/%s/close", ticketID), nil, managerID)
	if resp.StatusCode != http.StatusOK || body["status"] != "closed" {
		t.Fatalf("close: %d %v", resp.StatusCode, body)
	}

	// resident cannot resolve
	resp, _ = do(t, ts, "POST", fmt.Sprintf("/tickets/%s/close", ticketID), nil, tenantID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("tenant close should 403, got %d", resp.StatusCode)
	}

	// visibility: owner sees none, manager sees the building's ticket
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/tickets", nil, ownerID)
	if resp.StatusCode != http.StatusOK || len(body["data"].([]any)) != 0 {
		t.Fatalf("owner should see no tickets: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "GET", "/buildings/"+building+"/tickets", nil, managerID)
	if len(body["data"].([]any)) != 1 {
		t.Fatalf("manager should see building tickets: %v", body["data"])
	}
}
