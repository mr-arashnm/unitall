# Unital — UI/UX Design

> Design foundation for the resident app and the management console.
> Direction generated with the ui-ux-pro-max design-system tool ("property
> management resident dashboard") and adapted: the suggested scholarly
> serif pairing was rejected in favor of Persian-first utility typography.

## 1. Products & Personas

| App | Primary users | Core jobs |
|---|---|---|
| **Resident** (PWA, mobile-first, RTL Persian) | Owner سارا (45, checks charges monthly), Tenant علی (29, books gym, files requests on the go) | Pay charges, book facilities, track requests, read announcements, RSVP meetings, tickets |
| **Console** (web, desktop-first) | Manager مریم (runs 3 buildings), Board member حسین (approves budgets), Staff رضا (sees assigned tasks on phone) | Occupancy & contracts, billing runs & approvals, teams/tasks, meetings & announcements |

**Design principles**: 1) every resident action ≤3 taps from home;
2) money states never ambiguous (paid/partial/overdue color + label + icon);
3) RTL-first — Persian is the default locale, English is secondary, no
mirrored-icon mistakes (media/progress stay LTR); 4) trust — audit
history visible (transfers, payments) with timestamps.

## 2. Design System

**Style**: Flat design — no gradients/shadows, clean lines, typography-led,
fast transitions (150–200ms). Accessibility: contrast ≥4.5:1, visible focus
rings, `prefers-reduced-motion` respected, 44×44px touch targets.

