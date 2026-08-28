# Unital — Building & Complex Management System
## Full Business Document & Scenario Catalog

> Derived from a full code review of the existing Django/DRF backend (`unital_backend`, branch `backend`).
> Purpose: complete business understanding before redesigning the system as Go microservices with a new API, UI/UX, and frontend.

---

## 1. System Overview

**Unital** is a platform for managing residential and commercial buildings/complexes. It connects three groups:

- **Management** — building managers, board members, and staff who run the building.
- **Residents** — owners and tenants (renters) living in or owning units.
- **The platform** — charging, communications, operations, facility booking, and financial tracking.

Each **Building** (called "complex" throughout the code) is the central anchor: nearly every record (units, charges, bookings, teams, meetings, announcements) hangs off a Building. A per-building **Feature registry** toggles which modules are enabled per building (e.g. `support_tickets`, `meetings`, `notifications`).

### 1.1 User Types (Roles)

| Role | Description | Key capabilities |
|---|---|---|
| `manager` | Building manager | Full control of buildings they are a board member of; approve bookings, activate contracts, generate charges |
| `board_member` | Elected board member | Same data access as manager (code treats them identically) |
| `staff` | Operational staff (security, cleaning, technician…) | Member of teams; assigned tasks and facility maintenance |
| `owner` | Unit owner | Owns one or more units; sees own units' charges, contracts, bookings; submits service requests |
| `resident` | Tenant living in a unit | Same visibility as owner for the unit they reside in |
| `customer` | Generic customer type (defined but unused in any logic) | — |

**Auth**: email + password (JWT access/refresh), phone & national code stored, email verification, password reset flow, avatar upload. A user can be linked to many buildings (as owner, resident, or board member) — `GET /api/auth/my-complexes/` returns them.

### 1.2 Apps / Modules (current Django structure)

| App | Domain |
|---|---|
| `accounts` | Users, auth (register/login/refresh/profile/verify/reset-password) |
| `complexes` | Buildings, units, parkings, warehouses, transfer history, contracts |
| `financial` | Charge templates, charges, transactions, invoices, financial reports |
| `facilities` | Shared facilities, bookings, maintenance, usage rules, images |
| `operations` | Teams, tasks, service requests, task comments, team performance |
| `communications` | Notifications, meetings, attendance, minutes, announcements, support tickets |
| `charging` | **Empty/vestigial app** (11 lines, no real models) |

---

## 2. Domain Model (as-is)

### 2.1 complexes
- **Building**: name, unique code, type (residential/commercial/office/mixed), address, counts (floors/units/parkings/warehouses), M2M `board_members` (managers/board members), `created_by`, M2M `features`.
- **Feature**: registry of togglable module keys per building.
- **Unit**: belongs to a building; floor, unit_number (unique per building), area m², rooms, status (occupied/vacant/under_construction), single `owner` FK and single `resident` FK (both nullable).
- **Parking**: belongs to building, optionally assigned to a unit (available/occupied), floor, unique code.
- **Warehouse**: same pattern as parking, plus area. ⚠️ *The Warehouse model is accidentally defined twice in the same file (duplicated block) — a defect.*
- **UnitTransferHistory**: audit log of ownership/residency changes (previous/new owner & resident, transfer type, contract number/date, recorded_by).
- **Contract**: purchase / rental / transfer contracts on a unit. First party & second party, auto-generated number (`CONTRACT-YYYYMMDD-NNNN`), amount, deposit, duration, status (draft/active/expired/cancelled), signatures, file attachment. `activate_contract()` **applies side effects**: purchase → changes unit owner; rental → changes unit resident.

**Business rules captured here**
- Parking/warehouse can only be assigned to a unit in the same building; only to available ones; release makes them available again.
- Ownership/residency changes are always logged into transfer history (atomic with the unit update).
- Contract activation only from `draft`, and mutates unit ownership/residency depending on type.

