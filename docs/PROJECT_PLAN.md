# Unital Redesign — Project Plan

> Execution plan derived from [BUSINESS_DOCUMENT.md](BUSINESS_DOCUMENT.md). Target: Go microservices backend, OpenAPI-first API, modern frontend. Each phase has a definition of done (DoD) and demo scenario.

## Guiding Principles

1. **OpenAPI-first** — every service's contract is versioned before code is written.
2. **Database per service** — no shared tables; integration via events + explicit APIs.
3. **Runnable at every milestone** — `docker compose up` must always give a working slice.
4. **Migrate data last** — Django system stays source of truth until cutover phase.
5. **Vertical slices** — each phase ships one end-to-end capability, not one horizontal layer.

## Phase Overview

| Phase | Deliverable | Depends on |
|---|---|---|
| P0 | Design docs (this + ARCHITECTURE + API_DESIGN + UIUX_DESIGN) | Business doc |
| P1 | Identity service + API Gateway skeleton, JWT auth end-to-end | P0 |
| P2 | Property service (buildings, units, assets, transfers, contracts) | P1 |
| P3 | Billing service (templates, charges, invoices, payments, gateway stub) | P2 |
| P4 | Facilities + Operations services (bookings, requests→tasks workflow) | P2 |
| P5 | Communications service (notifications fan-out, announcements, meetings, tickets) | P2 |
| P6 | Frontend (resident app + management console) | P1–P5 APIs |
| P7 | Hardening: observability, CI/CD, load tests, data migration from Django | all |

## Phase Detail

### P0 — Design (now)
- Business document, architecture (services, events, data ownership), API conventions + specs, UI/UX system.
- **DoD**: docs reviewed; service boundaries and event catalog frozen for P1–P3.

### P1 — Identity & Access (week 1)
- Go monorepo scaffold: `pkg/` shared libs (config, httpx, jwtx, events, logger).
- Identity service: register/login/refresh/logout, profile, roles (`manager|board_member|staff|owner|resident`), building memberships & RBAC (` memberships` with roles per building), email verify + password reset (stub mailer).
- Gateway: reverse proxy, JWT validation, per-route auth.
- **DoD**: `docker compose up` → register user, login, call protected echo endpoint. Unit tests green.

### P2 — Property (week 2)
- Buildings, units, parkings, warehouses; assignment/release with same-building rule; ownership/residency change with **transfer history**; contracts with activation side-effects (emits `contract.activated`).
- Feature flags per building enforced on gateway routes.
- **DoD**: create building → units → draft & activate purchase contract → owner changes + history row + event published.

### P3 — Billing (week 3)
- Charge templates; period generation (Jalali-aware period keys); charges with partial payments; invoices aggregation; transactions with PSP adapter interface (sandbox stub + manual confirm); overdue sweeper emits `charge.overdue`.
- Consumes `contract.activated` (rental → first invoice), `unit.ownership_changed`.
- **DoD**: generate month charges for a building, pay one partially, invoice reflects totals, report endpoint returns collections/outstanding.

### P4 — Facilities & Operations (week 4)
- Facilities: definitions, availability calendar, bookings with approval workflow, maintenance windows blocking bookings, pricing by hour.
- Operations: teams, tasks lifecycle, service requests → auto-task, team performance projections.
- Consumes `user.created` (staff onboarding), `facility.maintenance_scheduled`.
- **DoD**: resident books party hall → manager approves → calendar blocks overlap; resident submits request → team task created → completed.

### P5 — Communications (week 5)
- Announcements, meetings (RSVP, minutes, signatures), support tickets with threaded responses, notification templates.
- Delivery channels: in-app inbox (default), email/SMS adapters behind interface; targeting by role/unit/team resolved via Identity & Property APIs.
- Consumes domain events from all services (charge.overdue → reminder campaign).
- **DoD**: send targeted announcement to owners of building X; defector gets overdue reminder; ticket thread works.

### P6 — Frontend (weeks 6–7)
- Design system per UIUX_DESIGN.md; resident web app (PWA-ready) + management console (admin).
- Screens: login/register, my units & charges (pay), facility booking, service requests, announcements/meetings, tickets; console: buildings, units/occupancy, billing run, approvals, teams/tasks, meetings, announcements composer.
- **DoD**: full resident journey (pay a charge, book a facility) and manager journey (charge run, approve booking) against live backend.

### P7 — Hardening & Migration (week 8)
- CI (lint, test, build), OTLP tracing/metrics, structured logs, load test (k6), backup strategy.
- ETL from Django SQLite/Postgres → service DBs with idempotent scripts + reconciliation report.
- **DoD**: production compose stack passes smoke suite; migration verified by row counts + spot checks.

## Repo Layout (target)

```
unital/
  docs/                     # design documents
  backend/
    pkg/                    # shared Go libs (config, httpx, jwtx, events, money, jalali)
    services/
      identity/             # auth, users, memberships, RBAC
      property/             # buildings, units, assets, transfers, contracts
      billing/              # templates, charges, invoices, payments
      facilities/           # facilities, bookings, maintenance
      operations/           # teams, tasks, service requests
      communications/       # announcements, meetings, tickets, notifications
    gateway/                # edge router, authN/Z, feature flags
    deploy/                 # docker-compose, migrations, makefiles
  frontend/
    apps/resident/          # resident PWA
    apps/console/           # management console
    packages/design-system/ # shared UI kit
```

## Risk Register

| Risk | Mitigation |
|---|---|
| Calendar mismatch (Jalali vs Gregorian) | Single `pkg/jalali` used everywhere; period keys stored as Jalali `YYYY-MM` + ISO date fields |
| Payment provider integration delays | PSP behind interface; manual confirmation path ships first |
| Over-engineering microservices for small scale | Start with coarse services (6), co-deployable in one compose; split later only on proven need |
| Frontend scope explosion | Two apps max; one design system; screens frozen per P6 list |
| Data migration integrity | Idempotent ETL, reconciliation reports, dual-run window |
