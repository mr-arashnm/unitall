# Unital — API Design (v1)

> OpenAPI-first. All external traffic enters through the gateway at `/api/v1/*`. Full OpenAPI source of truth: `backend/openapi/*.yaml` (identity ships now; others follow the same conventions). This document fixes the conventions and the complete endpoint catalog.

## 1. Conventions

- **Base**: `https://<host>/api/v1` — resource collections are plural nouns, kebab-case (`charge-templates`).
- **Headers in from gateway** (set after JWT validation, never accepted from outside): `X-User-Id`, `X-User-Platform-Role`, `X-Trace-Id`. Building role is derived per-request from membership.
- **Auth**: `Authorization: Bearer <access-jwt>`; refresh via cookie or body. 401 vs 403: 401 = no/bad token, 403 = authenticated but not allowed.
- **Pagination**: cursor-based. `?limit=50&cursor=…` → `{ "data": [...], "page": { "next_cursor": "…", "has_more": true } }`. Default limit 50, max 200.
- **Filtering**: exact filters as query params (`?status=pending&building_id=…`); ranges as `?due_from=&due_to=`; full-text as `?q=`.
- **Sorting**: `?sort=-due_date,unit_number` (leading `-` = desc).
- **Idempotency**: mutating POSTs accept `Idempotency-Key`; billing endpoints require it (409 on mismatched replay, 200 on exact replay).
- **Errors**: RFC 9457 Problem Details:
  ```json
  { "type": "https://api.unital.app/errors/unit-not-in-building",
    "title": "Unit is in a different building", "status": 422,
    "code": "UNIT_NOT_IN_BUILDING", "detail": "Parking P-12 belongs to building B-1.",
    "trace_id": "01J…" }
  ```
- **Money**: integer minor units (`"amount": 25000000` = Rial), `"currency": "IRR"`.
- **Dates**: ISO-8601 (`2026-08-23`, `2026-08-23T14:30:00Z`); billing periods are Jalali strings (`1405-06`).
- **IDs**: UUIDv7 (sortable) as strings.
- **State changes that aren't plain CRUD** use `POST /{resource}/{id}/<verb>` actions (`/activate`, `/approve`, `/pay`) — verbs are forbidden in collection paths.
- **Versioning**: URL major (`/v1`); additive changes only within v1; deprecation via `Deprecation` + `Sunset` headers.

## 2. Endpoint Catalog

### Identity (`/api/v1` → service :9001)
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /auth/register` | Create account (sends verification) | public |
| `POST /auth/login` | Email+password → access + refresh | public |
| `POST /auth/refresh` | Rotate refresh → new pair | public |
| `POST /auth/logout` | Revoke refresh token | any |
| `POST /auth/verify` / `POST /auth/password-reset` / `POST /auth/password-reset/confirm` | Account recovery | public |
| `GET /me` / `PATCH /me` | Profile (name, phone, national code, avatar) | any |
| `GET /me/buildings` | My buildings + role in each | any |
| `GET /users` `GET /users/{id}` | Directory (scoped to requester's buildings) | manager, board |
| `POST /buildings/{id}/memberships` · `GET …/memberships` · `DELETE …/memberships/{mid}` | Grant/review/revoke building roles | manager |
| `GET /internal/users/{id}` | Service-to-service (gateway-restricted) | internal |

### Property (`/api/v1` → :9002)
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /buildings` · `GET /buildings` · `GET/PATCH /buildings/{id}` | Building CRUD; features on PATCH | manager, board |
| `POST /buildings/{id}/units` · `GET /buildings/{id}/units` · `GET/PATCH/DELETE /units/{uid}` | Unit inventory | manager, board (read: owner/resident of unit) |
| `GET /units/{id}/parties` | Ownership/residency timeline | manager, board, unit parties |
| `POST /units/{id}/ownership-changes` · `POST /units/{id}/residency-changes` | Record transfer (validity-ranged) | manager |
| `GET /units/{id}/transfer-history` | Audit projection | manager, board |
| `POST /buildings/{id}/parkings` · `POST /buildings/{id}/warehouses` · CRUD `/parkings/{id}`, `/warehouses/{id}` | Asset inventory | manager, board |
| `POST /units/{id}/assets` `DELETE /units/{id}/assets/{aid}` | Assign/release parking or warehouse (same-building rule enforced) | manager |
| `GET /buildings/{id}/assets?status=available` | Availability | manager, board, staff |
| `POST /contracts` · `GET /contracts` · `GET/PATCH /contracts/{id}` | Contracts (draft, parties, file) | manager, board; parties read |
| `POST /contracts/{id}/sign` (body: party) · `POST /contracts/{id}/activate` | Signature + activation (side effects + event) | sign party / manager |

