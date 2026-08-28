# Unital Redesign — Build Review (P0–P4 slice)

> Review pass over everything delivered in this slice, with verification
> evidence and known gaps. Date: 2026-08-23.

## Delivered

| Artifact | Location | Verification |
|---|---|---|
| Business document (full domain + scenarios + as-is defects) | `docs/BUSINESS_DOCUMENT.md` | derived from full read of all 6 Django apps |
| Project plan (P0–P7, DoD per phase, risks) | `docs/PROJECT_PLAN.md` | — |
| Target architecture (6 services, event catalog, data ownership, sagas) | `docs/ARCHITECTURE.md` | — |
| API design (conventions + complete endpoint catalog v1) | `docs/API_DESIGN.md` | endpoints implemented for identity/property/billing verified by tests |
| UI/UX design (design system, resident + console screens, flows, a11y) | `docs/UIUX_DESIGN.md` | design-system tool run; RTL/Persian-first adaptations documented |
| Go backend monorepo | `backend/` | `gofmt -l` clean, `go vet` clean, `go test ./... -race` all pass |
| Gateway (JWT, prefix strip, user-context injection) | `backend/gateway/` | live smoke test: register 201, unverified login 403 problem-details, no-token /me 401 |
| Resident frontend — **React 19 + TS + Vite 8 + Router 7** (redesigned) | `frontend/` | production build green (80 KB gzip); full GUI suite passed in browser (see below) |
| **OpenAPI 3.1 + Swagger UI** — `openapi/unital-v1.yaml` (36 paths, 25 schemas), gateway serves `/api/schema` + `/api/docs` | `backend/openapi/`, `backend/gateway/` | spec refs validated; browser-verified: UI renders all ops, try-it-out executed live (401 unauth → authorized 200 with real templates) |
| Gateway full routing (identity + property + billing + notifications behind `/api/v1`) | `backend/gateway/` | authenticated end-to-end: building/unit 201, charge-template 201, charges:generate 200 across services |

## Backend verification evidence

- Unit/HTTP tests, race detector on: identity (auth flows, RBAC on
  memberships, validation), property (RBAC, asset same-building rules,
  purchase→ownership / rental→residency contract activation, history
  audit rows), billing (idempotent generation, partial payments,
  overpayment rejection, invoice aggregation, financial report, overdue
  sweep).
- Live run: identity :19001 + gateway :18080; gateway prefix-strip fix
  verified with curl (404 → proper 201/403/401 problem envelopes).
- Ports 9001/8080 on this machine are taken (MinIO) — the repo-root
  `docker-compose.yml` publishes the whole stack on 18000/18080/19001-06
  (frontend/gateway/services) so it runs beside MinIO unchanged.

## Frontend GUI test results (browser, mobile viewport 390×844)

| # | Test point | Result | Evidence |
|---|---|---|---|
| T1 | Login screen renders RTL Persian, flat teal design | PASS | screenshot `gui-test-screenshots/t1_login.png` (visually verified) + DOM snapshot |
| T2 | Login fallback → app shell with balance card, quick actions, announcements, bottom nav | PASS | DOM snapshot: balance ۱٬۹۰۰٬۰۰۰ ریال (correct demo sum), overdue pill, `t2_home.png` |
| T3 | Charges view: 3 charges, Persian numerals, status pills, nav active state | PASS | DOM snapshot + `t3_charges.png` |
| T4 | Request form submit → confirmation card, form reset | PASS | DOM snapshot + `t4_request.png` |
| T5 | Emoji icons violated design system → replaced with inline Lucide SVGs, re-verified | PASS (fix applied) | DOM snapshot: icons `aria-hidden`, no emoji text remains, `t5_home_fixed.png` |

Note: the external vision service refused the CDN URLs for screenshots
t2–t5 (its 400 error), so those were verified via DOM snapshots + saved
screenshots on disk; T1 was visually verified end-to-end.

## Notification service (added 2026-08-23, see NOTIFICATION_DESIGN.md)

Multi-channel notification server at `backend/services/notifications` (:9006):
- **Pipeline**: template → target resolution → render → per-recipient×channel
  deliveries (outbox) → dispatch worker with retry backoff
  (30s/2m/8m/30m, max 4 attempts), quiet hours (22:00–08:00, urgent bypasses).
- **Channels**: `inapp` (inbox store), `email` (net/smtp), `sms` (Kavenegar
  HTTP), `webhook` (HMAC-signed). Providers degrade to log adapters when
  env config is absent — pipeline runs with zero credentials.
- **Events**: `/internal/trigger` maps domain events (`charge.overdue`,
  `payment.completed`, `booking.confirmed`, `charges.generated`) to
  templates; NATS subscriber drops in later on the same usecase.
