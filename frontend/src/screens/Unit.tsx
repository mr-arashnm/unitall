// "My Unit" — resident's unit details + assigned assets + ownership history.
import { IconBox, IconBuilding, IconCar, IconUsers, IconHome } from "../components/icons";
import { BackBtn } from "../components/ui";

const transfers = [
  { role: "Owner", who: "Sara Ahmadi", from: "2024-07-10", note: "Initial purchase" },
  { role: "Tenant", who: "Ali Karimi", from: "2025-08-01", note: "12-month lease" },
];

export default function Unit() {
  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>My unit</h1>
        <div style={{ width: 38 }} />
      </div>

      <div className="card stack">
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <span className="row__icon row__icon--primary" style={{ width: 46, height: 46 }}><IconBuilding /></span>
          <div>
            <div className="row__title" style={{ fontSize: 17 }}>Unit 301 — Floor 3</div>
            <div className="row__sub">Unital Tower · 120 m² · 3 bed</div>
          </div>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
          <div className="row" style={{ boxShadow: "none", flexDirection: "column", alignItems: "stretch", padding: "12px 14px" }}>
            <span className="row__icon" style={{ background: "var(--c-info-soft)", color: "var(--c-info)", width: 36, height: 36 }}><IconCar /></span>
            <div className="row__title" style={{ fontSize: 14, marginTop: 6 }}>Parking P-01</div>
            <div className="row__sub">Floor B1</div>
          </div>
          <div className="row" style={{ boxShadow: "none", flexDirection: "column", alignItems: "stretch", padding: "12px 14px" }}>
            <span className="row__icon" style={{ background: "var(--c-warning-soft)", color: "var(--c-warning)", width: 36, height: 36 }}><IconBox /></span>
            <div className="row__title" style={{ fontSize: 14, marginTop: 6 }}>Storage W-12</div>
            <div className="row__sub">4 m²</div>
          </div>
        </div>
      </div>

      <h2 className="section-title">Unit history</h2>
      <div className="card">
        <div className="timeline">
          {transfers.map((t, i) => (
            <div className="timeline__item" key={i}>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <IconUsers size={15} style={{ color: "var(--c-fg-2)" }} />
                <span className="row__title">{t.who}</span>
                <span className="chip" style={{ fontSize: 11.5, padding: "1px 9px" }}>{t.role}</span>
              </div>
              <div className="row__sub" style={{ marginTop: 2 }}>From {t.from} · {t.note}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
