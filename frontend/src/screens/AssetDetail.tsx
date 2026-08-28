// Asset detail — 3 tabs: Overview, Assignment, History.
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  fetchAsset, patchAsset, assignAsset, releaseAsset,
  fetchUnits, fetchUnitParties,
  type Asset, type Unit,
} from "../api";
import { BackBtn, Tabs, Empty, Modal, Toast } from "../components/ui";
import {
  IconBox, IconHome, IconArrowRight, IconEdit,
} from "../components/icons";

type TabId = "overview" | "assignment" | "history";

export default function AssetDetail() {
  const { id: buildingId, assetId } = useParams<{ id: string; assetId: string }>();
  const nav = useNavigate();
  const [asset, setAsset] = useState<Asset | null>(null);
  const [assignedUnit, setAssignedUnit] = useState<Unit | null>(null);
  const [unitOwner, setUnitOwner] = useState<string>("");
  const [tab, setTab] = useState<TabId>("overview");
  const [toast, setToast] = useState("");
  const [editOpen, setEditOpen] = useState(false);

  async function reload() {
    if (!assetId || !buildingId) return;
    const a = await fetchAsset(assetId);
    setAsset(a);
    if (a.unit_id) {
      const u = await fetchUnitParties(a.unit_id);
      setAssignedUnit(u as any);
      setUnitOwner((u as any).current?.owner?.user_id || "");
    } else {
      setAssignedUnit(null);
      setUnitOwner("");
    }
  }

  useEffect(() => { reload().catch((e) => setToast((e as Error).message)); }, [assetId, buildingId]);

  if (!asset) {
    return <div className="view"><div className="card"><div className="skeleton" style={{ height: 120 }} /></div></div>;
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 16 }}>
        <BackBtn onClick={() => buildingId && nav(`/buildings/${buildingId}`)} />
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          <div style={{
            width: 56, height: 56, background: "var(--c-bg-1, #f0f0f0)",
            borderRadius: 10, display: "flex", alignItems: "center", justifyContent: "center",
          }}>
            <IconBox size={24} />
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <div style={{ fontSize: 19, fontWeight: 800, letterSpacing: -0.2, fontFamily: "JetBrains Mono, monospace" }}>
                {asset.code}
              </div>
              <span className={`badge ${asset.kind === "parking" ? "badge--info" : "badge--primary"}`}>
                {asset.kind}
              </span>
              <button className="iconbtn" title="Edit asset" onClick={() => setEditOpen(true)}>
                <IconEdit size={16} />
              </button>
            </div>
            <div className="secondary small" style={{ marginTop: 2 }}>
              Floor {asset.floor || 0} · {asset.area_m2 || 0} m²
            </div>
            <div className="row" style={{ marginTop: 10, padding: 0, background: "transparent", border: 0, boxShadow: "none", gap: 16 }}>
              <span className={`badge ${asset.available ? "badge--success" : "badge--warning"}`}>
                {asset.available ? "Available" : "Assigned"}
              </span>
            </div>
          </div>
        </div>
      </div>

      <Tabs
        tabs={[
          { id: "overview", label: "Overview" },
          { id: "assignment", label: "Assignment" },
          { id: "history", label: "History" },
        ]}
        active={tab}
        onChange={(t) => setTab(t as TabId)}
      />

      {tab === "overview" && (
        <OverviewTab
          asset={asset}
          assignedUnitId={asset.unit_id}
          onOpenUnit={() => asset.unit_id && nav(`/buildings/${buildingId}/units/${asset.unit_id}`)}
        />
      )}
      {tab === "assignment" && (
        <AssignmentTab
          asset={asset}
          buildingId={buildingId!}
          onAssigned={reload}
          onToast={setToast}
        />
      )}
      {tab === "history" && <HistoryTab />}

      {editOpen && asset && (
        <EditAssetModal
          asset={asset}
          onClose={() => setEditOpen(false)}
          onSaved={async () => { setEditOpen(false); await reload(); }}
        />
      )}

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}

// ─── Tabs ─────────────────────────────────────────────────────