- **Verified**: full HTTP suite with race detector (fan-out counting,
  RBAC, missing-var behavior, retry/backoff, event trigger, inbox read
  flow) + live smoke on :19006 (event → campaign → 2 deliveries → SMS log
  adapter fired with rendered Persian text → inbox message present).
- Gateway routes `/api/v1/templates` & `/api/v1/notifications`; compose
  entry added with provider env documentation.

## React redesign GUI test results (2026-08-23, Vite dev @5173, 390×844)

| # | Test point | Result | Evidence |
|---|---|---|---|
| R1 | React login screen (brand logo, LTR inputs, validation) | PASS | `r1_login.png` + DOM |
| R2 | Login → demo fallback → Home: gradient hero (۱٬۹۰۰٬۰۰۰ ریال), quick tiles, announcements, pending payments with progress bars | PASS | `r2_home.png` (full page, visually reviewed) + DOM |
| R3 | Charges: period grouping with totals, pay button → toast (role=status) | PASS | `r3_charges_pay.png` + DOM |
| R4 | Requests: form submit → toast + list entry | PASS | DOM (waits passed) |
| R5 | Inbox: meeting RSVP "حضور دارم" → confirmation pill | PASS | DOM |
| R6 | Unit: info card, parking/warehouse, transfer timeline | PASS | DOM |
| R7 | Logout → /login, re-login → home | PASS | DOM |
| Fixes during review | Persian word order ("شارژ معوق دارید"), hero label, persistent demo banner (useState instead of re-read) | PASS | HMR-applied, re-verified |

## Full Python-API parity pass (added 2026-08-23)

Every endpoint of the six Django apps was catalogued and diffed against
the Go backend; the gaps were implemented:

| Django app | Go service | Status |
|---|---|---|
| accounts (`/api/auth/*`) | identity | was complete (redesigned: real verification/reset tokens, memberships) |
| complexes (`/api/buildings/*`) | property | added PATCH building/unit; assets/parties/contracts were already redesigned-in |
| financial (`/api/financial/*`) | billing | added PATCH/DELETE charge-template; generate/pay/confirm/report were in |
| operations (`/api/operations/*`) | operations | **new service**: memory store, HTTP API (:9005), gateway route, compose, spec, tests |
| communications (`/api/communications/*`) | notifications | **new comms surface**: announcements (draft→publish→inbox fan-out, role/user targeting), meetings (RSVP, minutes, signatures, board-meeting privacy), support tickets (threaded responses, staff auto-triage, resolve/close) |
| facilities (`/api/facilities/*`) | facilities | was complete; **now gateway-routed** (`/facilities`, `/bookings`, `/maintenance-windows`) + compose + spec |
| charging | — | dead code in Django (not in INSTALLED_APPS, no urls) — not ported |

Django bugs fixed by the redesign rather than ported: manager-scoped
queryset `FieldError`s (`building__complex` chains), missing
`created_by`/FK fields breaking every POST, `NotificationViewSet`'s
wrong permission class, no-op verification codes, missing booking
overlap validation (Go enforces at creation *and* approval), CORS
installed but never enabled (gateway now has a configurable CORS
middleware).

Verified: `go build`/`go vet`/`go test ./... -race` all green (6
services); live gateway smoke test on 18080 with all 7 processes —
register/login → building/facility/team/template/ticket/announcement/
meeting all created through `/api/v1` with `DEV_TRUST_ALL=1`.

## Known gaps / next steps (by design, ordered)

1. ~~**Services not yet implemented**~~: **Done 2026-08-23** — facilities
   (was implemented, now gateway-routed + in spec), operations (full
   service: teams/tasks/comments/service-requests/performance), and
   communications (announcements with inbox fan-out, meetings with
   RSVP/minutes/signatures, support tickets with threads — all in the
   notifications service). OpenAPI now 83 paths / 106 operations /
   45 schemas. Remaining surface: unit-target resolution (needs the
   property internal API), meeting-invite fan-out, ticket→team linkage
   (ops teams not yet visible to notifications).
2. **Persistence**: in-memory stores only; postgres adapters + golang-migrate
   SQL are the first hardening task (ports already shaped for it).
3. **Events**: LogPublisher only; NATS JetStream adapter drops in behind
   `events.Publisher`.
4. **Auth hardening**: HS256 shared secret → RS256; rate limiting at
   gateway; lockout counters.
5. **Email verification**: dev mailer drops tokens; SMTP adapter +
   real token delivery needed before any user-facing signup.
6. **Frontend**: shell only; console app and full resident screens come
   next; static shell migrates into the Vite React app once node
   works on this machine (node is currently broken: GLIBC_2.44 mismatch).
7. **Django migration ETL**: not started (P7), source DB still
   `db.sqlite3` in repo root.
