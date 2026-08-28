// Inbox — announcements, charge reminders, booking confirmations, meeting RSVPs.
import { useEffect, useState } from "react";
import { fetchInbox, fmtDate } from "../api";
import type { InboxItem } from "../api";
import { BackBtn, Empty } from "../components/ui";
import { IconBell, IconCalendar, IconCheck, IconCreditCard, IconUsers, IconInbox, IconX } from "../components/icons";

const KIND = {
  announcement: { Icon: IconBell, bg: "var(--c-accent-soft)", fg: "var(--c-pill-accent-fg)", label: "Announcement" },
  charge: { Icon: IconCreditCard, bg: "var(--c-destructive-soft)", fg: "var(--c-destructive)", label: "Charge" },
  meeting: { Icon: IconUsers, bg: "var(--c-primary-soft)", fg: "var(--c-primary-strong)", label: "Meeting" },
  booking: { Icon: IconCalendar, bg: "var(--c-info-soft)", fg: "var(--c-info)", label: "Booking" },
} as const;

export default function Inbox() {
  const [items, setItems] = useState<InboxItem[]>([]);
  const [filter, setFilter] = useState<"all" | "unread">("all");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchInbox().then((r) => { setItems(r.data); setLoading(false); });
  }, []);

  function rsvp(id: string, going: boolean) {
    setItems((xs) => xs.map((x) => (x.id === id && x.meeting ? { ...x, meeting: { going } } : x)));
  }

  const visible = filter === "unread" ? items.filter((x) => !x.read) : items;
  const unread = items.filter((x) => !x.read).length;

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Inbox</h1>
        <div style={{ width: 38 }} />
      </div>

      <div className="row-between" style={{ marginBottom: 12 }}>
        <div className="segmented">
          <button className={`segmented__btn${filter === "all" ? " segmented__btn--active" : ""}`} onClick={() => setFilter("all")}>
            All ({items.length})
          </button>
          <button className={`segmented__btn${filter === "unread" ? " segmented__btn--active" : ""}`} onClick={() => setFilter("unread")}>
            Unread ({unread})
          </button>
        </div>
      </div>

      {loading && <div className="card"><div className="skeleton" style={{ height: 80 }} /></div>}
      {!loading && visible.length === 0 && <Empty text="No messages yet" icon={<IconInbox />} />}

      <div className="stack">
        {visible.map((x) => {
          const k = KIND[x.kind];
          const Icon = k.Icon;
          return (
            <div className="card" key={x.id} style={{ display: "grid", gap: 8, padding: 14 }}>
              <div style={{ display: "flex", gap: 12, alignItems: "start" }}>
                <span className="row__icon" style={{ background: k.bg, color: k.fg, flexShrink: 0 }}><Icon /></span>
                <div className="row__body">
                  <div className="row-between" style={{ gap: 8 }}>
                    <div className="row__title" style={{ display: "flex", gap: 6, alignItems: "center" }}>
                      {x.title}
                      {!x.read && <span style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--c-primary)" }} />}
                    </div>
                    <span className="tiny muted">{fmtDate(x.date)}</span>
                  </div>
                  <div className="row__sub" style={{ marginTop: 4 }}>{x.body}</div>
                </div>
              </div>
              {x.meeting && (
                <div className="rsvp" style={{ marginTop: 4 }}>
                  {x.meeting.going === undefined ? (
                    <>
                      <button className="btn btn-primary" style={{ flex: 1 }} onClick={() => rsvp(x.id, true)}>
                        <IconCheck size={15} /> Attending
                      </button>
                      <button className="btn btn-secondary" style={{ flex: 1 }} onClick={() => rsvp(x.id, false)}>
                        <IconX size={15} /> Not attending
                      </button>
                    </>
                  ) : (
                    <span className={`pill ${x.meeting.going ? "pill--success" : "pill--neutral"}`}>
                      {x.meeting.going ? "Attendance confirmed" : "Declined"}
                    </span>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