### 2.2 financial
- **ChargeTemplate**: per-building recurring charge definitions — monthly, maintenance, elevator, cleaning, security, green space, pool, gym, other; amount; active flag.
- **Charge**: per unit + template + period (`YYYY-MM`, unique triple). Amount, due date, status (pending/paid/overdue/cancelled/partially_paid), paid/remaining amounts.
- **Transaction**: payment against a charge — method (online/cash/bank_transfer/cheque/pos), status (pending/completed/failed/cancelled), auto reference (`TX-XXXXXXXXXXXX`), payment date.
- **Invoice**: aggregate per unit + period — total, paid, remaining, is_paid, due date.

**Flows**: `POST /financial/charges/generate_charges/` mass-generates charges from templates for a period; `POST /financial/transactions/{id}/confirm_payment/` confirms a payment (updates charge paid/remaining/status); `GET /financial/invoices/financial_report/` building financial report.

### 2.3 facilities
- **Facility**: per building — pool, gym, roof garden, meeting room, party hall, guest parking, playground, sports court, library, business center, other. Capacity, min/max advance booking (hours), opening/closing times (overnight windows supported), rules text, hourly rate, is_free, active/under-maintenance flags.
- **FacilityBooking**: user books a facility for start/end (duration auto-computed in whole hours); purpose, participants count, special requirements; status lifecycle pending → confirmed → active → completed / cancelled / rejected; manager approval (approved_by/at); auto cost = hours × hourly rate; is_paid.
- **FacilityMaintenance**: routine/repair/cleaning/inspection/upgrade with priority, scheduled vs actual times, assigned staff, `affect_bookings` flag.
- **FacilityUsageRule**, **FacilityImage** (primary image flag).

**Flows**: availability & time-slot lookup per facility; approve/reject/cancel booking; start/complete maintenance; `my_bookings`.

### 2.4 operations
- **Team**: per building — security, maintenance, cleaning, landscaping, pool, gym, other; supervisor + staff members.
- **Task**: belongs to team, assigned to a staff member; priority; status pending → assigned → in_progress → completed / cancelled / on_hold; due date, estimated vs actual hours (1–24h), optional related unit; comments with attachments.
- **ServiceRequest**: submitted by resident/owner for their unit — maintenance/cleaning/security/complaint/suggestion/other; status submitted → under_review → assigned → in_progress → completed / cancelled; can be assigned to a team and linked to a generated Task.
- **Performance**: monthly per-team KPIs — total/completed/overdue tasks, average completion time, satisfaction rate, completion rate.

**Flows**: add team member; task assign/start/complete; service request assign_to_team (creates a task); team performance report.

### 2.5 communications
- **Notification**: per building — type (charge/meeting/maintenance/security/general/urgent); audience targeting by `target_type` (all/owners/residents/board/specific users), by `target_roles` (JSON), and by `target_teams`; scheduled or immediate send (`send_now`).
- **Meeting**: board/general/committee/emergency; agenda, location, duration; status scheduled → ongoing → completed / cancelled; attendees via **MeetingAttendance** (invited → confirmed/declined → attended/absent; RSVP action); optional role/team targeting; **MeetingMinute** (content, decisions, action items, signatories).
- **Announcement**: per building, priority (low/normal/high/urgent), targeting by units, roles, or teams; published with publish/expiry dates.
- **SupportTicket**: technical/financial/complaint/suggestion/general; open → in_progress → resolved → closed; assigned to manager/staff or a team; threaded **TicketResponse** with attachments and internal-only visibility flag.

---

## 3. API Surface (as-is)

Base: `/api/` — Swagger at `/api/docs/` (drf-spectacular, partially broken).

