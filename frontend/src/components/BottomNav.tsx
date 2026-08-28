// Bottom navigation — 5 primary destinations for the mobile app.
import { NavLink } from "react-router-dom";
import { IconHome, IconCreditCard, IconWrench, IconBell, IconBuildings } from "./icons";

const items = [
  { to: "/", label: "Home", Icon: IconHome, end: true },
  { to: "/buildings", label: "Buildings", Icon: IconBuildings },
  { to: "/charges", label: "Charges", Icon: IconCreditCard },
  { to: "/requests", label: "Requests", Icon: IconWrench },
  { to: "/inbox", label: "Inbox", Icon: IconBell },
];

export default function BottomNav() {
  return (
    <nav className="bottom-nav" aria-label="Primary navigation">
      {items.map(({ to, label, Icon, end }) => (
        <NavLink key={to} to={to} end={end} className={({ isActive }) => `nav-item${isActive ? " active" : ""}`}>
          <Icon />
          {label}
        </NavLink>
      ))}
    </nav>
  );
}
