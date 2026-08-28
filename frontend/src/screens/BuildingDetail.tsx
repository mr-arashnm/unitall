// Building detail — tabbed view of units, assets, members, contracts.
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  fetchBuilding, fetchUnits, fetchAssets, fetchMemberships,
  createUnit, patchUnit, createAsset, assignAsset, releaseAsset,
  patchBuilding, fetchUnitContracts, createContract,
  type Building, type Unit, type Asset, type Membership, type Contract,
} from "../api";
import { BackBtn, Tabs, Empty, RoleBadge, Modal, Toast } from "../components/ui";
import {
  IconHome, IconCar, IconBox, IconUsers,
  IconPlus, IconFile, IconEdit, IconArrowRight,
} from "../components/icons";

type TabId = "units" | "assets" | "members" | "contracts";

export default function BuildingDetail() {
  const { id } = useParams<{ id: string }>();
  const nav = useNavigate();
  const [building, setBuilding] = useState<Building | null>(null);
  const [tab, setTab] = useState<TabId>("units");
  const [units, setUnits] = useState<Unit[]>([]);
  const [parkings, setParkings] = useState<Asset[]>([]);
  const [warehouses, setWarehouses] = useState<Asset[]>([]);
  const [members, setMembers] = useState<Membership[]>([]);
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [toast, setToast] = useState("");
  const [editBldg, setEditBldg] = useState(false);
  const [addUnit, setAddUnit] = useState(false);
  const [addAsset, setAddAsset] = useState<"parking" | "warehouse" | null>(null);
  const [assignFor, setAssignFor] = useState<Asset | null>(null);
  const [newContract, setNewContract] = useState(false);

  async function reload() {
    if (!id) return;
    const b = await fetchBuilding(id);
    setBuilding(b);
    setUnits(await fetchUnits(id));
    setParkings(await fetchAssets(id, "parking"));
    setWarehouses(await fetchAssets(id, "warehouse"));
    setMembers(await fetchMemberships(id));
    // Aggregate contracts across all units
    const allContracts: Contract[] = [];
    for (const u of await fetchUnits(id)) {
      const cs = await fetchUnitContracts(u.id);
      allContracts.push(...cs);
    }
    allContracts.sort((a, b) => (b.created_at || "").localeCompare(a.created_at || ""));
    setContracts(allContracts);
  }

  useEffect(() => { reload().catch((e) => setToast((e as Error).message)); }, [id]);

  if (!building) {
    return <div className="view"><div className="card"><div className="skeleton" style={{ height: 120 }} /></div></div>;
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 16 }}>
        <BackBtn onClick={() => nav("/buildings")} />
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
          <div className="building-plate__plate" style={{ width: 56, height: 56, fontSize: 14 }}>{building.code}</div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <div style={{ fontSize: 19, fontWeight: 800, letterSpacing: -0.2 }}>{building.name}</div>
              <button className="iconbtn" title="Edit building" onClick={() => setEditBldg(true)}><IconEdit size={16} /></button>
            </div>
            <div className="secondary small" style={{ marginTop: 2 }}>{building.address}</div>
            <div className="row" style={{ marginTop: 10, padding: 0, background: "transparent", border: 0, boxShadow: "none", gap: 16 }}>
              <span className="small secondary"><IconHome size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{units.length} units</span>
              <span className="small secondary"><IconCar size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{parkings.length} parking</span>
              <span className="small secondary"><IconBox size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{warehouses.length} storage</span>
              <span className="small secondary"><IconUsers size={14} style={{ verticalAlign: "-2px", marginInlineEnd: 4 }} />{members.length} members</span>
            </div>
          </div>
        </div>
      </div>

      <Tabs
        tabs={[
          { id: "units", label: "Units" },
          { id: "assets", label: "Assets" },
          { id: "members", label: "Members" },
          { id: "contracts", label: "Contracts" },
        ]}
        active={tab}
        onChange={(t) => setTab(t as TabId)}
      />

      {tab === "units" && (
        <UnitsTab
          units={units}
          onAdd={() => setAddUnit(true)}
          onOpen={(u) => nav(`/buildings/${id}/units/${u.id}`)}
          onEdit={(u) => patchUnit(u.id, { area_m2: u.area_m2, rooms: u.rooms, status: u.status }).then(() => reload())}
        />
      )}
      {tab === "assets" && (
        <AssetsTab
          parkings={parkings} warehouses={warehouses}
          units={units}
          onAdd={(k) => setAddAsset(k)}
          onAssign={(a) => setAssignFor(a)}
          onRelease={(a) => releaseAsset(a.unit_id || "", a.kind, a.code).then(reload)}
          onOpen={(a) => nav(`/buildings/${id}/assets/${a.id}`)}
        />
      )}
      {tab === "members" && <MembersTab members={members} />}
      {tab === "contracts" && (
        <ContractsTab
          contracts={contracts}
          units={units}
          onNew={() => setNewContract(true)}
        />
      )}

      {/* Modals */}
      {editBldg && (
        <EditBuildingModal
          building={building}
          onClose={() => setEditBldg(false)}
          onSaved={async () => { setEditBldg(false); await reload(); }}
        />
      )}
      {addUnit && id && (
        <AddUnitModal
          buildingId={id}
          onClose={() => setAddUnit(false)}
          onSaved={async () => { setAddUnit(false); await reload(); }}
        />
      )}
      {addAsset && id && (
        <AddAssetModal
          buildingId={id}
          kind={addAsset}
          onClose={() => setAddAsset(null)}
          onSaved={async () => { setAddAsset(null); await reload(); }}
        />
      )}
      {assignFor && (
        <AssignAssetModal
          asset={assignFor}
          units={units}
          onClose={() => setAssignFor(null)}
          onSaved={async () => { setAssignFor(null); await reload(); }}
        />
      )}
      {newContract && id && (
        <NewContractModal
          buildingId={id}
          units={units}
          onClose={() => setNewContract(false)}
          onSaved={async () => { setNewContract(false); await reload(); }}
        />
      )}

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}

