// Service requests — list + submit new request.
import { useState } from "react";
import { fetchServiceRequests, submitServiceRequest, session } from "../api";
import type { ServiceRequest } from "../api";
import { BackBtn, TaskStatusBadge, PriorityBadge, Empty, Toast, Modal } from "../components/ui";
import { IconSend, IconWrench, IconInbox, IconPlus } from "../components/icons";

const TYPE_LABELS: Record<string, string> = {
  maintenance: "Maintenance", cleaning: "Cleaning", security: "Security",
  complaint: "Complaint", suggestion: "Suggestion", other: "Other",
};

export default function Requests() {
  const [requests, setRequests] = useState<ServiceRequest[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [toast, setToast] = useState("");

  const [type, setType] = useState<ServiceRequest["type"]>("maintenance");
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [priority, setPriority] = useState<ServiceRequest["priority"]>("medium");

  useState(() => {
    const bid = session.currentBuilding;
    if (bid) fetchServiceRequests(bid).then(setRequests);
  });

  function submitForm(e: React.FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    const newReq: ServiceRequest = {
      id: `sr_${Date.now()}`, building_id: session.currentBuilding ?? "b1",
      unit_id: "u1", submitted_by: session.user?.id ?? "guest",
      title: title.trim(), description: desc.trim(), type, priority,
      status: "submitted", submitted_at: new Date().toISOString(),
    };
    setRequests((r) => [newReq, ...r]);
    setTitle(""); setDesc("");
    setShowForm(false);
    setToast("Your request has been submitted and will be routed to the relevant team.");
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Requests</h1>
        <button className="btn btn-primary btn-sm" onClick={() => setShowForm(true)}><IconPlus size={15} /></button>
      </div>

      {requests.length === 0 && (
        <div className="card" style={{ textAlign: "center", padding: 32 }}>
          <div className="row__icon row__icon--muted" style={{ margin: "0 auto 12px" }}><IconInbox /></div>
          <h3 style={{ margin: "0 0 6px" }}>No requests yet</h3>
          <p className="secondary small">Submit a new request to get started.</p>
          <button className="btn btn-primary" style={{ marginTop: 16 }} onClick={() => setShowForm(true)}><IconPlus size={16} /> New request</button>
        </div>
      )}

      <div className="stack">
        {requests.map((r) => (
          <div className="card" key={r.id} style={{ display: "grid", gap: 6 }}>
            <div className="row-between">
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <PriorityBadge priority={r.priority} />
                <span className="pill pill--neutral" style={{ fontSize: 11 }}>{TYPE_LABELS[r.type]}</span>
              </div>
              <TaskStatusBadge status={r.status} />
            </div>
            <div style={{ fontWeight: 700, fontSize: 15 }}>{r.title}</div>
            {r.description && <div className="small secondary">{r.description}</div>}
            <div className="tiny secondary">
              Submitted: {r.submitted_at ? new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long" }).format(new Date(r.submitted_at)) : "—"}
              {r.assigned_team ? ` · Team: ${r.assigned_team}` : ""}
            </div>
          </div>
        ))}
      </div>

      <Modal open={showForm} onClose={() => setShowForm(false)} title="New request">
        <form className="stack" onSubmit={submitForm} noValidate>
          <div className="field">
            <label>Type</label>
            <select value={type} onChange={(e) => setType(e.target.value as ServiceRequest["type"])}>
              {Object.entries(TYPE_LABELS).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
            </select>
          </div>
          <div className="field">
            <label>Priority</label>
            <select value={priority} onChange={(e) => setPriority(e.target.value as ServiceRequest["priority"])}>
              <option value="low">Low</option><option value="medium">Medium</option>
              <option value="high">High</option><option value="urgent">Urgent</option>
            </select>
          </div>
          <div className="field">
            <label>Title</label>
            <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="e.g. Kitchen pipe leaking" required />
          </div>
          <div className="field">
            <label>Description</label>
            <textarea rows={3} value={desc} onChange={(e) => setDesc(e.target.value)} placeholder="More details…" />
          </div>
          <button className="btn btn-primary btn-block" disabled={!title.trim()}><IconSend size={16} /> Submit</button>
        </form>
      </Modal>

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}