| Prefix | Endpoints |
|---|---|
| `/api/auth/` | `register`, `login`, `logout`, `refresh-token`, `profile`, `verify-account`, `reset-password-request`, `reset-password-confirm`, `my-complexes` |
| `/api/buildings/` | Building CRUD + nested: `/units`, `/units/{id}`, `/parking`, `/parking/{id}`, `/warehouse`, `/warehouse/{id}`, `/available-parking`, `/available-warehouse`, `/unit/{id}/transfer-history[/{id}]` (contracts endpoints exist but are commented out) |
| `/api/financial/` | `charge-templates`, `charges` (+`generate_charges`), `transactions` (+`{id}/confirm_payment`), `invoices` (+`financial_report`) |
| `/api/facilities/` | `facilities` (+`{id}/availability`, `{id}/time-slots`), `bookings` (+`approve`, `reject`, `cancel`, `my_bookings`), `maintenances` (+`start`, `complete`), `usage-rules`, `images` |
| `/api/operations/` | `teams` (+`{id}/add_member`), `tasks` (+`assign`, `start`, `complete`), `service-requests` (+`{id}/assign_to_team`), `task-comments`, `performance` (+`team_performance`) |
| `/api/communications/` | `notifications` (+`{id}/send_now`), `meetings` (+`{id}/rsvp`), `meeting-minutes`, `meeting-attendance`, `announcements`, `support-tickets` (+`{id}/add_response`, `{id}/resolve`), `ticket-responses` |

**Access control pattern**: every ViewSet filters its queryset by user_type — managers/board members see buildings they manage; owners/residents see only their own units' data. Some mutating actions additionally check `user_type in ['manager','board_member']`. There is no object-level permission framework and no staff scoping in most querysets.

---

## 4. Business Scenarios (all user journeys)

### S1. Onboarding & Setup
1. **Register** — user signs up with email/password (owner or resident initially), verifies account.
2. **Create building** — a manager registers the building: details, code, counts, board members, and enables features.
3. **Define inventory** — manager creates units (floor, number, area, rooms, status), parkings, warehouses.
4. **Link people to units** — manager sets owner/resident on each unit; every change is recorded in transfer history.

### S2. Living in the building (Owner / Resident)
- View "my complexes", my unit details, assigned parkings/warehouses.
- Receive and pay **charges**: monthly and service charges generated by management; pay online/cash/transfer/cheque/POS; partial payments tracked; overdue status.
- View **invoices** aggregated per period; check remaining balance.
- Book **facilities** (e.g. party hall): pick a slot within opening hours and advance-booking windows, state purpose & participants, pay hourly rate if not free, await approval, attend.
- Submit **service requests** (maintenance/cleaning/security/complaint/suggestion), follow status until completion.
- Receive **notifications** & **announcements** targeted to them (by unit, role, or building-wide).
- RSVP to **meetings** (general/board), attend, read minutes afterwards.
- Open **support tickets** for technical/financial issues and converse with management (threaded responses, attachments).

### S3. Buying / Selling / Renting (Ownership & Residency transfers)
1. Manager drafts a **contract** (purchase or rental) between first party (seller/landlord) and second party (buyer/tenant): amount, deposit, duration, file.
2. Both parties sign (tracked flags).
3. Manager **activates** the contract → system atomically updates unit owner (purchase) or resident (rental) and the change is auditable via **UnitTransferHistory**.
4. Manual path also exists: `change_ownership` / `change_residency` methods with history logging.
5. Ad-hoc transfers: parking/warehouse assignment and release to/from units by code, only within the same building.

### S4. Financial Management (Manager)
1. Define **charge templates** per building (e.g. monthly charge, elevator, cleaning).
2. Each period, **generate charges** in bulk from templates for all units.
3. Track payments: record transactions, confirm payments (auto-updates charge paid/remaining/status).
4. Monitor **invoices** per unit/period; run **financial reports** per building (collections, outstanding).
5. Overdue handling: charges flip to overdue; notification (type `charge`) can be sent to defaulters.

### S5. Facility Management (Manager / Staff)
1. Define facilities with capacity, opening hours, pricing, rules, images.
2. Residents book; manager **approves/rejects**; booking becomes **active** at start time and **completed** after.
3. Schedule **maintenance** windows (may block bookings via `affect_bookings`), staff start/complete them.
4. Facility under maintenance is deactivated for booking.

### S6. Operations & Staff Management (Manager)
1. Build **teams** (security, cleaning, maintenance…) with supervisor and staff members.
2. Incoming **service requests** are reviewed and **assigned to a team**, which creates a **task**.
3. Tasks are assigned to staff, started, completed; hours (estimated vs actual) recorded; comments & attachments on tasks.
4. Monthly **team performance** reviewed: completion rate, overdue count, average completion time, satisfaction.