// ─── Tabs ─────────────────────────────────────────────────────

function UnitsTab({ units, onAdd, onOpen, onEdit }: { units: Unit[]; onAdd: () => void; onOpen: (u: Unit) => void; onEdit: (u: Unit) => void }) {
  const [filter, setFilter] = useState<"all" | "occupied" | "vacant" | "under_construction">("all");
  const filtered = filter === "all" ? units : units.filter((u) => u.status === filter);

  return (
    <>
      <div className="row-between" style={{ marginBottom: 12 }}>
        <div className="segmented">
          {(["all", "occupied", "vacant", "under_construction"] as const).map((f) => (
            <button key={f} className={`segmented__btn${filter === f ? " segmented__btn--active" : ""}`} onClick={() => setFilter(f)}>
              {f === "all" ? "All" : f === "occupied" ? "Occupied" : f === "vacant" ? "Vacant" : "Under construction"}
            </button>
          ))}
        </div>
        <button className="btn btn-primary" onClick={onAdd}><IconPlus size={16} /> Add unit</button>
      </div>
      {filtered.length === 0 ? (
        <Empty text="No units found" icon={<IconHome />} />
      ) : (
        <div className="stack">
          {filtered.map((u) => <UnitRow key={u.id} unit={u} onOpen={() => onOpen(u)} />)}
        </div>
      )}
    </>
  );
}

function UnitRow({ unit, onOpen }: { unit: Unit; onOpen: () => void }) {
  const statusTone = unit.status === "occupied" ? "badge--success" : unit.status === "vacant" ? "badge--muted" : "badge--warning";
  const statusLabel = unit.status === "occupied" ? "Occupied" : unit.status === "vacant" ? "Vacant" : "Under construction";
  return (
    <div className="row row--clickable" onClick={onOpen}>
      <span className="row__icon row__icon--primary"><IconHome /></span>
      <div className="row__body">
        <div className="row__title">Unit {unit.number} — Floor {unit.floor}</div>
        <div className="row__sub">{unit.area_m2 ? `${unit.area_m2} m²` : ""}{unit.rooms ? ` · ${unit.rooms} bed` : ""}</div>
      </div>
      <div className="row__end">
        <span className={`badge ${statusTone}`}>{statusLabel}</span>
      </div>
    </div>
  );
}

