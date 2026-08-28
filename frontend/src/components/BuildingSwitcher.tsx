// Building switcher — shows the current building and lets the user
// switch between buildings they belong to.
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { fetchMyBuildings, session } from "../api";
import type { Building } from "../api";
import { IconBuildings, IconChevronDown, IconPlus } from "./icons";

export default function BuildingSwitcher() {
  const [buildings, setBuildings] = useState<Building[]>([]);
  const [open, setOpen] = useState(false);
  const nav = useNavigate();

  useEffect(() => {
    fetchMyBuildings().then((r) => {
      setBuildings(r.data);
      if (!session.currentBuilding && r.data.length > 0) {
        session.currentBuilding = r.data[0].id;
      }
    });
  }, []);

  const current = buildings.find((b) => b.id === session.currentBuilding) ?? buildings[0];

  function pick(id: string) {
    session.currentBuilding = id;
    setOpen(false);
    nav("/buildings");
  }

  if (!current) {
    return (
      <button className="switcher__btn" onClick={() => nav("/buildings/new")}>
        <IconPlus size={16} />
        Add building
      </button>
    );
  }

  return (
    <div className="switcher">
      <button className="switcher__btn" onClick={() => setOpen((v) => !v)} aria-haspopup="listbox" aria-expanded={open}>
        <IconBuildings size={16} />
        <span style={{ fontWeight: 600, fontSize: 14 }}>{current.name}</span>
        <IconChevronDown size={14} />
      </button>
      {open && (
        <div className="switcher__menu" role="listbox">
          {buildings.map((b) => (
            <button
              key={b.id}
              role="option"
              aria-selected={b.id === current.id}
              className={`switcher__item${b.id === current.id ? " switcher__item--active" : ""}`}
              onClick={() => pick(b.id)}
            >
              <span style={{
                width: 26, height: 26, borderRadius: 7,
                background: "var(--c-surface-alt)", color: "var(--c-primary-strong)",
                display: "grid", placeItems: "center",
                fontWeight: 700, fontSize: 10, fontFamily: "JetBrains Mono, monospace",
              }}>{b.code}</span>
              <span style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{b.name}</span>
            </button>
          ))}
          <div style={{ height: 1, background: "var(--c-border)", margin: "4px 0" }} />
          <Link to="/buildings" className="switcher__item" onClick={() => setOpen(false)}>
            <IconBuildings size={16} />
            All buildings
          </Link>
          <Link to="/buildings/new" className="switcher__item" onClick={() => setOpen(false)}>
            <IconPlus size={16} />
            New building
          </Link>
        </div>
      )}
    </div>
  );
}