### S7. Governance & Communication (Board / Manager)
1. Call **meetings** (board/general/committee/emergency) with agenda; targeted roles/teams are invited.
2. Track RSVPs and attendance; record **minutes** (decisions, action items) and collect signatories.
3. Publish **announcements** with priority and expiry, targeted to units/roles/teams.
4. Send **notifications** immediately or scheduled, by audience segment.
5. Handle resident lifecycle events: water outage → urgent notification; annual budget → general meeting + minutes + charge template update.

---

## 5. Issues & Gaps Found in the Current System (redesign input)

1. **Duplicated `Warehouse` model** defined twice in `complexes/models.py` (dead code block).
2. **Vestigial `charging` app** — empty, overlapping with `financial`.
3. **Duplicate `Team` model** — one in `operations` (per building) and one in `communications` (global) — inconsistent concepts.
4. **Contracts endpoints commented out** — contract lifecycle exists in model but is not exposed in API.
5. **Role checks ad-hoc** in views (`user_type in [...]`), no central authorization/permission framework; no staff scoping; `customer` role unused.
6. **Building-scoped feature flags exist but are unenforced** — features toggle UI at best; API doesn't respect them.
7. **Money as DecimalField(max_digits=12)** — okay, but no currency concept; Rial/Toman ambiguity.
8. **Persian (Jalali) calendar reality** — periods are stored `YYYY-MM` Gregorian while Iranian billing periods are Jalali months; no calendar handling anywhere.
9. **No real payment gateway integration** — transactions are manually confirmed.
10. **No notification delivery channel** — notifications are DB rows only (no push/SMS/email dispatch).
11. **Booking duration floors to whole hours**; overlap checking exists in views but slot logic is simplistic.
12. **Auto-numbering via string parsing** (`CONTRACT-...` / `TX-...`) — race-prone.
13. **No pagination/filter conventions**, no API versioning, mixed response shapes (`{message}` vs objects).
14. **No tests** (test files are stubs), SQLite in use, OpenAPI schema generation broken.
15. **Single owner / single resident per unit** — no co-ownership, no residency history date ranges.

---

## 6. Proposed Redesign Roadmap

**Target stack**: Go microservices backend, REST (OpenAPI-first) + optional gRPC internal, PostgreSQL, Redis, RabbitMQ/Kafka for events, containerized.

**Bounded contexts (microservice candidates)**
1. **Identity & Access** — users, auth, roles, building membership & permissions (RBAC/ABLA per building).
2. **Property** — buildings, units, parkings, warehouses, transfer history, contracts.
3. **Billing** — charge templates, charges, invoices, transactions, payment gateway integration.
4. **Facilities** — facilities, bookings, maintenance, availability calendar.
5. **Operations** — teams, tasks, service requests, SLA & performance.
6. **Communications** — notifications (push/SMS/email fan-out), announcements, meetings, minutes, support tickets.

**Cross-cutting**: API Gateway, event bus (domain events: `ContractActivated`, `ChargeOverdue`, `BookingConfirmed`…), audit log, Jalali calendar support, file storage (S3-compatible), feature flags per building enforced at gateway/service level.

**Phases**
1. **Phase 0 — Design**: finalize this document; design target architecture, domain events, and OpenAPI specs per service.
2. **Phase 1 — Core services**: Identity + Property (auth, buildings, units, transfers, contracts) — the minimum to run a building.
3. **Phase 2 — Money**: Billing service with gateway integration (Iranian PSPs), invoices, dunning/overdue workflow.
4. **Phase 3 — Living**: Facilities booking + Operations (service requests → tasks).
5. **Phase 4 — Engagement**: Communications (multi-channel notifications), meetings, tickets.
6. **Phase 5 — Frontend**: design system + UI/UX (resident app, manager/admin panel, staff mobile view), then implementation.
7. **Phase 6 — Hardening**: tests, CI/CD, observability, migrations from Django data.

---

*Document generated 2026-08-23 from code review of `unital_backend` (branch `backend`).*
