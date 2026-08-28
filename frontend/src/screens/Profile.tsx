// Profile — view and edit user profile.
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { session, logout, updateProfile } from "../api";
import { BackBtn, Toast } from "../components/ui";
import { IconUser, IconEdit, IconLogout } from "../components/icons";

export default function Profile() {
  const nav = useNavigate();
  const user = session.user!;
  const [name, setName] = useState(user.full_name);
  const [phone, setPhone] = useState(user.phone ?? "");
  const [national, setNational] = useState(user.national_code ?? "");
  const [editing, setEditing] = useState(false);
  const [toast, setToast] = useState("");

  async function save() {
    await updateProfile({ full_name: name, phone, national_code: national });
    setToast("Changes saved");
    setEditing(false);
  }

  function handleLogout() {
    logout();
    nav("/login", { replace: true });
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Profile</h1>
        <div style={{ width: 38 }} />
      </div>

      <div className="card" style={{ textAlign: "center", padding: 32, marginBottom: 16 }}>
        <div style={{
          width: 72, height: 72, borderRadius: "50%", background: "var(--c-primary)",
          color: "#fff", display: "grid", placeItems: "center",
          fontSize: 28, fontWeight: 800, margin: "0 auto 12px",
        }}>
          {name.charAt(0)}
        </div>
        <div style={{ fontWeight: 800, fontSize: 19 }}>{name}</div>
        <div className="secondary small" style={{ marginTop: 4 }}>{user.email}</div>
        <div className="chip" style={{ marginTop: 8 }}>{user.platform_role}</div>
      </div>

      <div className="card stack">
        <div style={{ fontWeight: 700, fontSize: 16, marginBottom: 4 }}>Personal information</div>
        <div className="field">
          <label>Full name</label>
          <input value={name} onChange={(e) => setName(e.target.value)} disabled={!editing} />
        </div>
        <div className="field">
          <label>Email</label>
          <input value={user.email} dir="ltr" disabled />
        </div>
        <div className="field">
          <label>Phone</label>
          <input value={phone} onChange={(e) => setPhone(e.target.value)} dir="ltr" disabled={!editing} />
        </div>
        <div className="field">
          <label>National ID</label>
          <input value={national} onChange={(e) => setNational(e.target.value)} dir="ltr" disabled={!editing} />
        </div>

        {editing ? (
          <div className="field-row" style={{ marginTop: 4 }}>
            <button className="btn btn-primary" onClick={save}>Save</button>
            <button className="btn btn-secondary" onClick={() => setEditing(false)}>Cancel</button>
          </div>
        ) : (
          <button className="btn btn-secondary btn-block" onClick={() => setEditing(true)}>
            <IconEdit size={16} /> Edit
          </button>
        )}
      </div>

      <div className="card stack" style={{ marginTop: 16 }}>
        <div style={{ fontWeight: 700, fontSize: 16, marginBottom: 8 }}>My buildings</div>
        <div className="small secondary">Manage your building memberships from the Buildings section.</div>
        <button className="btn btn-secondary btn-block" onClick={() => nav("/buildings")}>
          View buildings
        </button>
      </div>

      <button
        className="btn btn-secondary btn-block"
        style={{ marginTop: 24, color: "var(--c-destructive)", borderColor: "var(--c-destructive-soft)" }}
        onClick={handleLogout}
      >
        <IconLogout size={16} /> Sign out
      </button>

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}
