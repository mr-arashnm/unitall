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

	"unital/backend/pkg/events"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/notifications/internal/adapter/channel"
	"unital/backend/services/notifications/internal/adapter/store/memory"
	"unital/backend/services/notifications/internal/domain"
	"unital/backend/services/notifications/internal/usecase"
)

const (
	managerID = "user-manager"
	ownerID   = "user-owner"  // email + phone
	tenantID  = "user-tenant" // phone only
	building  = "bld-1"
)

// fakeChannel records sends; fails the first failNext attempts.
type fakeChannel struct {
	mu       sync.Mutex
	name     string
	sent     []domain.Message
	failNext int
}

func (f *fakeChannel) Name() string { return f.name }

func (f *fakeChannel) Send(_ context.Context, m domain.Message) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext > 0 {
		f.failNext--
		return "", fmt.Errorf("provider down")
	}
	f.sent = append(f.sent, m)
	return "fake-ref", nil
}

func newTestServer(t *testing.T) (*httptest.Server, *fakeChannel, *usecase.Notifier) {
	t.Helper()
	b := memory.New()
	b.Directory.SeedMembership(managerID, building, "manager")
	b.Directory.SeedMembership(ownerID, building, "owner")
	b.Directory.SeedMembership(tenantID, building, "resident")
	b.Directory.Seed(building,
		domain.Recipient{ID: ownerID, Name: "Sara", Email: "sara@example.com", Phone: "+989120000001"},
		domain.Recipient{ID: tenantID, Name: "Ali", Phone: "+989120000002"},
	)

	fakeSMS := &fakeChannel{name: domain.ChanSMS}
	notifier := usecase.New(b.Templates, b.Campaigns, b.Deliveries, b.Inbox, b.Directory, b.Directory,
		[]domain.Channel{
			&channel.InApp{Inbox: b.Inbox},
			fakeSMS,
		}, events.LogPublisher{})
	comms := usecase.NewComms(b.Announcements, b.Meetings, b.Attendance, b.Minutes,
		b.Tickets, b.TicketResp, b.Inbox, b.Directory, b.Directory, events.LogPublisher{})
	seed(t, notifier)

	signer := jwtx.NewSigner("test-secret", 15*time.Minute)
	ts := httptest.NewServer(NewServer(New(notifier, comms, signer)))
	t.Cleanup(ts.Close)
	return ts, fakeSMS, notifier
}

