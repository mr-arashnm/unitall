# Unital — Notification Service Design

> Multi-channel notification server (P5 core of the Communications
> bounded context): templates, targeting, fan-out, delivery with retries,
> and pluggable channels — in-app inbox, **email (SMTP)**, **SMS**,
> webhook, and a clear path for push (FCM) and IM bots (Telegram).

## 1. Position in the system

```
 domain events (charge.overdue, booking.confirmed, …)          manager/API
        │                                                      │
        ▼                                                      ▼
┌──────────────────────────────────────────────────────────────────────┐
│                     notifications service (:9006)                    │
│                                                                      │
│  ┌─────────┐   ┌───────────┐   ┌──────────┐   ┌──────────────────┐  │
│  │ Event   │──►│ Campaign  │──►│  Outbox  │──►│ Channel adapters │  │
│  │ mapper  │   │ (fan-out) │   │ (retry)  │   │ inapp/email/sms/ │  │
│  └─────────┘   └───────────┘   └──────────┘   │ webhook/push     │  │
│                                     │        └────────┬─────────┘  │
│                              delivery tracking        ▼            │
└─────────────────────────────────────────── provider APIs ──────────┘
                                             (SMTP · Kavenegar · FCM · …)
```

Other services **never** call providers directly — they publish domain
events (`charge.overdue`, `contract.activated`, …) or call the internal
send endpoint. The notification service owns templates, recipient
resolution, channel policy, retries, and delivery state.

## 2. Core concepts

| Concept | Meaning |
|---|---|
| **Template** | Per-event message skeleton with `{{var}}` placeholders, one **variant per channel** (inapp/email/sms have different lengths & tones). Versioned by name+version; sends pin the rendered snapshot. |
| **Target** | Who receives: `all` of a building, by `role` (owners/residents/staff), by explicit user IDs, or by `unit` parties. Resolved by the `RecipientResolver` port (identity/property internal APIs in prod, seeded directory in dev). |
| **Campaign** | One send request → one campaign: resolved recipients × requested channels = **deliveries**. Tracks counts per status. |
| **Delivery** | The unit of work: (campaign, recipient, channel), state machine `pending → sent / failed`, attempt counter, next_retry_at, provider ref. |
| **Inbox message** | What the in-app channel produces — read/unread, kept per user. |
| **Channel policy** | Per-template defaults plus per-send overrides: which channels, quiet hours (don't SMS at 2am — queue for morning), and severity (urgent bypasses quiet hours). |

## 3. Delivery pipeline (transactional outbox)

1. **Accept** — API (`POST /notifications:send`) or event mapper creates a
   campaign; template + target + vars validated.
2. **Resolve** — expand target → user list (cached); fetch per-user channel
   endpoints (email address, phone number, device token) from the resolver.
3. **Render** — substitute `{{vars}}` into each channel variant; failures
   are recorded per delivery (no whole-campaign abort).
4. **Enqueue** — create deliveries (`pending`, `next_retry_at=now`),
   schedule_at respected (campaign stays `scheduled` until due — a tick
   activates it).
5. **Dispatch worker** (ticker, multiple goroutines possible behind the
   same port): claim due deliveries → channel adapter `Send` → mark
   `sent` (with provider ref) or requeue.
6. **Retry policy** — exponential backoff `30s → 2m → 8m → 30m`, max 4
   attempts, then `failed` with the last error retained for inspection.
7. **Quiet hours** — SMS/email deliveries inside quiet window get
   `next_retry_at = window_end` unless severity=urgent.

At-least-once semantics: idempotency key per campaign; adapters treat
provider "already sent" refs as success.

## 4. Channel SPI

```go
type Channel interface {
    Name() string                                  // "inapp" | "email" | "sms" | ...
    Send(ctx context.Context, m Message) (ref string, err error)
}

type Message struct {
    To       string            // email addr / phone MSISDN / user id
    Title    string
    Body     string
    Vars     map[string]string
    Meta     map[string]string // campaign id, trace id, severity
}
```

| Adapter | Implementation | Notes |
|---|---|---|
| `inapp` | writes `InboxStore` | always enabled; the fallback channel |
| `email` | `SMTPSender` interface; net/smtp adapter; dev = log | HTML+text bodies from template; From name per building |
| `sms` | `SMSSender` interface; **Kavenegar** HTTP adapter (`/v1/{key}/sms/send.json`), Ghasedak/MelliPayamak as siblings behind same interface; dev = log | 160-char awareness for template vars; delivery receipt webhook updates status |
| `webhook` | signed HTTP POST (HMAC) | lets other systems subscribe (e.g. Telegram bot bridge, Slack) |
| `push` (next) | FCM v1 interface, stub now | requires device-token registry on identity |

**Provider selection** for SMS is config-driven (`SMS_PROVIDER=kavenegar|ghasedak|log`),
so localities/providers can change without code. All provider secrets come
from env, never stored in templates.

### Iranian SMS specifics (encoded in the design)
- Numbers normalized to MSISDN `+98…`; verification templates must fit
  one segment (70 chars UCS-2 / 160 ASCII) — the renderer warns otherwise.
- Provider send-rate limits: worker semaphore per channel
  (`SMS_MAX_INFLIGHT`, default 20).
- OTP/verification traffic uses a **separate template + line number** so
  transactional SMS is never blocked by marketing volume.

## 5. Event → notification mapping

| Domain event | Template | Channels (default) |
|---|---|---|
| `charge.overdue` | `charge.overdue.reminder` | inapp + sms (respect quiet hours) |
| `charges.generated` | `charges.generated.summary` | inapp |
| `payment.completed` | `payment.receipt` | inapp + email |
| `contract.activated` | `contract.activated.notice` | inapp + email |
| `booking.confirmed` | `booking.confirmed.reminder` | inapp + sms |
| `meeting.scheduled` | `meeting.invite` | inapp + email |
| `announcement.published` | `announcement.broadcast` | inapp (+ sms when urgent) |

Mapping lives in config (map in code today, YAML/env later), so adding an
event doesn't require redeploying producers.

## 6. API (gateway: `/api/v1`)

| Method & path | Purpose |
|---|---|
| `POST /notifications:send` | Create campaign `{template, building_id, target{kind,value}, vars, channels?, schedule_at?}` (idempotency-key honored) |
| `GET /notifications/{id}` | Campaign status + per-channel counters |
| `GET /notifications/{id}/deliveries?status=` | Delivery inspection |
| `GET /me/notifications?unread=true` | My inbox |
| `POST /me/notifications/{id}/read` | Mark read |
| `POST /internal/trigger` | Feed a domain event `{type, building_id, data}` (what the NATS subscriber will call) |
| `GET /healthz` | liveness |

Errors follow the shared problem-details contract; money-free service so
no special formats beyond it.

## 7. Scaling & hardening path

- Store swap: memory → Postgres tables (`templates`, `campaigns`,
  `deliveries`, `inbox`) with `FOR UPDATE SKIP LOCKED` for worker claims.
- Event bus: NATS JetStream subscriber replaces `/internal/trigger`.
- Observability: delivery success rate per channel, retry histogram,
  dead-letter view for `failed` deliveries.
- Suppression list (per user opt-out per channel) — port exists on
  resolver (`ChannelEndpoints` returns only opted-in endpoints).

## 8. What ships now

Go service `backend/services/notifications` with the full pipeline
(campaign fan-out, outbox worker with backoff + quiet hours, event
trigger), channels `inapp` + `email` + `sms` (log adapters default;
Kavenegar HTTP + net/smtp adapters ready behind env config), HTTP API,
and race-tested HTTP suite. Compose + gateway wired.