function OverviewTab({ asset, assignedUnitId, onOpenUnit }: { asset: Asset; assignedUnitId?: string; onOpenUnit: () => void }) {
  if (!assignedUnitId) {
    return (
      <div className="card">
        <div style={{ textAlign: "center", padding: "32px 0" }}>
          <div style={{ color: "var(--c-text-2, #888)", fontSize: 14 }}>This {asset.kind} is currently available.</div>
          <div style={{ color: "var(--c-text-2, #888)", fontSize: 13, marginTop: 6 }}>
            Use the Assignment tab to assign it to a unit.
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="card">
      <div style={{ marginBottom: 10 }}>
        <h3 style={{ margin: 0 }}>Assigned unit</h3>
      </div>
      <div className="row" style={{ cursor: "pointer" }} onClick={onOpenUnit}>
        <span className="row__icon row__icon--primary"><IconHome /></span>
        <div className="row__body">
          <div className="row__title">Unit {assignedUnitId.slice(0, 8)}…</div>
          <div className="row__sub">Click to view unit details</div>
        </div>
        <IconArrowRight size={16} style={{ color: "var(--c-text-2, #888)" }} />
      </div>
    </div>
  );
}

function AssignmentTab({ asset, buildingId, onAssigned, onToast }: {
  asset: Asset;
  buildingId: string;
  onAssigned: () => void;
  onToast: (msg: string) => void;
}) {
  const [units, setUnits] = useState<Unit[]>([]);
  const [picked, setPicked] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    fetchUnits(buildingId).then(setUnits).catch(() => setUnits([]));
  }, [buildingId]);

  async function doAssign() {
    if (!picked) { setErr("Pick a unit"); return; }
    setBusy(true); setErr("");
    try {
      await assignAsset(picked, asset.kind, asset.code);
      onToast(`${asset.kind} ${asset.code} assigned successfully`);
      onAssigned();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function doRelease() {
    if (!asset.unit_id) return;
    setBusy(true); setErr("");
    try {
      await releaseAsset(asset.unit_id, asset.kind, asset.code);
      onToast(`${asset.kind} ${asset.code} released`);
      onAssigned();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  if (!asset.available) {
    return (
      <div className="stack">
        <div className="card">
          <div style={{ marginBottom: 10 }}>
            <h3 style={{ margin: 0 }}>Currently assigned</h3>
          </div>
          <div className="row">
            <span className="row__icon row__icon--warning"><IconHome /></span>
            <div className="row__body">
              <div className="row__title">Unit {asset.unit_id.slice(0, 8)}…</div>
              <div className="row__sub">{asset.kind} is in use</div>
            </div>
          </div>
        </div>
        <div style={{ textAlign: "center" }}>
          <button className="btn" style={{ color: "var(--c-danger, #c33)" }} disabled={busy} onClick={doRelease}>
            {busy ? "Releasing…" : "Release asset"}
          </button>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)", textAlign: "center" }}>{err}</div>}
      </div>
    );
  }

  return (
    <div className="stack">
      <div className="card">
        <div style={{ marginBottom: 10 }}>
          <h3 style={{ margin: 0 }}>Assign to a unit</h3>
        </div>
        <div className="field">
          <label>Select unit</label>
          <select value={picked} onChange={(e) => setPicked(e.target.value)}>
            <option value="">— pick a unit —</option>
            {units.map((u) => (
              <option key={u.id} value={u.id}>Unit {u.number} · Floor {u.floor}</option>
            ))}
          </select>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div style={{ marginTop: 12 }}>
          <button className="btn btn-primary" disabled={busy || !picked} onClick={doAssign}>
            {busy ? "Assigning…" : `Assign ${asset.code}`}
          </button>
        </div>
      </div>
    </div>
  );
}

function HistoryTab() {
  // v1: history tracking is a future feature — show a placeholder.
  return (
    <div className="card">
      <div style={{ textAlign: "center", padding: "32px 0" }}>
        <div style={{ color: "var(--c-text-2, #888)", fontSize: 14 }}>
          Assignment history will be available in a future update.
        </div>
      </div>
    </div>
  );
}

// ─── Modal ────────────────────────────────────────────────────

function EditAssetModal({ asset, onClose, onSaved }: { asset: Asset; onClose: () => void; onSaved: () => void }) {
  const [code, setCode] = useState(asset.code);
  const [floor, setFloor] = useState(String(asset.floor || 0));
  const [area, setArea] = useState(String(asset.area_m2 || 0));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    setBusy(true); setErr("");
    try {
      await patchAsset(asset.id, {
        code: code !== asset.code ? code : undefined,
        floor: Number(floor),
        area_m2: Number(area),
      });
      onSaved();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal open title={`Edit ${asset.kind}`} onClose={onClose}>
      <div className="stack">
        <div className="field">
          <label>Code</label>
          <input value={code} onChange={(e) => setCode(e.target.value)} placeholder="e.g. P-01" />
        </div>
        <div className="field-row">
          <div className="field">
            <label>Floor</label>
            <input type="number" value={floor} onChange={(e) => setFloor(e.target.value)} />
          </div>
          <div className="field">
            <label>Area (m²)</label>
            <input type="number" value={area} onChange={(e) => setArea(e.target.value)} />
          </div>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Saving…" : "Save"}</button>
        </div>
      </div>
    </Modal>
  );
}
