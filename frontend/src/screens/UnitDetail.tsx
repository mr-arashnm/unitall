// Unit detail — 5 tabs: Overview, Parties, Transfers, Contracts, Assets.
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  fetchUnit, fetchUnitParties, fetchUnitTransfers, fetchUnitContracts,
  fetchAssets, changeOwnership, changeResidency, signContract, activateContract,
  searchUsers, inviteUser, patchUnit, type Unit, type UnitPartiesResponse, type TransferRecord,
  type Contract, type Asset, type User,
} from "../api";
import { BackBtn, Tabs, Empty, Modal, Toast } from "../components/ui";
import {
  IconHome, IconUsers, IconClock, IconFile, IconBox, IconArrowRight, IconEdit,
} from "../components/icons";

type TabId = "overview" | "parties" | "transfers" | "contracts" | "assets";

export default function UnitDetail() {
  const { id: buildingId, unitId } = useParams<{ id: string; unitId: string }>();
  const nav = useNavigate();
  const [unit, setUnit] = useState<Unit | null>(null);
  const [parties, setParties] = useState<UnitPartiesResponse>({ current: {}, history: [] });
  const [transfers, setTransfers] = useState<TransferRecord[]>([]);
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [assets, setAssets] = useState<Asset[]>([]);
  const [tab, setTab] = useState<TabId>("overview");
  const [toast, setToast] = useState("");
  const [changeFor, setChangeFor] = useState<"owner" | "resident" | null>(null);
  const [editUnit, setEditUnit] = useState(false);

  async function reload() {
    if (!unitId || !buildingId) return;
    const u = await fetchUnit(unitId);
    setUnit(u);
    setParties(await fetchUnitParties(unitId));
    setTransfers(await fetchUnitTransfers(unitId));
    setContracts(await fetchUnitContracts(unitId));
    const pk = await fetchAssets(buildingId, "parking");
    const wh = await fetchAssets(buildingId, "warehouse");
    setAssets([...pk, ...wh].filter((a) => a.unit_id === unitId));
  }

  useEffect(() => { reload().catch((e) => setToast((e as Error).message)); }, [unitId, buildingId]);

  if (!unit) {
    return <div className="view"><div className="card"><div className="skeleton" style={{ height: 120 }} /></div></div>;
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 16 }}>
        <BackBtn onClick={() => buildingId && nav(`/buildings/${buildingId}`)} />
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          <div className="building-plate__plate" style={{ width: 56, height: 56, fontSize: 14 }}>{unit.number}</div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <div style={{ fontSize: 19, fontWeight: 800, letterSpacing: -0.2 }}>Unit {unit.number}</div>
              <button className="iconbtn" title="Edit unit" onClick={() => setEditUnit(true)}><IconEdit size={16} /></button>
            </div>
            <div className="secondary small" style={{ marginTop: 2 }}>Floor {unit.floor} · {unit.area_m2 || 0} m² · {unit.rooms || 0} rooms</div>
            <div className="row" style={{ marginTop: 10, padding: 0, background: "transparent", border: 0, boxShadow: "none", gap: 16 }}>
              <span className="small secondary"><IconHome size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{unit.status}</span>
              <span className="small secondary"><IconUsers size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{parties.current.owner ? "Has owner" : "No owner"}</span>
              <span className="small secondary"><IconBox size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{assets.length} assets</span>
            </div>
          </div>
        </div>
      </div>

      <Tabs
        tabs={[
          { id: "overview", label: "Overview" },
          { id: "parties", label: "Parties" },
          { id: "transfers", label: "Transfers" },
          { id: "contracts", label: "Contracts" },
          { id: "assets", label: "Assets" },
        ]}
        active={tab}
        onChange={(t) => setTab(t as TabId)}
      />

      {tab === "overview" && <OverviewTab unit={unit} parties={parties} assets={assets} />}
      {tab === "parties" && <PartiesTab history={parties.history} current={parties.current} onChange={(r) => setChangeFor(r)} />}
      {tab === "transfers" && <TransfersTab transfers={transfers} />}
      {tab === "contracts" && <ContractsTab contracts={contracts} onSign={async (id) => { await signContract(id); await reload(); }} onActivate={async (id) => { await activateContract(id); await reload(); }} />}
      {tab === "assets" && <AssetsTab assets={assets} />}

      {changeFor && unitId && (
        <ChangePartyModal
          role={changeFor}
          unitId={unitId}
          onClose={() => setChangeFor(null)}
          onSaved={async () => { setChangeFor(null); await reload(); }}
        />
      )}
      {editUnit && unit && (
        <EditUnitModal
          unit={unit}
          onClose={() => setEditUnit(false)}
          onSaved={async () => { setEditUnit(false); await reload(); }}
        />
      )}

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}

