// Dashboard — hero balance card, quick action grid, recent activity, stats.
import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { fetchCharges, fmtRial, session } from "../api";
import type { Charge } from "../api";
import { SectionTitle, StatCard, StatusPill, PriorityBadge } from "../components/ui";
import {
  IconBell, IconCalendar, IconCreditCard, IconHome, IconWrench,
  IconCheck, IconBuildings, IconUsers, IconChart, IconMegaphone, IconClock,
} from "../components/icons";
import BuildingSwitcher from "../components/BuildingSwitcher";

const RECENT_ACTIVITIES = [
  { id: 1, kind: "announcement", title: "Water outage in the building", sub: "Friday, 9 AM – 1 PM", time: "2 hours ago", priority: "high" as const },
  { id: 2, kind: "booking", title: "Hall booking confirmed", sub: "Friday at 5 PM", time: "Yesterday" },
  { id: 3, kind: "ticket", title: "Support ticket closed", sub: "Parking camera installed", time: "3 days ago" },
  { id: 4, kind: "meeting", title: "Board meeting", sub: "Monday, 6:00 PM", time: "Next week" },
];

export default function Home() {
  const nav = useNavigate();
  const [charges, setCharges] = useState<Charge[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!session.user) { nav("/login", { replace: true }); return; }
    fetchCharges()
      .then((r) => setCharges(r.data))
      .catch((err: unknown) => setError(err instanceof Error ? err.message : "Could not load charges"));
  }, [nav]);

  const outstanding = useMemo(() => charges.reduce((s, c) => s + c.remaining, 0), [charges]);
  const overdue = charges.some((c) => c.status === "overdue");
  const nearest = useMemo(
    () => charges.filter((c) => c.remaining > 0).sort((a, b) => a.due_date.localeCompare(b.due_date))[0],
    [charges],
  );
  const user = session.user;

  return (
    <div className="view">
      {error && (
        <div className="banner banner--info" style={{ marginBottom: 12 }}>
          <IconCalendar />
          <span>{error}</span>
        </div>
      )}

      <div className="row-between" style={{ marginBottom: 16 }}>
        <div>
          <div className="eyebrow">Welcome back, {user?.full_name?.split(" ")[0] ?? "Guest"}</div>
          <h1 className="page-title" style={{ margin: "4px 0 0" }}>Home</h1>
        </div>
        <BuildingSwitcher />
      </div>

      <div className="hero">
        <div className="hero__label"><IconCreditCard size={16} /> Outstanding balance</div>
        <div className="hero__amount">{fmtRial(outstanding)}</div>
        <div className="hero__meta">
          <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
            <IconCalendar size={15} />
            {nearest ? `Next due: ${new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long" }).format(new Date(nearest.due_date))}` : "No charges yet"}
          </span>
          <span className="pill">
            {overdue ? "You have overdue charges" : "All clear"}
          </span>
        </div>
        <Link to="/charges" className="btn btn-primary btn-block" style={{ textDecoration: "none" }}>
          <IconCreditCard size={17} /> Pay charges
        </Link>
      </div>

      <div className="grid-2" style={{ marginTop: 16 }}>
        <StatCard label="Paid" value={fmtRial(charges.reduce((s, c) => s + c.paid, 0))} sub="This period" />
        <StatCard label="Outstanding" value={fmtRial(outstanding)} sub={`${charges.filter(c => c.remaining > 0).length} items`} accent={overdue} />
      </div>

      <SectionTitle>Quick access</SectionTitle>
      <div className="quick-grid">
        <button className="quick-tile" onClick={() => nav("/buildings")}>
          <span className="quick-tile__icon"><IconBuildings /></span>
          <span>Buildings</span>
        </button>
        <button className="quick-tile" onClick={() => nav("/charges")}>
          <span className="quick-tile__icon"><IconCreditCard /></span>
          <span>Charges</span>
        </button>
        <button className="quick-tile" onClick={() => nav("/requests")}>
          <span className="quick-tile__icon"><IconWrench /></span>
          <span>Requests</span>
        </button>
        <button className="quick-tile" onClick={() => nav("/inbox")}>
          <span className="quick-tile__icon"><IconBell /></span>
          <span>Inbox</span>
        </button>
      </div>

      <div className="quick-grid" style={{ marginTop: 8 }}>
        <button className="quick-tile" onClick={() => nav("/facilities")}>
          <span className="quick-tile__icon" style={{ background: "var(--c-info-soft)", color: "var(--c-info)" }}><IconHome /></span>
          <span>Facilities</span>
        </button>
        <button className="quick-tile" onClick={() => nav("/teams")}>
          <span className="quick-tile__icon" style={{ background: "var(--c-warning-soft)", color: "var(--c-warning)" }}><IconUsers /></span>
          <span>Teams</span>
        </button>
        <button className="quick-tile" onClick={() => nav("/tickets")}>
          <span className="quick-tile__icon" style={{ background: "var(--c-accent-soft)", color: "var(--c-pill-accent-fg)" }}><IconWrench /></span>
          <span>Tickets</span>
        </button>
        <button className="quick-tile" onClick={() => nav("/reports")}>
          <span className="quick-tile__icon" style={{ background: "var(--c-success-soft)", color: "var(--c-success)" }}><IconChart /></span>
          <span>Reports</span>
        </button>
      </div>

      <SectionTitle>Recent activity</SectionTitle>
      <div className="stack">
        {RECENT_ACTIVITIES.map((a) => (
          <div className="row" key={a.id}>
            <span className="row__icon" style={{
              background: a.kind === "announcement" ? "var(--c-accent-soft)" :
                          a.kind === "booking" ? "var(--c-info-soft)" :
                          a.kind === "ticket" ? "var(--c-primary-soft)" : "var(--c-warning-soft)",
              color: a.kind === "announcement" ? "var(--c-pill-accent-fg)" :
                     a.kind === "booking" ? "var(--c-info)" :
                     a.kind === "ticket" ? "var(--c-primary-strong)" : "var(--c-warning)",
            }}>
              {a.kind === "announcement" ? <IconMegaphone /> :
               a.kind === "booking" ? <IconCalendar /> :
               a.kind === "ticket" ? <IconWrench /> : <IconUsers />}
            </span>
            <div className="row__body">
              <div className="row__title">{a.title}</div>
              <div className="row__sub">{a.sub}</div>
            </div>
            <div className="row__end">
              {"priority" in a && a.priority ? <PriorityBadge priority={a.priority} /> : null}
              <span className="tiny muted" style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                <IconClock size={11} /> {a.time}
              </span>
            </div>
          </div>
        ))}
      </div>

      {charges.filter((c) => c.remaining > 0).length > 0 && (
        <>
          <SectionTitle>Pending payments</SectionTitle>
          <div className="stack">
            {charges.filter((c) => c.remaining > 0).map((c) => (
              <div className="row" key={c.id}>
                <span className="row__icon row__icon--primary"><IconCreditCard /></span>
                <div className="row__body">
                  <div className="row__title">{c.template_name || "Charge"}</div>
                  <div className="row__sub">Period {c.period}</div>
                  <div className="progress" style={{ maxWidth: 160 }}>
                    <div className="progress__fill" style={{ width: `${(c.paid / c.amount) * 100}%` }} />
                  </div>
                </div>
                <div className="row__end">
                  <span className="row__amount">{fmtRial(c.remaining)}</span>
                  <StatusPill status={c.status} />
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      <div style={{ textAlign: "center", marginTop: 32 }}>
        <span className="small secondary" style={{ display: "inline-flex", gap: 6, alignItems: "center" }}>
          <IconCheck size={14} /> All payments include an official receipt
        </span>
      </div>
    </div>
  );
}
