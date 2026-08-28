// Create a new building.
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { createBuilding, session } from "../api";
import { BackBtn, Toast } from "../components/ui";

export default function NewBuilding() {
  const nav = useNavigate();
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [type, setType] = useState<"residential" | "commercial" | "office" | "mixed">("residential");
  const [address, setAddress] = useState("");
  const [floors, setFloors] = useState("10");
  const [features, setFeatures] = useState<string[]>(["billing", "notifications"]);
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState("");

  function toggleFeature(f: string) {
    setFeatures((arr) => arr.includes(f) ? arr.filter((x) => x !== f) : [...arr, f]);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!name || !code || !address) { setToast("Name, code, and address are required."); return; }
    setBusy(true);
    setToast("");
    try {
      const b = await createBuilding({ name, code, type, address, floors: Number(floors) || undefined, features });
      if (!b || !b.id) { throw new Error("Server returned no building id"); }
      session.currentBuilding = b.id;
      nav(`/buildings/${b.id}`);
    } catch (err) {
      console.error("create building failed", err);
      setToast((err as Error)?.message || "Failed to create building");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 8 }}>
        <BackBtn onClick={() => nav(-1)} />
        <h1 className="page-title" style={{ margin: 0 }}>New building</h1>
        <div style={{ width: 38 }} />
      </div>

      <form className="card stack" onSubmit={submit} noValidate>
        <div className="field">
          <label htmlFor="b-name">Building name</label>
          <input id="b-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Unital Tower" required />
        </div>
        <div className="field-row">
          <div className="field">
            <label htmlFor="b-code">Code</label>
            <input id="b-code" value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} placeholder="TWR-1" required />
          </div>
          <div className="field">
            <label htmlFor="b-type">Type</label>
            <select id="b-type" value={type} onChange={(e) => setType(e.target.value as typeof type)}>
              <option value="residential">Residential</option>
              <option value="commercial">Commercial</option>
              <option value="office">Office</option>
              <option value="mixed">Mixed</option>
            </select>
          </div>
        </div>
        <div className="field">
          <label htmlFor="b-addr">Address</label>
          <input id="b-addr" value={address} onChange={(e) => setAddress(e.target.value)} placeholder="123 Main St, City" required />
        </div>
        <div className="field">
          <label htmlFor="b-floors">Number of floors</label>
          <input id="b-floors" type="number" min="1" max="100" value={floors} onChange={(e) => setFloors(e.target.value)} />
        </div>
        <div className="field">
          <label>Features</label>
          <div className="role-strip">
            {[
              { v: "billing", l: "Billing" },
              { v: "notifications", l: "Notifications" },
              { v: "facilities", l: "Facilities" },
              { v: "operations", l: "Operations" },
            ].map((f) => (
              <button
                key={f.v} type="button"
                onClick={() => toggleFeature(f.v)}
                className={`badge ${features.includes(f.v) ? "badge--success" : "badge--muted"}`}
                style={{ border: 0, cursor: "pointer", padding: "4px 12px" }}
              >
                {f.l}
              </button>
            ))}
          </div>
        </div>
        <button className="btn btn-primary btn-block" type="submit" disabled={busy}>
          {busy ? "Creating…" : "Create building"}
        </button>
      </form>

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}
