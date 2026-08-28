# Unital Frontend — Resident App (React)

Modern redesign of the resident app: **React 19 + TypeScript + Vite 8 + React Router 7**,
implementing the design system from `../docs/UIUX_DESIGN.md` (flat teal, RTL Persian,
Vazirmatn, mobile-first with bottom nav).

## Run

Requires Node ≥ 22 (this machine: use `~/.local/opt/node-v22.23.2-linux-x64/bin`,
already on PATH via `~/.bashrc` — system node is broken due to GLIBC).

    npm install
    npm run dev       # http://localhost:5173 (HMR)
    npm run build     # production bundle → dist/ (~80 KB gzip JS)

## Structure

```
src/
  api.ts              # gateway client + demo fallback + Jalali-friendly types
  styles.css          # design tokens + all component styles (single source)
  components/
    icons.tsx         # Lucide line icons as inline React components
    ui.tsx            # StatusPill, SectionTitle, Empty
    BottomNav.tsx     # NavLink bottom navigation with active indicator
  screens/
    Login.tsx         # auth with graceful demo-mode fallback
    Home.tsx          # gradient balance hero, quick tiles, announcements
    Charges.tsx       # per-period grouping, partial-pay progress, pay + toast
    Requests.tsx      # service request wizard + my requests list
    Inbox.tsx         # notifications, meeting RSVP
    Unit.tsx          # unit info, parking/warehouse, transfer timeline
```

The app talks to the Go gateway at `http://localhost:8080/api/v1`
(override via `localStorage.unital_api`). When the backend is
unreachable it enters demo mode (chip in the app bar) with realistic
data so the UI stays reviewable.
