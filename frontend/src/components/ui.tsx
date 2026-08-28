// Shared UI primitives. Follows the design system (styles.css).
import type { ReactNode } from "react";
import { useEffect, useRef } from "react";
import type { ChargeStatus } from "../api";
import { IconAlert, IconCheck, IconClock, IconX } from "./icons";

// ─── Status pill ──────────────────────────────────────────────
const CHARGE_STATUS: Record<ChargeStatus, { label: string; tone: string; icon: typeof IconCheck }> = {
  paid: { label: "Paid", tone: "pill--success", icon: IconCheck },
  partially_paid: { label: "Partially paid", tone: "pill--accent", icon: IconClock },
  pending: { label: "Pending", tone: "pill--neutral", icon: IconClock },
  overdue: { label: "Overdue", tone: "pill--destructive", icon: IconAlert },
  cancelled: { label: "Cancelled", tone: "pill--neutral", icon: IconX },
};
export function StatusPill({ status }: { status: ChargeStatus }) {
  const s = CHARGE_STATUS[status] ?? CHARGE_STATUS.pending;
  return (
    <span className={`pill ${s.tone}`}>
      <s.icon size={12} />
      {s.label}
    </span>
  );
}

// ─── Priority badge ────────────────────────────────────────────
type Priority = "low" | "medium" | "high" | "urgent";
const PRIORITY: Record<Priority, { label: string; tone: string }> = {
  low: { label: "Low", tone: "badge--muted" },
  medium: { label: "Medium", tone: "badge--accent" },
  high: { label: "High", tone: "badge--warning" },
  urgent: { label: "Urgent", tone: "badge--destructive" },
};
export function PriorityBadge({ priority }: { priority: Priority }) {
  const p = PRIORITY[priority] ?? PRIORITY.low;
  return <span className={`badge ${p.tone}`}>{p.label}</span>;
}

// ─── Task/Service status badge ─────────────────────────────────
type TaskStatus = "pending" | "assigned" | "in_progress" | "completed" | "cancelled" | "on_hold" |
  "submitted" | "under_review" | "resolved" | "closed";
const TASK_STATUS: Record<TaskStatus, { label: string; tone: string }> = {
  pending: { label: "Pending", tone: "badge--muted" },
  assigned: { label: "Assigned", tone: "badge--accent" },
  submitted: { label: "Submitted", tone: "badge--accent" },
  under_review: { label: "Under review", tone: "badge--accent" },
  in_progress: { label: "In progress", tone: "badge--info" },
  resolved: { label: "Resolved", tone: "badge--success" },
  completed: { label: "Completed", tone: "badge--success" },
  cancelled: { label: "Cancelled", tone: "badge--muted" },
  on_hold: { label: "On hold", tone: "badge--warning" },
  closed: { label: "Closed", tone: "badge--muted" },
};
export function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const s = TASK_STATUS[status] ?? TASK_STATUS.pending;
  return <span className={`badge ${s.tone}`}>{s.label}</span>;
}

// ─── Section title ────────────────────────────────────────────
export function SectionTitle({ children }: { children: ReactNode }) {
  return <h2 className="section-title">{children}</h2>;
}

// ─── Empty state ──────────────────────────────────────────────
export function Empty({ text, icon }: { text: string; icon?: ReactNode }) {
  return (
    <div className="empty">
      {icon}
      <span className="small">{text}</span>
    </div>
  );
}

// ─── Toast ────────────────────────────────────────────────────
export function Toast({ msg, onClose }: { msg: string; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const t = setTimeout(onClose, 3500);
    return () => clearTimeout(t);
  }, [onClose]);
  return (
    <div className="toast" ref={ref} role="status" aria-live="polite">
      {msg}
    </div>
  );
}

// ─── Tabs ────────────────────────────────────────────────────
interface Tab { id: string; label: string }
export function Tabs({ tabs, active, onChange }: { tabs: Tab[]; active: string; onChange: (id: string) => void }) {
  return (
    <div className="tabs" role="tablist">
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          aria-selected={active === t.id}
          className={`tabs__tab${active === t.id ? " tabs__tab--active" : ""}`}
          onClick={() => onChange(t.id)}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

// ─── Modal ────────────────────────────────────────────────────
export function Modal({ open, onClose, title, children }: { open: boolean; onClose: () => void; title: string; children: ReactNode }) {
  if (!open) return null;
  return (
    <div className="modal-overlay" onClick={onClose} role="dialog" aria-modal="true" aria-label={title}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal__header">
          <h3 className="modal__title">{title}</h3>
          <button className="icon-btn" onClick={onClose} aria-label="Close"><IconX size={16} /></button>
        </div>
        <div className="modal__body">{children}</div>
      </div>
    </div>
  );
}

// ─── Role badge ───────────────────────────────────────────────
type Role = "manager" | "board_member" | "staff" | "owner" | "resident";
const ROLE: Record<Role, { label: string; tone: string }> = {
  manager: { label: "Manager", tone: "badge--manager" },
  board_member: { label: "Board", tone: "badge--board" },
  staff: { label: "Staff", tone: "badge--staff" },
  owner: { label: "Owner", tone: "badge--owner" },
  resident: { label: "Resident", tone: "badge--resident" },
};
export function RoleBadge({ role }: { role: Role }) {
  const r = ROLE[role] ?? ROLE.resident;
  return <span className={`badge ${r.tone}`}>{r.label}</span>;
}

// ─── Back button ─────────────────────────────────────────────
export function BackBtn({ onClick }: { onClick: () => void }) {
  return (
    <button className="icon-btn back-btn" onClick={onClick} aria-label="Back">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <polyline points="15 18 9 12 15 6" />
      </svg>
    </button>
  );
}

// ─── Stat card ────────────────────────────────────────────────
export function StatCard({ label, value, sub, accent }: { label: string; value: string; sub?: string; accent?: boolean }) {
  return (
    <div className={`stat-card${accent ? " stat-card--accent" : ""}`}>
      <div className="stat-card__label">{label}</div>
      <div className="stat-card__value">{value}</div>
      {sub && <div className="stat-card__sub">{sub}</div>}
    </div>
  );
}

// ─── Progress bar ─────────────────────────────────────────────
export function ProgressBar({ value, max = 100, color }: { value: number; max?: number; color?: string }) {
  const pct = Math.min(100, Math.max(0, (value / max) * 100));
  return (
    <div className="progress">
      <div className="progress__fill" style={{ width: `${pct}%`, ...(color ? { background: color } : {}) }} />
    </div>
  );
}
