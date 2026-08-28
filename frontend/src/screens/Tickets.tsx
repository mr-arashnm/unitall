// Support tickets — list + open new ticket.
import { useEffect, useState } from "react";
import { fetchTickets, openTicket, fetchUnits, session } from "../api";
import type { Ticket, Unit } from "../api";
import { BackBtn, TaskStatusBadge, PriorityBadge, Empty, Toast, Modal } from "../components/ui";
import { IconTicket, IconPlus, IconSend } from "../components/icons";

const TYPE_LABELS: Record<string, string> = {
  technical: "Technical", financial: "Financial", complaint: "Complaint", suggestion: "Suggestion", general: "General",
};

export default function Tickets() {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [units, setUnits] = useState<Unit[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [toast, setToast] = useState("");
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [type, setType] = useState<Ticket["type"]>("general");
  const [priority, setPriority] = useState<Ticket["priority"]>("medium");
  const [unitId, setUnitId] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  async function reload() {
    const bid = session.currentBuilding;
    if (!bid) return;
    setTickets(await fetchTickets(bid));
    const us = await fetchUnits(bid);
    setUnits(us);
    if (us.length > 0 && !unitId) setUnitId(us[0].id);
  }

  useEffect(() => {
    reload().catch((e) => setToast((e as Error).message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    if (!unitId) { setErr("Pick a unit"); return; }
    setBusy(true); setErr("");
    try {
      await openTicket({
        unit_id: unitId,
        title: title.trim(),
        description: desc.trim() || undefined,
        type,
        priority,
      });
      setTitle(""); setDesc("");
      setShowForm(false);
      setToast("Ticket submitted. We'll respond shortly.");
      await reload();
    } catch (ex) {
      setErr((ex as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Support tickets</h1>
        <button className="btn btn-primary btn-sm" onClick={() => setShowForm(true)}><IconPlus size={15} /></button>
      </div>

      {tickets.length === 0 && (
        <div className="card" style={{ textAlign: "center", padding: 32 }}>
          <div className="row__icon row__icon--muted" style={{ margin: "0 auto 12px" }}><IconTicket /></div>
          <h3 style={{ margin: "0 0 6px" }}>No tickets yet</h3>
          <p className="secondary small">Have a problem or suggestion? Submit a ticket.</p>
          <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => setShowForm(true)}>
            <IconPlus size={16} /> New ticket
          </button>
        </div>
      )}

      <div className="stack">
        {tickets.map((t) => (
          <div key={t.id} className="card" style={{ display: "grid", gap: 6 }}>
            <div className="row-between">
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <PriorityBadge priority={t.priority} />
                <span className="pill pill--neutral" style={{ fontSize: 11 }}>{TYPE_LABELS[t.type] || t.type}</span>
              </div>
              <TaskStatusBadge status={t.status} />
            </div>
            <div style={{ fontWeight: 700, fontSize: 15 }}>{t.title}</div>
            {t.description && <div className="small secondary">{t.description}</div>}
            <div className="tiny secondary">
              Submitted: {t.submitted_at ? new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long" }).format(new Date(t.submitted_at)) : "—"}
              {t.assigned_to ? ` · Assigned: ${t.assigned_to}` : ""}
            </div>
          </div>
        ))}
      </div>

      <Modal open={showForm} onClose={() => { if (!busy) { setShowForm(false); setErr(""); } }} title="New ticket">
        <form className="stack" onSubmit={submit} noValidate>
          <div className="field">
            <label>Unit</label>
            <select value={unitId} onChange={(e) => setUnitId(e.target.value)}>
              {units.length === 0 && <option value="">— no units in this building —</option>}
              {units.map((u) => (
                <option key={u.id} value={u.id}>Unit {u.number} · Floor {u.floor}</option>
              ))}
            </select>
          </div>
          <div className="field">
            <label>Type</label>
            <select value={type} onChange={(e) => setType(e.target.value as Ticket["type"])}>
              {Object.entries(TYPE_LABELS).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
          </div>
          <div className="field">
            <label>Priority</label>
            <select value={priority} onChange={(e) => setPriority(e.target.value as Ticket["priority"])}>
              <option value="low">Low</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
              <option value="urgent">Urgent</option>
            </select>
          </div>
          <div className="field">
            <label>Title</label>
            <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Brief summary…" required />
          </div>
          <div className="field">
            <label>Description</label>
            <textarea rows={4} value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="More details…" />
          </div>
          {err && <div className="small" style={{ color: "var(--c-danger, #c33)" }}>{err}</div>}
          <button className="btn btn-primary btn-block" disabled={!title.trim() || busy}>
            <IconSend size={16} /> {busy ? "Submitting…" : "Submit ticket"}
          </button>
        </form>
      </Modal>

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}