// ─── Tabs ─────────────────────────────────────────────────────

function OverviewTab({ unit, parties, assets }: { unit: Unit; parties: UnitPartiesResponse; assets: Asset[] }) {
  return (
    <div className="stack">
      <div className="card">
        <div className="row-between" style={{ marginBottom: 8 }}>
          <h3 style={{ margin: 0 }}>Current parties</h3>
        </div>
        <div className="small secondary" style={{ marginBottom: 4 }}>Owner</div>
        <div className="row" style={{ background: "var(--c-bg-1, #f8f8f8)", padding: 10, borderRadius: 8 }}>
          <span className="row__icon row__icon--info"><IconUsers /></span>
          <div className="row__body">
            <div className="row__title">{parties.current.owner ? `User ${parties.current.owner.user_id.slice(0, 8)}` : "No owner"}</div>
            <div className="row__sub">{parties.current.owner ? `Since ${parties.current.owner.from.slice(0, 10)}` : "Assign via the Parties tab"}</div>
          </div>
        </div>
        <div className="small secondary" style={{ marginTop: 12, marginBottom: 4 }}>Resident</div>
        <div className="row" style={{ background: "var(--c-bg-1, #f8f8f8)", padding: 10, borderRadius: 8 }}>
          <span className="row__icon row__icon--info"><IconUsers /></span>
          <div className="row__body">
            <div className="row__title">{parties.current.resident ? `User ${parties.current.resident.user_id.slice(0, 8)}` : "No resident"}</div>
            <div className="row__sub">{parties.current.resident ? `Since ${parties.current.resident.from.slice(0, 10)}` : "Assign via the Parties tab"}</div>
          </div>
        </div>
      </div>
      <div className="card">
        <h3 style={{ margin: 0, marginBottom: 8 }}>Assigned assets ({assets.length})</h3>
        {assets.length === 0 ? (
          <Empty text="No assets assigned to this unit" icon={<IconBox />} />
        ) : (
          <div className="stack">
            {assets.map((a) => (
              <div className="row" key={a.id}>
                <span className="row__icon row__icon--primary"><IconBox /></span>
                <div className="row__body">
                  <div className="row__title">{a.code} ({a.kind})</div>
                  <div className="row__sub">Floor {a.floor} · {a.area_m2} m²</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function PartiesTab({ history, current, onChange }: { history: { id: string; unit_id: string; role: "owner" | "resident"; user_id: string; from: string; to?: string }[]; current: UnitPartiesResponse["current"]; onChange: (role: "owner" | "resident") => void }) {
  return (
    <div className="stack">
      <div className="row-between"><h3 style={{ margin: 0 }}>Current</h3></div>
      <div className="card" style={{ padding: 12 }}>
        <div className="row-between" style={{ marginBottom: 6 }}>
          <div>
            <div className="small secondary">Owner</div>
            <div>{current.owner ? `User ${current.owner.user_id.slice(0, 8)} · since ${current.owner.from.slice(0, 10)}` : "—"}</div>
          </div>
          <button className="btn btn-sm" onClick={() => onChange("owner")}>{current.owner ? "Change" : "Assign"}</button>
        </div>
        <div className="row-between" style={{ marginTop: 12 }}>
          <div>
            <div className="small secondary">Resident</div>
            <div>{current.resident ? `User ${current.resident.user_id.slice(0, 8)} · since ${current.resident.from.slice(0, 10)}` : "—"}</div>
          </div>
          <button className="btn btn-sm" onClick={() => onChange("resident")}>{current.resident ? "Change" : "Assign"}</button>
        </div>
      </div>
      <h3 style={{ margin: 0 }}>History</h3>
      {history.length === 0 ? <Empty text="No history" icon={<IconClock />} /> : (
        <div className="stack">
          {history.map((p) => (
            <div className="row" key={p.id}>
              <span className="row__icon row__icon--info"><IconUsers /></span>
              <div className="row__body">
                <div className="row__title">{p.role}: User {p.user_id.slice(0, 8)}</div>
                <div className="row__sub">{p.from.slice(0, 10)} → {p.to ? p.to.slice(0, 10) : "present"}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function TransfersTab({ transfers }: { transfers: TransferRecord[] }) {
  if (transfers.length === 0) return <Empty text="No transfer history" icon={<IconClock />} />;
  return (
    <div className="stack">
      {transfers.map((t) => (
        <div className="row" key={t.id}>
          <span className="row__icon row__icon--info"><IconClock /></span>
          <div className="row__body">
            <div className="row__title">{t.role}: User {t.previous_user_id?.slice(0, 8) || "—"} → {t.new_user_id.slice(0, 8)}</div>
            <div className="row__sub">{t.effective_date?.slice(0, 10)} {t.contract_number ? `· Contract ${t.contract_number}` : ""}</div>
            {t.description && <div className="row__sub">{t.description}</div>}
          </div>
        </div>
      ))}
    </div>
  );
}

function ContractsTab({ contracts, onSign, onActivate }: { contracts: Contract[]; onSign: (id: string) => void; onActivate: (id: string) => void }) {
  if (contracts.length === 0) return <Empty text="No contracts for this unit" icon={<IconFile />} />;
  return (
    <div className="stack">
      {contracts.map((c) => (
        <div className="card" key={c.id} style={{ padding: 12 }}>
          <div className="row-between" style={{ marginBottom: 6 }}>
            <div>
              <div style={{ fontWeight: 700 }}>{c.number}</div>
              <div className="small secondary">{c.type} · {c.start_date?.slice(0, 10)}</div>
            </div>
            <span className={`badge ${c.status === "active" ? "badge--success" : c.status === "draft" ? "badge--muted" : "badge--warning"}`}>{c.status}</span>
          </div>
          <div className="small secondary" style={{ marginTop: 4 }}>
            Amount: {c.amount || 0} · Deposit: {c.deposit_amount || 0} · {c.duration_months} months
          </div>
          <div className="row" style={{ marginTop: 8, padding: 0, background: "transparent", border: 0, boxShadow: "none", gap: 6 }}>
            <span className={`badge ${c.first_signed ? "badge--success" : "badge--muted"}`}>1st {c.first_signed ? "✓" : "—"}</span>
            <span className={`badge ${c.second_signed ? "badge--success" : "badge--muted"}`}>2nd {c.second_signed ? "✓" : "—"}</span>
            <div style={{ flex: 1 }} />
            {c.status === "draft" && (
              <button className="btn btn-sm" onClick={() => onSign(c.id)}>Sign</button>
            )}
            {c.status === "draft" && c.first_signed && c.second_signed && (
              <button className="btn btn-sm btn-primary" onClick={() => onActivate(c.id)}>Activate</button>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}

function AssetsTab({ assets }: { assets: Asset[] }) {
  if (assets.length === 0) return <Empty text="No assets assigned" icon={<IconBox />} />;
  return (
    <div className="grid-3">
      {assets.map((a) => (
        <div className="card" key={a.id} style={{ padding: 12, textAlign: "center", display: "grid", gap: 4 }}>
          <div style={{ fontFamily: "JetBrains Mono, monospace", fontWeight: 800, fontSize: 14 }}>{a.code}</div>
          <div className="tiny secondary">{a.kind} · Floor {a.floor} · {a.area_m2} m²</div>
        </div>
      ))}
    </div>
  );
}

// ─── Modals ───────────────────────────────────────────────────

function ChangePartyModal({ role, unitId, onClose, onSaved }: { role: "owner" | "resident"; unitId: string; onClose: () => void; onSaved: () => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<User[]>([]);
  const [picked, setPicked] = useState<User | null>(null);
  const [email, setEmail] = useState("");
  const [date, setDate] = useState(new Date().toISOString().slice(0, 10));
  const [contractNumber, setContractNumber] = useState("");
  const [description, setDescription] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!query) { setResults([]); return; }
    const t = setTimeout(() => { searchUsers(query).then(setResults).catch(() => setResults([])); }, 250);
    return () => clearTimeout(t);
  }, [query]);

  async function save() {
    setBusy(true); setErr("");
    try {
      let userId = picked?.id;
      if (!userId && email) {
        const u = await inviteUser(email);
        userId = u.id;
      }
      if (!userId) { setErr("Pick a user or enter an email to invite"); setBusy(false); return; }
      const fn = role === "owner" ? changeOwnership : changeResidency;
      await fn(unitId, { new_user_id: userId, effective_date: date, contract_number: contractNumber || undefined, description: description || undefined });
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title={`Change ${role}`} onClose={onClose}>
      <div className="stack">
        <div className="field">
          <label>Find existing user</label>
          <input value={query} onChange={(e) => { setQuery(e.target.value); setPicked(null); }} placeholder="email prefix" />
          {results.length > 0 && !picked && (
            <div className="card" style={{ marginTop: 6, padding: 4, maxHeight: 160, overflow: "auto" }}>
              {results.map((u) => (
                <div key={u.id} className="row" style={{ padding: 6, cursor: "pointer" }} onClick={() => { setPicked(u); setQuery(u.email); setResults([]); }}>
                  <div className="row__body">
                    <div className="row__title">{u.email}</div>
                    <div className="row__sub">{u.full_name || "(no name)"}</div>
                  </div>
                </div>
              ))}
            </div>
          )}
          {picked && <div className="small" style={{ marginTop: 4, color: "var(--c-primary, #0a8)" }}>Picked: {picked.email}</div>}
        </div>
        <div className="field">
          <label>…or invite by email</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="newperson@example.com" disabled={!!picked} />
        </div>
        <div className="field-row">
          <div className="field"><label>Effective date</label><input type="date" value={date} onChange={(e) => setDate(e.target.value)} /></div>
          <div className="field"><label>Contract #</label><input value={contractNumber} onChange={(e) => setContractNumber(e.target.value)} placeholder="Optional" /></div>
        </div>
        <div className="field"><label>Description</label><input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional" /></div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Saving…" : "Save"}</button>
        </div>
      </div>
    </Modal>
  );
}

function EditUnitModal({ unit, onClose, onSaved }: { unit: Unit; onClose: () => void; onSaved: () => void }) {
  const [area, setArea] = useState(String(unit.area_m2 || 0));
  const [rooms, setRooms] = useState(String(unit.rooms || 0));
  const [status, setStatus] = useState<Unit["status"]>(unit.status);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    setBusy(true); setErr("");
    try {
      await patchUnit(unit.id, { area_m2: Number(area), rooms: Number(rooms), status });
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title="Edit unit" onClose={onClose}>
      <div className="stack">
        <div className="field-row">
          <div className="field"><label>Area (m²)</label><input type="number" value={area} onChange={(e) => setArea(e.target.value)} /></div>
          <div className="field"><label>Rooms</label><input type="number" value={rooms} onChange={(e) => setRooms(e.target.value)} /></div>
        </div>
        <div className="field"><label>Status</label>
          <select value={status} onChange={(e) => setStatus(e.target.value as Unit["status"])}>
            <option value="vacant">Vacant</option>
            <option value="occupied">Occupied</option>
            <option value="under_construction">Under construction</option>
          </select>
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