### Billing (`/api/v1` → :9003)
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /buildings/{id}/charge-templates` · CRUD `/charge-templates/{id}` | Define recurring charges | manager, board |
| `POST /buildings/{id}/charges:generate` (`{period:"1405-06"}`) | Idempotent bulk generation from templates | manager |
| `GET /buildings/{id}/charges?period=&status=&unit_id=` · `GET /charges/{id}` | Charge queries | manager/board; owner/resident see own units |
| `POST /charges/{id}/payments` `{method, amount, ref?}` | Record payment attempt (creates transaction) | manager, staff(cash), owner/resident (online → PSP) |
| `POST /transactions/{id}/confirm` · `POST /transactions/{id}/fail` | Manual/PSP callback confirm | manager, PSP callback |
| `GET /units/{id}/invoices` · `GET /invoices/{id}` · `GET /buildings/{id}/invoices` | Period aggregates | per-role scoping |
| `GET /buildings/{id}/reports/financial?period=` | Collections, outstanding, aging | manager, board |

### Facilities (`/api/v1` → :9004)
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /buildings/{id}/facilities` · CRUD `/facilities/{id}` (+`usage-rules`, `images`) | Facility setup | manager, board |
| `GET /facilities/{id}/availability?date=&from=&to=` | Free slots given hours & maintenance | any member |
| `POST /facilities/{id}/bookings` `{start,end,purpose,participants}` | Create booking (validates window/capacity/overlap) | owner, resident, staff |
| `GET /bookings?mine=true` · `GET /bookings/{id}` · `POST /bookings/{id}/cancel` | Self-service | booker, manager |
| `POST /bookings/{id}/approve` · `POST /bookings/{id}/reject` | Approval workflow | manager, board |
| `POST /facilities/{id}/maintenance-windows` · `POST /maintenance-windows/{id}/start` `…/complete` | Maintenance (blocks bookings) | manager, staff |

### Operations (`/api/v1` → :9005)
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /buildings/{id}/teams` · CRUD `/teams/{id}` (+`POST /teams/{id}/members`) | Teams | manager, board |
| `POST /teams/{id}/tasks` · `GET /tasks?team_id=&assignee_id=&status=` · `GET/PATCH /tasks/{id}` | Task management | manager, board |
| `POST /tasks/{id}/assign` · `…/start` · `…/complete` · `…/hold` | Lifecycle | manager, assignee |
| `POST /tasks/{id}/comments` · `GET /tasks/{id}/comments` | Discussion + attachments | team, manager |
| `POST /units/{id}/service-requests` | Resident intake | owner, resident |
| `GET /service-requests?status=&building_id=` · `POST /service-requests/{id}/assign-to-team` | Triage (auto-creates task) | manager, board |
| `GET /teams/{id}/performance?period=` | KPI snapshot | manager, board |

### Communications (`/api/v1` → :9006)
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /buildings/{id}/announcements` · `GET /announcements?building_id=` · `POST /announcements/{id}/publish` | Announcements (target: units/roles/teams) | manager, board |
| `POST /buildings/{id}/meetings` · `POST /meetings/{id}/rsvp` · `POST /meetings/{id}/minutes` · `POST /meetings/{id}/status` | Meetings, RSVP, minutes | manager/board create; any invited RSVP |
| `GET /meetings/{id}/attendance` | Attendance sheet | manager, board |
| `POST /units/{id}/tickets` · `GET /tickets?mine=true` · `POST /tickets/{id}/responses` · `POST /tickets/{id}/resolve` · `…/close` | Support tickets | any; resolve/close: manager/staff |

### Notifications (`/api/v1` → :9006, implemented — see [NOTIFICATION_DESIGN.md](NOTIFICATION_DESIGN.md))
| Method & path | Purpose | Roles |
|---|---|---|
| `POST /notifications:send` `{template, building_id, target{kind,values}, vars, channels?, schedule_at?}` | Create campaign (fan-out to recipients × channels) | manager, board |
| `GET /notifications/{id}` · `GET /notifications/{id}/deliveries?status=` | Campaign status + delivery inspection | manager, board |
| `POST /templates` · `GET /templates` | Template registry (per-channel variants) | manager, board |
| `GET /me/notifications?unread=true` · `POST /me/notifications/{id}/read` | Personal inbox | any |
| `POST /internal/trigger` `{type, building_id, data}` | Feed domain event through binding (NATS later) | internal |
| `POST /internal/dispatch` | Kick the outbox worker (cron/tick in prod) | internal |

## 3. Cross-Service Internal Contracts

Internal endpoints (network-restricted, no user JWT): `GET /internal/users/{id}`, `GET /internal/buildings/{id}/members?role=owner`, `GET /internal/units/{id}/parties/current`. Cached 60s in Redis. Event payloads carry denormalized display fields (`full_name`) to avoid hot lookups.

## 4. Webhooks & PSP

`POST /api/v1/callbacks/psp/{provider}` — signature-verified, maps to `transactions/{id}/confirm` internally. Providers behind `PaymentProvider` interface (`zarinpal` first, `manual` for cash/transfer).
