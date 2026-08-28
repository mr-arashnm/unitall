// Login + register combined page (toggled internally).
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { login, register } from "../api";
import { IconBuilding, IconAlert } from "../components/icons";

export default function Login() {
  const nav = useNavigate();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!email || !password) { setError("Please enter email and password."); return; }
    if (mode === "register" && !name) { setError("Please enter your name."); return; }
    setBusy(true);
    setError("");
    try {
      if (mode === "register") {
        await register({ email: email.trim(), password, full_name: name.trim(), phone: phone.trim() || undefined });
      } else {
        await login(email.trim(), password);
      }
      nav("/", { replace: true });
    } catch (err: any) {
      const code = err?.code ?? "";
      if (mode === "login" && (code === "INVALID_CREDENTIALS" || code === "EMAIL_NOT_VERIFIED" || err?.status === 401 || err?.status === 403)) {
        setError("Invalid email or password. If you don't have an account, sign up below.");
      } else if (code === "EMAIL_TAKEN" || err?.status === 409) {
        setError("This email is already registered. Try signing in.");
      } else if (err?.message?.includes("fetch") || err?.message?.includes("network")) {
        setError("Cannot reach the server. Check that the API gateway is running.");
      } else {
        setError(err?.title || err?.detail || err?.message || "Something went wrong. Please try again.");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="auth">
      <form className="card auth__card" onSubmit={submit} noValidate>
        <div className="auth__logo"><IconBuilding size={32} /></div>
        <h1 className="page-title" style={{ margin: 0 }}>Unital</h1>
        <p className="auth__tagline">Smart management for your building</p>

        {mode === "register" && (
          <>
            <div className="field">
              <label htmlFor="name">Full name</label>
              <input id="name" value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Sara Ahmadi" required />
            </div>
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor="phone">Phone</label>
              <input id="phone" dir="ltr" value={phone} onChange={(e) => setPhone(e.target.value)} placeholder="+1 555 000 0000" />
            </div>
          </>
        )}

        <div className="field" style={{ marginTop: mode === "register" ? 12 : 0 }}>
          <label htmlFor="email">Email</label>
          <input id="email" type="email" inputMode="email" autoComplete="email" dir="ltr" placeholder="you@example.com" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="field" style={{ marginTop: 12 }}>
          <label htmlFor="password">Password</label>
          <input id="password" type="password" autoComplete={mode === "login" ? "current-password" : "new-password"} placeholder="••••••••" value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        {error && (
          <div className="banner banner--info" style={{ marginTop: 14, padding: "10px 12px" }}>
            <IconAlert />
            <span>{error}</span>
          </div>
        )}

        <button className="btn btn-primary btn-block" style={{ marginTop: 20 }} disabled={busy}>
          {busy ? "Please wait…" : mode === "login" ? "Sign in" : "Create account"}
        </button>

        <p className="auth__switch">
          {mode === "login" ? "Don't have an account?" : "Already have an account?"}{" "}
          <button type="button" onClick={() => { setMode(mode === "login" ? "register" : "login"); setError(""); }}>
            {mode === "login" ? "Sign up" : "Sign in"}
          </button>
        </p>
      </form>
    </div>
  );
}
