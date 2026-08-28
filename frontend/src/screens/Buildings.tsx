// Buildings list — address-plate style cards.
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { fetchBuildings, session } from "../api";
import type { Building } from "../api";
import { BackBtn } from "../components/ui";
import { IconBuildings, IconHome, IconPlus, IconChevronRight } from "../components/icons";
import BuildingSwitcher from "../components/BuildingSwitcher";

export default function Buildings() {
  const [buildings, setBuildings] = useState<Building[]>([]);
  const [loading, setLoading] = useState(true);
  const nav = useNavigate();

  useEffect(() => {
    fetchBuildings().then((r) => { setBuildings(r.data); setLoading(false); });
  }, []);

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => nav("/")} />
        <BuildingSwitcher />
      </div>

      <div className="row-between" style={{ marginBottom: 16 }}>
        <h1 className="page-title" style={{ margin: 0 }}>My buildings</h1>
        <Link to="/buildings/new" className="btn btn-secondary btn-sm">
          <IconPlus size={15} /> New building
        </Link>
      </div>

      {loading && (
        <div className="stack">
          {[1, 2, 3].map((i) => <div key={i} className="card"><div className="skeleton" style={{ height: 80 }} /></div>)}
        </div>
      )}

      {!loading && buildings.length === 0 && (
        <div className="card" style={{ textAlign: "center", padding: 40 }}>
          <div className="row__icon row__icon--muted" style={{ margin: "0 auto 12px" }}><IconBuildings /></div>
          <h3 style={{ margin: "0 0 4px" }}>No buildings yet</h3>
          <p className="secondary small">Add your building to get started.</p>
          <Link to="/buildings/new" className="btn btn-primary" style={{ marginTop: 16 }}>
            <IconPlus size={16} /> Add building
          </Link>
        </div>
      )}

      <div className="stack">
        {buildings.map((b) => (
          <button
            key={b.id}
            className="building-plate"
            onClick={() => { session.currentBuilding = b.id; nav(`/buildings/${b.id}`); }}
            style={{ textAlign: "start" }}
          >
            <div className="building-plate__header">
              <div className="building-plate__plate">{b.code}</div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div className="building-plate__title">{b.name}</div>
                <div className="building-plate__addr">{b.address}</div>
              </div>
              <IconChevronRight size={18} style={{ color: "var(--c-fg-3)" }} />
            </div>
            <div className="building-plate__meta">
              <span style={{ display: "inline-flex", gap: 4, alignItems: "center" }}>
                <IconHome size={14} />
                {typeLabel(b.type)}
                {b.floors ? ` · ${b.floors} floors` : ""}
              </span>
              {b.features && b.features.length > 0 && (
                <span style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
                  {b.features.slice(0, 3).map((f) => <span key={f} className="pill pill--neutral" style={{ fontSize: 10.5 }}>{featureLabel(f)}</span>)}
                </span>
              )}
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}

function typeLabel(t: Building["type"]) {
  return { residential: "Residential", commercial: "Commercial", office: "Office", mixed: "Mixed" }[t] ?? t;
}
function featureLabel(f: string) {
  return { billing: "Billing", notifications: "Notifications", facilities: "Facilities", operations: "Operations" }[f] ?? f;
}
