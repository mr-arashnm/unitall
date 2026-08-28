// Jira-style shell: top bar + collapsible sidebar + main content area.
// BottomNav stays for mobile. Theme toggle lives in the top bar.
import { NavLink, useLocation, useNavigate } from "react-router-dom";
import { useEffect, useState } from "react";
import {
  IconBuilding, IconHome, IconBuildings, IconCreditCard, IconWrench,
  IconBell, IconInbox, IconUser, IconLogout, IconSettings,
  IconChevronLeft, IconChevronRight, IconChevronDown, IconSearch,
  IconPlus, IconClipboard, IconChart, IconMegaphone, IconCalendar,
  IconTicket, IconLayout, IconDoor, IconStar,
} from "./icons";
import { fetchMyBuildings, session, logout } from "../api";
import type { Building } from "../api";
import { useTheme } from "../themes/ThemeContext";
import BottomNav from "./BottomNav";

// ── Small icon helpers (no extra component files) ──────────────
const SunIcon = (p: { size?: number }) => (
  <svg width={p.size ?? 16} height={p.size ?? 16} viewBox="0 0 24 24" fill="none"
    stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
  </svg>
);
const MoonIcon = (p: { size?: number }) => (
  <svg width={p.size ?? 16} height={p.size ?? 16} viewBox="0 0 24 24" fill="none"
    stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
  </svg>
);
const SystemIcon = (p: { size?: number }) => (
  <svg width={p.size ?? 16} height={p.size ?? 16} viewBox="0 0 24 24" fill="none"
    stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <rect x="2" y="3" width="20" height="14" rx="2" />
    <path d="M8 21h8M12 17v4" />
  </svg>
);
const MenuIcon = (p: { size?: number }) => (
  <svg width={p.size ?? 18} height={p.size ?? 18} viewBox="0 0 24 24" fill="none"
    stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <line x1="3" y1="6" x2="21" y2="6" />
    <line x1="3" y1="12" x2="21" y2="12" />
    <line x1="3" y1="18" x2="21" y2="18" />
  </svg>
);
const CloseIcon = (p: { size?: number }) => (
  <svg width={p.size ?? 18} height={p.size ?? 18} viewBox="0 0 24 24" fill="none"
    stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

// ── Sidebar nav items ─────────────────────────────────────────
const NAV_ITEMS = [
  { to: "/", Icon: IconHome, label: "Home", end: true },
  { to: "/buildings", Icon: IconBuildings, label: "Buildings" },
  { to: "/charges", Icon: IconCreditCard, label: "Charges" },
  { to: "/requests", Icon: IconWrench, label: "Requests" },
  { to: "/inbox", Icon: IconInbox, label: "Inbox" },
  { to: "/facilities", Icon: IconDoor, label: "Facilities" },
  { to: "/teams", Icon: IconUser, label: "Teams" },
  { to: "/announcements", Icon: IconMegaphone, label: "Announcements" },
  { to: "/meetings", Icon: IconCalendar, label: "Meetings" },
  { to: "/tickets", Icon: IconTicket, label: "Support" },
  { to: "/reports", Icon: IconChart, label: "Reports" },
];

const BOTTOM_ITEMS = [
  { to: "/profile", Icon: IconUser, label: "Profile" },
  { to: "/settings", Icon: IconSettings, label: "Settings" },
];

// ── Layout ─────────────────────────────────────────────────────
interface LayoutProps {
  children: React.ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { choice, cycle } = useTheme();
  const [collapsed, setCollapsed] = useState(false);
  const [buildings, setBuildings] = useState<Building[]>([]);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    fetchMyBuildings().then((r) => setBuildings(r.data));
  }, []);

  // Close mobile drawer on route change
  useEffect(() => {
    setMobileOpen(false);
  }, [location.pathname]);

  const user = session.user;
  const currentBuilding = buildings.find((b) => b.id === session.currentBuilding) ?? buildings[0];

  function handleLogout() {
    logout();
    navigate("/login", { replace: true });
  }

  const themeIcon = choice === "light" ? <SunIcon size={17} /> :
    choice === "dark" ? <MoonIcon size={17} /> :
    <SystemIcon size={17} />;

  return (
    <div className="app-shell">
      {/* ── Top bar ── */}
      <header className="topbar">
        {/* Mobile menu toggle */}
        <button
          className="btn-icon btn-icon--ghost"
          aria-label="Toggle menu"
          onClick={() => setMobileOpen((v) => !v)}
        >
          {mobileOpen ? <CloseIcon size={18} /> : <MenuIcon size={18} />}
        </button>

        {/* Logo */}
        <NavLink to="/" className="topbar__logo">
          <svg width="28" height="28" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <rect width="24" height="24" rx="8" fill="currentColor" opacity="0.15" />
            <path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z" fill="currentColor" />
            <path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2" stroke="white" strokeWidth="1.5" />
            <path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2" stroke="white" strokeWidth="1.5" />
          </svg>
          Unital
        </NavLink>

        {/* Desktop search */}
        <div className="search">
          <IconSearch size={15} />
          <input type="search" placeholder="Search…" aria-label="Search" />
        </div>

        <div className="topbar__spacer" />

        <div className="topbar__actions">
          {/* Notifications */}
          <button className="btn-icon btn-icon--ghost" aria-label="Notifications">
            <IconBell size={18} />
          </button>

          {/* Theme toggle */}
          <button
            className="btn-icon btn-icon--ghost"
            aria-label={`Theme: ${choice}. Click to cycle.`}
            title={`Theme: ${choice}`}
            onClick={cycle}
          >
            {themeIcon}
          </button>

          {/* User avatar + dropdown */}
          <div className="switcher">
            <button
              className="switcher__btn"
              onClick={() => navigate("/profile")}
              style={{ padding: "4px 8px" }}
              aria-label="User menu"
            >
              <span style={{
                width: 30, height: 30, borderRadius: "50%",
                background: "var(--c-primary)", color: "white",
                display: "grid", placeItems: "center",
                fontWeight: 700, fontSize: 12,
              }}>
                {user?.full_name?.charAt(0)?.toUpperCase() ?? "?"}
              </span>
              <span style={{ fontSize: 13.5, fontWeight: 600, maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {user?.full_name ?? "User"}
              </span>
              <IconChevronDown size={13} />
            </button>
          </div>

          {/* Logout */}
          <button
            className="btn-icon btn-icon--ghost"
            aria-label="Sign out"
            title="Sign out"
            onClick={handleLogout}
          >
            <IconLogout size={17} />
          </button>
        </div>
      </header>

      {/* ── Desktop Sidebar ── */}
      <aside className={`sidebar${collapsed ? " collapsed" : ""}`}>
        {/* Workspace / current building */}
        <div className="sidebar__section">
          {currentBuilding && (
            <div className="sidebar__building-item" style={{ marginBottom: 4 }}>
              <span className="sidebar__building-plate">{currentBuilding.code}</span>
              <span className="sidebar__building-name">{currentBuilding.name}</span>
            </div>
          )}
        </div>

        {/* Main nav */}
        <div className="sidebar__section">
          <div className="sidebar__section-label">Navigation</div>
          {NAV_ITEMS.map(({ to, Icon, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) => `sidebar__item${isActive ? " active" : ""}`}
              title={label}
            >
              <Icon size={18} />
              <span className="sidebar__item-label">{label}</span>
            </NavLink>
          ))}
        </div>

        <div className="sidebar__divider" />

        {/* Building list */}
        {buildings.length > 0 && (
          <div className="sidebar__section">
            <div className="sidebar__section-label">Buildings</div>
            {buildings.map((b) => (
              <button
                key={b.id}
                className={`sidebar__building-item${b.id === session.currentBuilding ? " active" : ""}`}
                onClick={() => {
                  session.currentBuilding = b.id;
                  navigate(`/buildings/${b.id}`);
                }}
                title={b.name}
              >
                <span className="sidebar__building-plate">{b.code}</span>
                <span className="sidebar__building-name">{b.name}</span>
              </button>
            ))}
            <button
              className="sidebar__item"
              onClick={() => navigate("/buildings/new")}
              title="Add building"
            >
              <IconPlus size={18} />
              <span className="sidebar__item-label">Add building</span>
            </button>
          </div>
        )}

        <div className="sidebar__divider" />

        {/* Bottom links */}
        <div className="sidebar__section">
          {BOTTOM_ITEMS.map(({ to, Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) => `sidebar__item${isActive ? " active" : ""}`}
              title={label}
            >
              <Icon size={18} />
              <span className="sidebar__item-label">{label}</span>
            </NavLink>
          ))}
        </div>

        {/* Collapse toggle */}
        <button
          className="sidebar__collapse"
          onClick={() => setCollapsed((v) => !v)}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          title={collapsed ? "Expand" : "Collapse"}
        >
          {collapsed
            ? <IconChevronRight size={17} />
            : <IconChevronLeft size={17} />
          }
        </button>
      </aside>

      {/* ── Mobile slide-in drawer ── */}
      {mobileOpen && (
        <div
          style={{
            position: "fixed", inset: 0, zIndex: 25,
            background: "rgba(0,0,0,0.5)",
            backdropFilter: "blur(4px)",
          }}
          onClick={() => setMobileOpen(false)}
          aria-hidden="true"
        />
      )}
      <aside
        className={`sidebar${collapsed ? " collapsed" : ""}`}
        style={{
          display: mobileOpen ? "flex" : undefined,
          position: "fixed",
          top: 0,
          zIndex: 35,
          width: mobileOpen ? "var(--sidebar-w)" : undefined,
          transform: mobileOpen ? "translateX(0)" : "translateX(-100%)",
          transition: "transform 200ms ease",
          ...(mobileOpen ? { height: "100vh", bottom: "auto" } : {}),
        }}
      >
        <div style={{ display: "flex", justifyContent: "flex-end", padding: "var(--sp-3)" }}>
          <button
            className="btn-icon btn-icon--ghost"
            onClick={() => setMobileOpen(false)}
            aria-label="Close menu"
          >
            <CloseIcon size={18} />
          </button>
        </div>
        <div className="sidebar__section">
          {currentBuilding && (
            <div className="sidebar__building-item" style={{ marginBottom: 4 }}>
              <span className="sidebar__building-plate">{currentBuilding.code}</span>
              <span className="sidebar__building-name">{currentBuilding.name}</span>
            </div>
          )}
        </div>
        <div className="sidebar__section">
          <div className="sidebar__section-label">Navigation</div>
          {NAV_ITEMS.map(({ to, Icon, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) => `sidebar__item${isActive ? " active" : ""}`}
              onClick={() => setMobileOpen(false)}
            >
              <Icon size={18} />
              <span className="sidebar__item-label">{label}</span>
            </NavLink>
          ))}
        </div>
        <div className="sidebar__divider" />
        <div className="sidebar__section">
          {BOTTOM_ITEMS.map(({ to, Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) => `sidebar__item${isActive ? " active" : ""}`}
              onClick={() => setMobileOpen(false)}
            >
              <Icon size={18} />
              <span className="sidebar__item-label">{label}</span>
            </NavLink>
          ))}
        </div>
      </aside>

      {/* ── Main content ── */}
      <main className="main-content" id="main">
        {children}
      </main>

      {/* ── Mobile bottom nav ── */}
      <BottomNav />
    </div>
  );
}