**Color tokens** (light; dark mode = inverted surfaces, same hues):
| Token | Value | Use |
|---|---|---|
| `--color-primary` | `#0D9488` (teal-600) | Primary actions, active nav |
| `--color-secondary` | `#2DD4BF` | Highlights, selected chips |
| `--color-accent` | `#D97706` (amber-600) | Due-soon, warnings, pending |
| `--color-destructive` | `#DC2626` | Overdue, destructive |
| `--color-success` | `#16A34A` | Paid, confirmed, completed |
| `--color-background` | `#F0FDFA` | Resident app canvas |
| `--color-card` | `#FFFFFF` | Surfaces |
| `--color-muted-fg` | `#475569` | Secondary text (4.5:1 on card) |
| `--color-border` | `#CBd5e1` (neutral-300) | Hairlines (border token lightened from tool's #5EEAD4 for neutral UI chrome) |

Money/status semantics: paid=success, partially_paid=accent, overdue=destructive,
pending=neutral. Never color alone — always label + icon (WCAG 1.4.1).

**Typography**: `Vazirmatn` (Persian+Latin, variable) everywhere;
weights 400/500/700. Scale: 14 (secondary) / 16 base / 20 section /
24 page title; line-height 1.6 for Persian. Numerals: Persian digits in
copy, Latin digits in inputs/charts for accuracy.

**Spacing/radius**: 4/8/12/16/24/32 scale; radius 12 cards, 8 inputs,
full for chips. **Icons**: Lucide, 20/24px, stroke 1.75. **Motion**:
150–200ms ease-out for state changes; list staggers ≤50ms items;
no parallax.

**Density dials**: resident app = comfortable (16–64px scale); console
dashboard = dense (8–32px scale, compact tables 40px rows).

## 3. Resident App — Screens & Flows

Navigation: bottom bar ≤5 — **خانه (Home) · شارژ (Charges) · رزرو (Book) ·
درخواست‌ها (Requests) · پیام‌ها (Inbox)**; profile under avatar.

1. **Home** — building switcher (if multiple), balance card (outstanding
   total + nearest due date + CTA «پرداخت»), quick actions grid (Pay /
   Book facility / New request / Tickets), latest announcements (3),
   upcoming bookings & meetings.
2. **Charges** — period tabs (Jalali months), charge list rows
   (template name, amount, status pill, due), invoice card per period
   (total/paid/remaining progress bar), pay flow → method sheet
   (online gateway / bank receipt upload) → success state with receipt ref.
3. **Book (facilities)** — facility cards (image, hours, price/free,
   status open/under maintenance) → date picker (Jalali) → time-slot grid
   (disabled slots crossed) → request form (purpose, participants) →
   pending state; approvals notified via Inbox.
4. **Requests** — new request wizard: type (maintenance/cleaning/security/
   complaint) → description + photos → submit; list with status timeline
   (submitted → assigned → in-progress → completed) and team comments.
5. **Inbox** — notifications (targeted announcements, charge reminders,
   booking updates), meetings with RSVP buttons (going/declined), minutes
   attached after completion; tickets thread view with attachments.
6. **My unit** — unit info, parking/warehouse assignments, transfer
   history (owner/resident timeline), contracts list with status.

## 4. Console — Screens

Left sidebar (collapsible): **داشبورد · ساختمان‌ها · مالی · درخواست‌ها ·
تیم‌ها و وظایف · امکانات · جلسات · اطلاع‌رسانی · پشتیبانی**.

1. **Dashboard** — KPI cards (collection rate %, outstanding Rial, open
   requests, overdue count), collections chart by period, building
   switcher, today's approvals queue.
2. **Buildings** — list → detail: units table (occupancy, owner/resident,
   area, assets), asset manager (assign/release parking & warehouse via
   dropdown of available codes), transfer history per unit.
3. **Contracts** — draft list with signature status, activate action with
   confirm dialog showing side-effect summary («مالکیت واحد ۳۰۱ به علی
   منتقل می‌شود»), archive.
4. **Finance** — templates CRUD; period generation wizard (period
   picker → preview unit×template counts → run, idempotent badge);
   transactions to confirm (cash/transfer evidence); financial report
   (billed/collected/outstanding/aging) with export CSV.
5. **Requests & Tasks** — triage board (kanban by status), assign to
   team (auto-creates task), task detail with comments & hours;
   team performance view (completion rate, overdue, satisfaction).
6. **Facilities** — definitions with images/rules/hours, bookings
   approval queue, maintenance calendar.
7. **Meetings & Announcements** — meeting composer (type, agenda,
   targeted roles), attendance sheet export, minutes editor with
   signatories; announcement composer with audience picker
   (all/owners/residents/units/teams), schedule or publish, expiry.
8. **Support** — ticket inbox with SLA badges, threaded replies,
   internal notes toggle.

## 5. Key Flows (must-feel-good journeys)

- **Pay a charge** (resident): Home → balance card → pay → method →
  gateway → success receipt. Optimistic pending state; webhook flips
  to paid with confetti-free, calm success card.
- **Sell a unit** (manager): Contracts → new purchase contract
  (autofill unit parties) → both parties e-sign → activate (confirm
  dialog) → ownership changes + history row + invoice target switches.
- **Charge run** (manager): Finance → generate 1405-06 → preview
  "۲۴ واحد × ۳ قالب = ۷۲ شارژ" → run → rerun shows "۰ جدید" (idempotent).
- **Book the party hall** (resident): Book → hall → Thursday slots →
  purpose + 30 guests → manager approves → calendar invite + reminder.

## 6. States & Feedback

Empty states with one-line explanation + primary action; skeletons on
first load (never spinners >300ms without skeleton); offline banner in
PWA; optimistic submit with undo for cancellable actions; destructive
actions require typed confirmation only for contracts activation and
membership revoke.

## 7. Accessibility Checklist (pre-delivery)

- [ ] Contrast ≥4.5:1 all text (muted-fg on card verified)
- [ ] Focus visible (2px ring, offset 2px) — never removed
- [ ] Touch targets ≥44×44; 8px minimum spacing between targets
- [ ] RTL layout mirrored correctly; `dir="rtl"`; charts LTR with Persian labels
- [ ] Status never color-only (label + icon)
- [ ] Forms: visible labels, inline errors near fields, error summary + focus management
- [ ] `prefers-reduced-motion` disables staggers/transitions
- [ ] Screen-reader labels on icon-only buttons (avatar, language, back)
