// Meetings — upcoming, past, RSVP.
import { useEffect, useState } from "react";
import { fetchMeetings, session } from "../api";
import type { Meeting } from "../api";
import { BackBtn, Empty, TaskStatusBadge } from "../components/ui";
import { IconUsers, IconCalendar, IconLocation, IconClock } from "../components/icons";

const TYPE_LABELS: Record<string, string> = {
  board: "Board meeting", general: "General assembly",
  committee: "Committee", emergency: "Emergency meeting",
};

export default function Meetings() {
  const [meetings, setMeetings] = useState<Meeting[]>([]);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<"upcoming" | "past">("upcoming");

  useEffect(() => {
    const bid = session.currentBuilding;
    if (bid) fetchMeetings(bid).then((r) => { setMeetings(r); setLoading(false); });
    else setLoading(false);
  }, []);

  const upcoming = meetings.filter((m) => m.status === "scheduled" || m.status === "ongoing");
  const past = meetings.filter((m) => m.status === "completed" || m.status === "cancelled");
  const visible = tab === "upcoming" ? upcoming : past;

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Meetings</h1>
        <div style={{ width: 38 }} />
      </div>

      <div className="segmented" style={{ marginBottom: 12 }}>
        <button className={`segmented__btn${tab === "upcoming" ? " segmented__btn--active" : ""}`} onClick={() => setTab("upcoming")}>
          Upcoming ({upcoming.length})
        </button>
        <button className={`segmented__btn${tab === "past" ? " segmented__btn--active" : ""}`} onClick={() => setTab("past")}>
          Past ({past.length})
        </button>
      </div>

      {loading && <div className="card"><div className="skeleton" style={{ height: 80 }} /></div>}
      {!loading && visible.length === 0 && <Empty text={tab === "upcoming" ? "No upcoming meetings" : "No past meetings"} icon={<IconUsers />} />}

      <div className="stack">
        {visible.map((m) => (
          <div key={m.id} className="card" style={{ display: "grid", gap: 8 }}>
            <div className="row-between">
              <div style={{ fontWeight: 700, fontSize: 15.5 }}>{m.title}</div>
              <TaskStatusBadge status={m.status} />
            </div>
            <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
              <span className="pill pill--neutral">{TYPE_LABELS[m.type] ?? m.type}</span>
              {m.location && (
                <span className="small secondary" style={{ display: "flex", gap: 4, alignItems: "center" }}>
                  <IconLocation size={13} /> {m.location}
                </span>
              )}
            </div>
            <div className="small secondary" style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
              <span style={{ display: "flex", gap: 4, alignItems: "center" }}>
                <IconCalendar size={13} />
                {new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long" }).format(new Date(m.scheduled_at))}
              </span>
              <span style={{ display: "flex", gap: 4, alignItems: "center" }}>
                <IconClock size={13} />
                {new Intl.DateTimeFormat("en-US", { hour: "2-digit", minute: "2-digit" }).format(new Date(m.scheduled_at))}
                {m.duration_min ? ` · ${m.duration_min} min` : ""}
              </span>
            </div>
            {m.agenda && (
              <div className="small secondary" style={{ marginTop: 4, lineHeight: 1.6 }}>{m.agenda}</div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
