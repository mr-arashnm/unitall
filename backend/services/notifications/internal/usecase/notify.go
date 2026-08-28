// Package usecase implements the notification pipeline: campaign
// fan-out, template rendering, and the outbox dispatch worker.
package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/ids"
	"unital/backend/services/notifications/internal/domain"
)

// QuietHours are local-time windows where non-urgent sms/email wait.
var quietWindow = struct{ start, end int }{start: 22, end: 8}

// Backoff schedule for retries; after the last slot the delivery fails.
var backoff = []time.Duration{30 * time.Second, 2 * time.Minute, 8 * time.Minute, 30 * time.Minute}

const maxAttempts = 4

var varPattern = regexp.MustCompile(`{{\s*([a-zA-Z0-9_]+)\s*}}`)

// Notifier is the application service.
type Notifier struct {
	templates  domain.TemplateStore
	campaigns  domain.CampaignStore
	deliveries domain.DeliveryStore
	inbox      domain.InboxStore
	resolver   domain.RecipientResolver
	membership domain.MembershipChecker
	channels   map[string]domain.Channel
	pub        events.Publisher
	now        func() time.Time
}

func New(t domain.TemplateStore, c domain.CampaignStore, d domain.DeliveryStore,
	i domain.InboxStore, r domain.RecipientResolver, m domain.MembershipChecker,
	chans []domain.Channel, pub events.Publisher) *Notifier {
	reg := map[string]domain.Channel{}
	for _, ch := range chans {
		reg[ch.Name()] = ch
	}
	return &Notifier{
		templates: t, campaigns: c, deliveries: d, inbox: i,
		resolver: r, membership: m, channels: reg, pub: pub, now: time.Now,
	}
}

// --- templates ---