function AssetsTab({ parkings, warehouses, units, onAdd, onAssign, onRelease, onOpen }: {
  parkings: Asset[]; warehouses: Asset[]; units: Unit[];
  onAdd: (k: "parking" | "warehouse") => void;
  onAssign: (a: Asset) => void;
  onRelease: (a: Asset) => void;
  onOpen: (a: Asset) => void;
}) {
  const [tab, setTab] = useState<"parking" | "warehouse">("parking");
  const list = tab === "parking" ? parkings : warehouses;

  return (
    <>
      <div className="row-between" style={{ marginBottom: 12 }}>
        <div className="segmented">
          <button className={`segmented__btn${tab === "parking" ? " segmented__btn--active" : ""}`} onClick={() => setTab("parking")}>Parking ({parkings.length})</button>
          <button className={`segmented__btn${tab === "warehouse" ? " segmented__btn--active" : ""}`} onClick={() => setTab("warehouse")}>Storage ({warehouses.length})</button>
        </div>
        <button className="btn btn-primary" onClick={() => onAdd(tab)}><IconPlus size={16} /> Add {tab === "parking" ? "parking" : "storage"}</button>
      </div>
      {list.length === 0 ? (
        <Empty text={`No ${tab === "parking" ? "parking" : "storage"} slots yet`} icon={<IconBox />} />
      ) : (
        <div className="grid-3">
          {list.map((a) => (
            <div
              key={a.id}
              className="card row--clickable"
              style={{ padding: 12, display: "grid", gap: 4, cursor: "pointer" }}
              onClick={() => onOpen(a)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onOpen(a); } }}
            >
              <div style={{
                fontFamily: "JetBrains Mono, monospace", fontWeight: 800, fontSize: 14,
                color: a.available ? "var(--c-primary-strong)" : "var(--c-fg-2)",
              }}>{a.code}</div>
              <div className="tiny secondary">{a.area_m2} m² · Floor {a.floor}</div>
              <span className={`badge ${a.available ? "badge--success" : "badge--muted"}`} style={{ marginTop: 4 }}>
                {a.available ? "Available" : "Assigned"}
              </span>
              <div className="row" style={{ marginTop: 8, padding: 0, background: "transparent", border: 0, boxShadow: "none", gap: 6 }}>
                {a.available ? (
                  <button className="btn btn-sm" onClick={(e) => { e.stopPropagation(); onAssign(a); }}><IconArrowRight size={14} /> Assign</button>
                ) : (
                  <button className="btn btn-sm" onClick={(e) => { e.stopPropagation(); onRelease(a); }}>Release</button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

function MembersTab({ members }: { members: Membership[] }) {
  return (
    <div className="stack">
      {members.length === 0 && <Empty text="No members found" icon={<IconUsers />} />}
      {members.map((m) => (
        <div className="row" key={m.id}>
          <span className="row__icon row__icon--info"><IconUsers /></span>
          <div className="row__body">
            <div className="row__title">User {m.user_id.slice(0, 8)}</div>
            <div className="row__sub">Member since {new Intl.DateTimeFormat("en-US").format(new Date(m.from))}</div>
          </div>
          <div className="row__end"><RoleBadge role={m.role} /></div>
        </div>
      ))}
    </div>
  );
}

function ContractsTab({ contracts, units, onNew }: { contracts: Contract[]; units: Unit[]; onNew: () => void }) {
  const unitNum = (id: string) => units.find((u) => u.id === id)?.number || id.slice(0, 8);
  return (
    <>
      <div className="row-between" style={{ marginBottom: 12 }}>
        <div className="small secondary">{contracts.length} contract(s)</div>
        <button className="btn btn-primary" onClick={onNew}><IconPlus size={16} /> New contract</button>
      </div>
      {contracts.length === 0 ? (
        <Empty text="No contracts yet" icon={<IconFile />} />
      ) : (
        <div className="stack">
          {contracts.map((c) => (
            <div className="row" key={c.id}>
              <span className="row__icon row__icon--primary"><IconFile /></span>
              <div className="row__body">
                <div className="row__title">{c.number} · {c.type}</div>
                <div className="row__sub">Unit {unitNum(c.unit_id)} · {c.start_date?.slice(0, 10) || ""}</div>
              </div>
              <div className="row__end">
                <span className={`badge ${c.status === "active" ? "badge--success" : c.status === "draft" ? "badge--muted" : "badge--warning"}`}>
                  {c.status}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}

// ─── Modals ───────────────────────────────────────────────────

function EditBuildingModal({ building, onClose, onSaved }: { building: Building; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(building.name);
  const [type, setType] = useState<Building["type"]>(building.type);
  const [address, setAddress] = useState(building.address);
  const [floors, setFloors] = useState(String(building.floors || 1));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    setBusy(true); setErr("");
    try {
      await patchBuilding(building.id, { name, type, address, floors: Number(floors) || 1 });
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title="Edit building" onClose={onClose}>
      <div className="stack">
        <div className="field"><label>Name</label><input value={name} onChange={(e) => setName(e.target.value)} /></div>
        <div className="field"><label>Type</label>
          <select value={type} onChange={(e) => setType(e.target.value as Building["type"])}>
            <option value="residential">Residential</option><option value="commercial">Commercial</option>
            <option value="office">Office</option><option value="mixed">Mixed</option>
          </select>
        </div>
        <div className="field"><label>Address</label><input value={address} onChange={(e) => setAddress(e.target.value)} /></div>
        <div className="field"><label>Floors</label><input type="number" min="1" value={floors} onChange={(e) => setFloors(e.target.value)} /></div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Saving…" : "Save"}</button>
        </div>
      </div>
    </Modal>
  );
}

function AddUnitModal({ buildingId, onClose, onSaved }: { buildingId: string; onClose: () => void; onSaved: () => void }) {
  const [floor, setFloor] = useState("1");
  const [number, setNumber] = useState("");
  const [area, setArea] = useState("");
  const [rooms, setRooms] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    if (!number) { setErr("Unit number is required"); return; }
    setBusy(true); setErr("");
    try {
      await createUnit(buildingId, {
        floor: Number(floor) || 1, number,
        area_m2: area ? Number(area) : undefined,
        rooms: rooms ? Number(rooms) : undefined,
      });
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title="Add unit" onClose={onClose}>
      <div className="stack">
        <div className="field-row">
          <div className="field"><label>Floor</label><input type="number" min="0" value={floor} onChange={(e) => setFloor(e.target.value)} /></div>
          <div className="field"><label>Number</label><input value={number} onChange={(e) => setNumber(e.target.value.toUpperCase())} placeholder="101" /></div>
        </div>
        <div className="field-row">
          <div className="field"><label>Area (m²)</label><input type="number" value={area} onChange={(e) => setArea(e.target.value)} /></div>
          <div className="field"><label>Rooms</label><input type="number" value={rooms} onChange={(e) => setRooms(e.target.value)} /></div>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Saving…" : "Add"}</button>
        </div>
      </div>
    </Modal>
  );
}

function AddAssetModal({ buildingId, kind, onClose, onSaved }: { buildingId: string; kind: "parking" | "warehouse"; onClose: () => void; onSaved: () => void }) {
  const [code, setCode] = useState("");
  const [floor, setFloor] = useState("0");
  const [area, setArea] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    if (!code) { setErr("Code is required"); return; }
    setBusy(true); setErr("");
    try {
      await createAsset(buildingId, { kind, code, floor: Number(floor) || 0, area_m2: area ? Number(area) : undefined });
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title={`Add ${kind === "parking" ? "parking" : "storage"}`} onClose={onClose}>
      <div className="stack">
        <div className="field"><label>Code</label><input value={code} onChange={(e) => setCode(e.target.value.toUpperCase())} placeholder={kind === "parking" ? "P-101" : "S-A1"} /></div>
        <div className="field-row">
          <div className="field"><label>Floor</label><input type="number" value={floor} onChange={(e) => setFloor(e.target.value)} /></div>
          <div className="field"><label>Area (m²)</label><input type="number" value={area} onChange={(e) => setArea(e.target.value)} /></div>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Saving…" : "Add"}</button>
        </div>
      </div>
    </Modal>
  );
}

function AssignAssetModal({ asset, units, onClose, onSaved }: { asset: Asset; units: Unit[]; onClose: () => void; onSaved: () => void }) {
  const [unitId, setUnitId] = useState(units[0]?.id || "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    if (!unitId) { setErr("Pick a unit"); return; }
    setBusy(true); setErr("");
    try {
      await assignAsset(unitId, asset.kind, asset.code);
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title={`Assign ${asset.code}`} onClose={onClose}>
      <div className="stack">
        <div className="field"><label>Unit</label>
          <select value={unitId} onChange={(e) => setUnitId(e.target.value)}>
            {units.map((u) => <option key={u.id} value={u.id}>Unit {u.number} — Floor {u.floor}</option>)}
          </select>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Assigning…" : "Assign"}</button>
        </div>
      </div>
    </Modal>
  );
}

function NewContractModal({ buildingId, units, onClose, onSaved }: { buildingId: string; units: Unit[]; onClose: () => void; onSaved: () => void }) {
  const [type, setType] = useState<Contract["type"]>("rental");
  const [unitId, setUnitId] = useState(units[0]?.id || "");
  const [firstParty, setFirstParty] = useState("");
  const [secondParty, setSecondParty] = useState("");
  const [title, setTitle] = useState("");
  const [amount, setAmount] = useState("");
  const [deposit, setDeposit] = useState("");
  const [startDate, setStartDate] = useState(new Date().toISOString().slice(0, 10));
  const [duration, setDuration] = useState("12");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function save() {
    if (!unitId || !firstParty || !secondParty) { setErr("Unit and both parties are required"); return; }
    setBusy(true); setErr("");
    try {
      await createContract({
        type, unit_id: unitId, first_party_id: firstParty, second_party_id: secondParty,
        title: title || undefined,
        amount: amount ? Number(amount) : undefined,
        deposit_amount: deposit ? Number(deposit) : undefined,
        start_date: startDate,
        duration_months: Number(duration) || 0,
      });
      onSaved();
    } catch (e) { setErr((e as Error).message); }
    finally { setBusy(false); }
  }

  return (
    <Modal open title="New contract" onClose={onClose}>
      <div className="stack">
        <div className="field-row">
          <div className="field"><label>Type</label>
            <select value={type} onChange={(e) => setType(e.target.value as Contract["type"])}>
              <option value="rental">Rental</option>
              <option value="purchase">Purchase</option>
              <option value="transfer">Transfer</option>
            </select>
          </div>
          <div className="field"><label>Unit</label>
            <select value={unitId} onChange={(e) => setUnitId(e.target.value)}>
              {units.map((u) => <option key={u.id} value={u.id}>Unit {u.number}</option>)}
            </select>
          </div>
        </div>
        <div className="field-row">
          <div className="field"><label>First party (user id)</label><input value={firstParty} onChange={(e) => setFirstParty(e.target.value)} placeholder="uuid" /></div>
          <div className="field"><label>Second party (user id)</label><input value={secondParty} onChange={(e) => setSecondParty(e.target.value)} placeholder="uuid" /></div>
        </div>
        <div className="field"><label>Title</label><input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Optional" /></div>
        <div className="field-row">
          <div className="field"><label>Amount (Rial)</label><input type="number" value={amount} onChange={(e) => setAmount(e.target.value)} /></div>
          <div className="field"><label>Deposit (Rial)</label><input type="number" value={deposit} onChange={(e) => setDeposit(e.target.value)} /></div>
        </div>
        <div className="field-row">
          <div className="field"><label>Start date</label><input type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} /></div>
          <div className="field"><label>Duration (months)</label><input type="number" value={duration} onChange={(e) => setDuration(e.target.value)} /></div>
        </div>
        {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
        <div className="row" style={{ gap: 8, justifyContent: "flex-end" }}>
          <button className="btn" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" disabled={busy} onClick={save}>{busy ? "Creating…" : "Create contract"}</button>
        </div>
      </div>
    </Modal>
  );
}
