// Facilities — browse and book common areas.
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchFacilities, fetchMyBookings, bookFacility, session } from "../api";
import type { Facility, Booking } from "../api";
import { BackBtn, Empty, Toast, Modal } from "../components/ui";
import { IconPool, IconDumbbell, IconDoor, IconHome, IconCalendar, IconCar } from "../components/icons";

const TYPE_ICONS: Record<string, typeof IconHome> = {
  pool: IconPool, gym: IconDumbbell, party_hall: IconDoor, meeting_room: IconHome,
  guest_parking: IconCar, roof_garden: IconHome, playground: IconHome, sports_court: IconHome,
  library: IconHome, business_center: IconHome, other: IconHome,
};
const TYPE_LABELS: Record<string, string> = {
  pool: "Pool", gym: "Gym", party_hall: "Party Hall",
  meeting_room: "Meeting Room", guest_parking: "Guest Parking", roof_garden: "Rooftop Garden",
  playground: "Playground", sports_court: "Sports Court", library: "Library",
  business_center: "Business Center", other: "Other",
};

export default function Facilities() {
  const nav = useNavigate();
  const [facilities, setFacilities] = useState<Facility[]>([]);
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [showBooking, setShowBooking] = useState<Facility | null>(null);
  const [toast, setToast] = useState("");
  const [booking, setBooking] = useState({ date: "", start: "08:00", end: "09:00", purpose: "" });
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    const bid = session.currentBuilding;
    if (bid) {
      fetchFacilities(bid).then(setFacilities).catch(() => setFacilities([]));
      fetchMyBookings().then(setBookings).catch(() => setBookings([]));
    } else {
      // No building selected — show empty state but still render the page
      setFacilities([]);
      setBookings([]);
    }
  }, []);

  async function doBook(f: Facility) {
    if (!booking.date) { setToast("Please pick a date"); return; }
    setSubmitting(true);
    const start = `${booking.date}T${booking.start}:00Z`;
    const end = `${booking.date}T${booking.end}:00Z`;
    const b = await bookFacility(f.id, { start, end, purpose: booking.purpose });
    setBookings((bs) => [b, ...bs]);
    setShowBooking(null);
    setSubmitting(false);
    setToast(`Booking for ${f.name} submitted and pending approval.`);
  }

  const activeBookings = bookings.filter((b) => b.status === "confirmed" || b.status === "pending");

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Facilities</h1>
        <div style={{ width: 38 }} />
      </div>

      {activeBookings.length > 0 && (
        <>
          <p className="eyebrow" style={{ marginBottom: 8 }}>My bookings</p>
          <div className="stack" style={{ marginBottom: 16 }}>
            {activeBookings.map((b) => {
              const f = facilities.find((x) => x.id === b.facility_id);
              return (
                <div className="row" key={b.id} style={{ borderColor: b.status === "pending" ? "var(--c-warning-soft)" : "var(--c-border)" }}>
                  <span className="row__icon row__icon--primary"><IconCalendar /></span>
                  <div className="row__body">
                    <div className="row__title">{f?.name ?? "—"}{" "}</div>
                    <div className="row__sub">
                      {new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long", hour: "2-digit", minute: "2-digit" }).format(new Date(b.start))}
                    </div>
                  </div>
                  <span className={`badge ${b.status === "confirmed" ? "badge--success" : "badge--accent"}`}>
                    {b.status === "confirmed" ? "Confirmed" : "Pending"}
                  </span>
                </div>
              );
            })}
          </div>
        </>
      )}

      <p className="eyebrow" style={{ marginBottom: 8 }}>Shared spaces</p>

      {facilities.length === 0 && <Empty text="No facilities defined" icon={<IconPool />} />}

      <div className="grid-2">
        {facilities.map((f) => {
          const Icon = TYPE_ICONS[f.type] ?? IconHome;
          const isFree = f.hourly_rate === 0;
          return (
            <div key={f.id} className="facility-card" onClick={() => setShowBooking(f)}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "start" }}>
                <div className="facility-card__icon"><Icon /></div>
                {isFree && <span className="badge badge--success">Free</span>}
              </div>
              <div className="facility-card__name">{f.name}</div>
              <div className="facility-card__meta">
                <span>{TYPE_LABELS[f.type] ?? f.type}</span>
                <span>·</span>
                <span>{f.capacity} people</span>
              </div>
              <div className="facility-card__meta" style={{ color: "var(--c-fg-3)" }}>
                <span>{f.opening_time} – {f.closing_time}</span>
                {!isFree && <span>· {new Intl.NumberFormat("en-US").format(f.hourly_rate)} R/h</span>}
              </div>
            </div>
          );
        })}
      </div>

      <Modal open={!!showBooking} onClose={() => setShowBooking(null)} title={showBooking ? `Book: ${showBooking.name}` : ""}>
        {showBooking && (
          <div className="stack">
            <div className="card card--inset" style={{ padding: 12 }}>
              <div style={{ fontWeight: 700, fontSize: 15 }}>{showBooking.name}</div>
              <div className="small secondary" style={{ marginTop: 4 }}>
                {TYPE_LABELS[showBooking.type] ?? showBooking.type} · Capacity {showBooking.capacity}
                {showBooking.hourly_rate > 0 && ` · ${new Intl.NumberFormat("en-US").format(showBooking.hourly_rate)} R/h`}
              </div>
              {showBooking.rules && showBooking.rules.length > 0 && (
                <div className="tiny secondary" style={{ marginTop: 8 }}>
                  {showBooking.rules.map((r) => `• ${r}`).join("\n")}
                </div>
              )}
            </div>
            <div className="field">
              <label>Date</label>
              <input type="date" value={booking.date} onChange={(e) => setBooking((b) => ({ ...b, date: e.target.value }))} />
            </div>
            <div className="field-row">
              <div className="field">
                <label>From</label>
                <input type="time" value={booking.start} onChange={(e) => setBooking((b) => ({ ...b, start: e.target.value }))} />
              </div>
              <div className="field">
                <label>To</label>
                <input type="time" value={booking.end} onChange={(e) => setBooking((b) => ({ ...b, end: e.target.value }))} />
              </div>
            </div>
            <div className="field">
              <label>Purpose (optional)</label>
              <input value={booking.purpose} onChange={(e) => setBooking((b) => ({ ...b, purpose: e.target.value }))} placeholder="e.g. Birthday party" />
            </div>
            <button className="btn btn-primary btn-block" disabled={submitting} onClick={() => doBook(showBooking)}>
              {submitting ? "Submitting…" : "Confirm booking"}
            </button>
          </div>
        )}
      </Modal>

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}