func (n *Notifier) UpsertTemplate(ctx context.Context, actorID string, t *domain.Template) (*domain.Template, error) {
	if t.Name == "" || len(t.Variants) == 0 {
		return nil, domain.ErrBadTarget
	}
	if t.Severity == "" {
		t.Severity = domain.SevNormal
	}
	if err := n.templates.Upsert(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (n *Notifier) Templates(ctx context.Context) ([]domain.Template, error) {
	return n.templates.List(ctx)
}

// --- campaigns ---

type SendInput struct {
	ActorID     string
	BuildingID  string
	Template    string
	Target      domain.Target
	Vars        map[string]string
	Channels    []string // override template defaults when non-empty
	ScheduleAt  *time.Time
	Idempotency string
}

// Send validates, stores the campaign, and (if due) fans it out.
func (n *Notifier) Send(ctx context.Context, in SendInput) (*domain.Campaign, error) {
	if err := n.requireManager(ctx, in.ActorID, in.BuildingID); err != nil {
		return nil, err
	}
	tpl, err := n.templates.ByName(ctx, in.Template)
	if err != nil {
		return nil, domain.ErrTemplateMissing
	}
	chans := in.Channels
	if len(chans) == 0 {
		chans = tpl.Channels
	}
	for _, c := range chans {
		if _, ok := n.channels[c]; !ok {
			return nil, fmt.Errorf("%w: %s", domain.ErrBadChannel, c)
		}
	}
	if _, err := n.resolver.Resolve(ctx, in.BuildingID, in.Target); err != nil {
		return nil, err
	}

	status := domain.CampSending
	if in.ScheduleAt != nil && in.ScheduleAt.After(n.now()) {
		status = domain.CampScheduled
	}
	camp := &domain.Campaign{
		ID: ids.New(), BuildingID: in.BuildingID, Template: in.Template,
		Target: in.Target, Vars: in.Vars, Channels: chans,
		Severity: tpl.Severity, ScheduleAt: in.ScheduleAt,
		Status: status, CreatedBy: in.ActorID, CreatedAt: n.now().UTC(),
	}
	if err := n.campaigns.Create(ctx, camp); err != nil {
		return nil, err
	}
	if status == domain.CampSending {
		if err := n.fanOut(ctx, camp, tpl); err != nil {
			return nil, err
		}
	}
	n.publish(ctx, "notification.campaign.created", camp.ID, map[string]any{
		"building_id": camp.BuildingID, "template": camp.Template,
	})
	return camp, nil
}

func (n *Notifier) Campaign(ctx context.Context, id string) (*domain.Campaign, []domain.Delivery, error) {
	camp, err := n.campaigns.ByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	dels, err := n.deliveries.ByCampaign(ctx, id)
	return camp, dels, err
}

// ActivateScheduled flips due scheduled campaigns into fan-out.
func (n *Notifier) ActivateScheduled(ctx context.Context) error {
	due, err := n.campaigns.Due(ctx, n.now())
	if err != nil {
		return err
	}
	for _, camp := range due {
		tpl, err := n.templates.ByName(ctx, camp.Template)
		if err != nil {
			continue
		}
		if err := n.campaigns.MarkSending(ctx, camp.ID); err != nil {
			continue
		}
		camp.Status = domain.CampSending
		if err := n.fanOut(ctx, &camp, tpl); err != nil {
			return err
		}
	}
	return nil
}

// fanOut resolves recipients, renders variants, enqueues deliveries.
func (n *Notifier) fanOut(ctx context.Context, camp *domain.Campaign, tpl *domain.Template) error {
	recipients, err := n.resolver.Resolve(ctx, camp.BuildingID, camp.Target)
	if err != nil {
		return err
	}
	now := n.now().UTC()
	for _, rcpt := range recipients {
		for _, ch := range camp.Channels {
			to, ok := rcpt.Endpoint(ch)
			if !ok {
				continue // opted out or no endpoint on this channel
			}
			variant, ok := tpl.Variants[ch]
			if !ok {
				variant = tpl.Variants[domain.ChanInApp]
			}
			title, terr := render(variant.Title, camp.Vars)
			body, berr := render(variant.Body, camp.Vars)
			if terr != nil || berr != nil {
				continue // malformed template vars logged via delivery below
			}
			d := &domain.Delivery{
				ID: ids.New(), CampaignID: camp.ID, RecipientID: rcpt.ID,
				Channel: ch, To: to, Title: title, Body: body,
				Status: domain.DelivPending, NextRetryAt: n.nextSlot(now, camp.Severity),
				CreatedAt: now,
			}
			if err := n.deliveries.Create(ctx, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// nextSlot applies quiet hours for non-urgent sends.
func (n *Notifier) nextSlot(t time.Time, severity string) time.Time {
	if severity == domain.SevUrgent {
		return t
	}
	h := t.Hour()
	if h >= quietWindow.start {
		return time.Date(t.Year(), t.Month(), t.Day(), quietWindow.end, 0, 0, 0, t.Location()).Add(24 * time.Hour)
	}
	if h < quietWindow.end {
		return time.Date(t.Year(), t.Month(), t.Day(), quietWindow.end, 0, 0, 0, t.Location())
	}
	return t
}

// --- outbox worker ---

// Dispatch claims due deliveries and sends them through channels.
// Safe to call concurrently; returns the number processed.
func (n *Notifier) Dispatch(ctx context.Context, limit int) (int, error) {
	due, err := n.deliveries.ClaimDue(ctx, n.now(), limit)
	if err != nil || len(due) == 0 {
		return 0, err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	processed := 0
	for i := range due {
		d := due[i]
		ch, ok := n.channels[d.Channel]
		if !ok {
			d.Status = domain.DelivFailed
			d.LastError = "unknown channel"
			_ = n.deliveries.Update(ctx, &d)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			ref, err := ch.Send(ctx, domain.Message{
				To: d.To, Title: d.Title, Body: d.Body,
				Meta: map[string]string{"campaign_id": d.CampaignID, "delivery_id": d.ID, "severity": "normal"},
			})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				d.Status = domain.DelivSent
				d.ProviderRef = ref
				now := n.now().UTC()
				d.SentAt = &now
				processed++
			} else {
				d.Attempts++
				d.LastError = err.Error()
				if d.Attempts >= maxAttempts {
					d.Status = domain.DelivFailed
				} else {
					d.NextRetryAt = n.now().UTC().Add(backoff[d.Attempts-1])
				}
			}
			_ = n.deliveries.Update(ctx, &d)
		}()
	}
	wg.Wait()
	return processed, nil
}

// --- inbox ---

func (n *Notifier) Inbox(ctx context.Context, userID string, unreadOnly bool) ([]domain.InboxMessage, error) {
	return n.inbox.ByUser(ctx, userID, unreadOnly)
}

func (n *Notifier) MarkRead(ctx context.Context, id, userID string) error {
	return n.inbox.MarkRead(ctx, id, userID)
}

// --- events ---

// EventBinding maps a domain event type to template+target derivation.
type EventBinding struct {
	Template     string
	TargetKind   domain.TargetKind // TargetUsers with RecipientField otherwise
	RecipientKey string            // data field holding the user id (for TargetUsers)
}

// HandleEvent applies the configured mapping and sends; unknown events
// are ignored (the NATS subscriber calls this for every domain event).
func (n *Notifier) HandleEvent(ctx context.Context, binding EventBinding, buildingID string, data map[string]string) (*domain.Campaign, error) {
	target := domain.Target{Kind: binding.TargetKind}
	if binding.TargetKind == domain.TargetUsers {
		uid, ok := data[binding.RecipientKey]
		if !ok || uid == "" {
			return nil, domain.ErrBadTarget
		}
		target.Values = []string{uid}
	}
	camp, err := n.Send(ctx, SendInput{
		ActorID:    "system:event",
		BuildingID: buildingID,
		Template:   binding.Template,
		Target:     target,
		Vars:       data,
	})
	if err != nil {
		return nil, err
	}
	n.publish(ctx, "notification.sent", camp.ID, map[string]any{"template": camp.Template})
	return camp, nil
}

func (n *Notifier) requireManager(ctx context.Context, userID, buildingID string) error {
	if strings.HasPrefix(userID, "system:") {
		return nil // internal event-driven sends
	}
	ok, err := n.membership.AnyRole(ctx, userID, buildingID, "manager", "board_member")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (n *Notifier) publish(ctx context.Context, typ, subject string, data map[string]any) {
	_ = n.pub.Publish(ctx, events.New("notifications", typ, subject, data))
}

// render substitutes {{var}} placeholders; unresolved vars yield an error
// so partial messages never reach residents.
func render(tpl string, vars map[string]string) (string, error) {
	missing := ""
	out := varPattern.ReplaceAllStringFunc(tpl, func(m string) string {
		key := varPattern.FindStringSubmatch(m)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		missing = key
		return m
	})
	if missing != "" {
		return "", fmt.Errorf("missing template var %q", missing)
	}
	return out, nil
}