func seed(t *testing.T, n *usecase.Notifier) {
	t.Helper()
	_, err := n.UpsertTemplate(context.Background(), "system:seed", &domain.Template{
		Name: "charge.overdue.reminder", Severity: domain.SevUrgent, // urgent → skips quiet hours
		Channels: []string{domain.ChanInApp, domain.ChanSMS},
		Variants: map[string]domain.TemplateVariant{
			domain.ChanInApp: {Title: "یادآوری شارژ", Body: "شارژ دوره {{period}} معوق است ({{remaining}} ریال)."},
			domain.ChanSMS:   {Title: "یونیتال", Body: "شارژ {{period}} معوق است"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
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

func TestSendFansOutPerRecipientPerChannel(t *testing.T) {
	ts, fakeSMS, notifier := newTestServer(t)

	resp, body := do(t, ts, "POST", "/notifications:send", map[string]any{
		"building_id": building,
		"template":    "charge.overdue.reminder",
		"target":      map[string]any{"kind": "all"},
		"vars":        map[string]string{"period": "1405-06", "remaining": "1500000"},
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("send: %d %v", resp.StatusCode, body)
	}
	campID := body["id"].(string)

	// owner(inapp+sms) + tenant(inapp+sms) = 4 deliveries, all pending
	resp, body = do(t, ts, "GET", "/notifications/"+campID, nil, managerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: %d", resp.StatusCode)
	}
	if got := body["counters"].(map[string]any)["pending"].(float64); got != 4 {
		t.Fatalf("expected 4 pending deliveries, got %v", got)
	}

	// dispatch → 2 SMS sends, 1 inbox message
	if _, err := notifier.Dispatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(fakeSMS.sent) != 2 {
		t.Fatalf("expected 2 sms sends, got %d", len(fakeSMS.sent))
	}
	if fakeSMS.sent[0].Body != "شارژ 1405-06 معوق است" {
		t.Fatalf("sms not rendered: %q", fakeSMS.sent[0].Body)
	}
	msgs, err := notifier.Inbox(context.Background(), ownerID, false)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("owner inbox should have 1 message: %v %v", msgs, err)
	}
	if msgs[0].Title != "یادآوری شارژ" {
		t.Fatalf("inapp title wrong: %q", msgs[0].Title)
	}
}

func TestMissingTemplateVarYieldsNoDeliveries(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, body := do(t, ts, "POST", "/notifications:send", map[string]any{
		"building_id": building,
		"template":    "charge.overdue.reminder",
		"target":      map[string]any{"kind": "all"},
		"vars":        map[string]string{"period": "1405-06"}, // remaining missing
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("send: %d %v", resp.StatusCode, body)
	}
	resp, body = do(t, ts, "GET", "/notifications/"+body["id"].(string), nil, managerID)
	dels := body["deliveries"].([]any)
	chans := map[string]int{}
	for _, d := range dels {
		chans[d.(map[string]any)["channel"].(string)]++
	}
	// inapp variant needs {{remaining}} → skipped; sms variant fits without it
	if chans[domain.ChanInApp] != 0 || chans[domain.ChanSMS] != 2 {
		t.Fatalf("missing var: want 0 inapp + 2 sms, got %v", chans)
	}
}

func TestRBACOnlyManagersSend(t *testing.T) {
	ts, _, _ := newTestServer(t)
	resp, _ := do(t, ts, "POST", "/notifications:send", map[string]any{
		"building_id": building,
		"template":    "charge.overdue.reminder",
		"target":      map[string]any{"kind": "users", "values": []string{ownerID}},
		"vars":        map[string]string{"period": "1405-06", "remaining": "1"},
	}, ownerID)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("owner send should 403, got %d", resp.StatusCode)
	}
}

func TestRetryBackoffAfterProviderFailure(t *testing.T) {
	b := memory.New()
	b.Directory.SeedMembership("system:test", building, "manager")
	b.Directory.Seed(building, domain.Recipient{ID: ownerID, Phone: "+989120000001"})
	fake := &fakeChannel{name: domain.ChanSMS, failNext: 2}
	n := usecase.New(b.Templates, b.Campaigns, b.Deliveries, b.Inbox, b.Directory, b.Directory,
		[]domain.Channel{fake}, events.LogPublisher{})
	seed(t, n)

	camp, err := n.Send(context.Background(), usecase.SendInput{
		ActorID: "system:test", BuildingID: building, Template: "charge.overdue.reminder",
		Target:   domain.Target{Kind: domain.TargetAll},
		Vars:     map[string]string{"period": "1405-06", "remaining": "1"},
		Channels: []string{domain.ChanSMS}, // only sms registered in this harness
	})
	if err != nil {
		t.Fatal(err)
	}

	// attempt 1 fails; delivery requeued with attempts=1 and future retry time
	if _, err := n.Dispatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(fake.sent) != 0 {
		t.Fatal("first attempt should fail")
	}
	_, dels, err := n.Campaign(context.Background(), camp.ID)
	if err != nil || len(dels) != 1 {
		t.Fatalf("deliveries: %v %v", dels, err)
	}
	if dels[0].Status != domain.DelivPending || dels[0].Attempts != 1 {
		t.Fatalf("after failure: %+v", dels[0])
	}
	if !dels[0].NextRetryAt.After(time.Now()) {
		t.Fatal("retry time should be in the future (backoff)")
	}

	// immediate re-dispatch: nothing due, provider untouched
	if n2, _ := n.Dispatch(context.Background(), 10); n2 != 0 {
		t.Fatal("no delivery should be due during backoff")
	}
	if fake.mu.TryLock() {
		defer fake.mu.Unlock()
		if len(fake.sent) != 0 || fake.failNext != 1 {
			t.Fatal("provider should not have been called during backoff")
		}
	}
}

func TestEventTriggerMapsToTemplateAndDelivers(t *testing.T) {
	ts, fakeSMS, notifier := newTestServer(t)

	resp, body := do(t, ts, "POST", "/internal/trigger", map[string]any{
		"type": "charge.overdue", "building_id": building,
		"data": map[string]string{"user_id": ownerID, "period": "1405-06", "remaining": "900000"},
	}, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("trigger: %d %v", resp.StatusCode, body)
	}
	if _, err := notifier.Dispatch(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(fakeSMS.sent) != 1 {
		t.Fatalf("event should send 1 sms, got %d", len(fakeSMS.sent))
	}
	msgs, _ := notifier.Inbox(context.Background(), ownerID, false)
	if len(msgs) != 1 {
		t.Fatalf("event should push 1 inbox message, got %d", len(msgs))
	}

	// unmapped events are a clean 422
	resp, _ = do(t, ts, "POST", "/internal/trigger", map[string]any{
		"type": "unknown.event", "building_id": building, "data": map[string]string{},
	}, "")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("unmapped event should 422, got %d", resp.StatusCode)
	}
}

func TestInboxReadFlow(t *testing.T) {
	ts, _, _ := newTestServer(t)

	resp, body := do(t, ts, "POST", "/notifications:send", map[string]any{
		"building_id": building,
		"template":    "charge.overdue.reminder",
		"target":      map[string]any{"kind": "users", "values": []string{ownerID}},
		"vars":        map[string]string{"period": "1405-06", "remaining": "1"},
	}, managerID)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("send: %d", resp.StatusCode)
	}
	_ = body

	resp, body = do(t, ts, "GET", "/me/notifications", nil, ownerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inbox: %d", resp.StatusCode)
	}
	// deliveries not dispatched yet → inbox empty until worker runs
	// (this test uses the HTTP dispatch endpoint to drive it)
	do(t, ts, "POST", "/internal/dispatch", nil, "")
	resp, body = do(t, ts, "GET", "/me/notifications", nil, ownerID)
	data := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("inbox should have 1 message after dispatch, got %v", data)
	}
	msg := data[0].(map[string]any)
	if msg["read"] != false {
		t.Fatalf("message should be unread, got %v", msg)
	}
	resp, _ = do(t, ts, "POST", "/me/notifications/"+msg["id"].(string)+"/read", nil, ownerID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mark read: %d", resp.StatusCode)
	}
	resp, body = do(t, ts, "GET", "/me/notifications?unread=true", nil, ownerID)
	if d := body["data"].([]any); len(d) != 0 {
		t.Fatalf("no unread should remain, got %v", d)
	}
}
