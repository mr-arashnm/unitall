// Announcements — building-wide notices.
import { useEffect, useState } from "react";
import { fetchAnnouncements, session } from "../api";
import type { Announcement } from "../api";
import { BackBtn, Empty } from "../components/ui";
import { IconMegaphone, IconCalendar } from "../components/icons";

const PRIORITY_STYLE: Record<string, { bg: string; color: string; border: string }> = {
  urgent: { bg: "var(--c-destructive-soft)", color: "var(--c-destructive)", border: "1px solid var(--c-destructive-soft)" },
  high: { bg: "var(--c-accent-soft)", color: "var(--c-pill-accent-fg)", border: "1px solid var(--c-warning-soft)" },
  normal: { bg: "var(--c-surface-alt)", color: "var(--c-fg-2)", border: "1px solid var(--c-border)" },
  low: { bg: "transparent", color: "var(--c-fg-3)", border: "1px solid var(--c-border)" },
};

const PRIORITY_LABEL: Record<string, string> = {
  urgent: "Urgent", high: "Important", normal: "Normal", low: "Low",
};

export default function Announcements() {
  const [items, setItems] = useState<Announcement[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const bid = session.currentBuilding;
    if (bid) fetchAnnouncements(bid).then((r) => { setItems(r); setLoading(false); });
    else setLoading(false);
  }, []);

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Announcements</h1>
        <div style={{ width: 38 }} />
      </div>

      {loading && <div className="card"><div className="skeleton" style={{ height: 80 }} /></div>}
      {!loading && items.length === 0 && <Empty text="No announcements" icon={<IconMegaphone />} />}

      <div className="stack">
        {items.map((a) => {
          const ps = PRIORITY_STYLE[a.priority] ?? PRIORITY_STYLE.normal;
          return (
            <div
              key={a.id}
              className="card"
              style={{ background: ps.bg, border: ps.border, padding: "14px 16px" }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "start", gap: 10, marginBottom: 6 }}>
                <div style={{ fontWeight: 700, fontSize: 15.5, color: ps.color }}>{a.title}</div>
                <span className="badge" style={{ background: "rgba(0,0,0,0.06)", color: ps.color, flexShrink: 0 }}>
                  {PRIORITY_LABEL[a.priority]}
                </span>
              </div>
              {(a.body || a.content) && (
                <div style={{ fontSize: 14, lineHeight: 1.7, color: "var(--c-fg)" }}>{(a.body ?? a.content ?? "").split("\n").map((line, i) => (
                  <span key={i}>{line}{i < (a.body ?? a.content ?? "").split("\n").length - 1 && <br />}</span>
                ))}</div>
              )}
              {a.date && (
                <div className="small" style={{ color: "var(--c-fg-3)", marginTop: 8, display: "flex", gap: 4, alignItems: "center" }}>
                  <IconCalendar size={13} /> {new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long" }).format(new Date(a.date))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
