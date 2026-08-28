// Lucide line icons as inline React components. No emoji — visual restraint.
import type { SVGProps } from "react";

type P = SVGProps<SVGSVGElement> & { size?: number };

function Base({ size = 24, ...rest }: P) {
  return {
    width: size,
    height: size,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.75,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
    ...rest,
  };
}

export const IconBuilding = (p: P) => (
  <svg {...Base(p)}><path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z" /><path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2" /><path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2" /><path d="M10 6h4M10 10h4M10 14h4M10 18h4" /></svg>
);
export const IconBuildings = (p: P) => (
  <svg {...Base(p)}><path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z" /><path d="M6 12H4a2 2 0 0 0-2 2v8h20v-8a2 2 0 0 0-2-2h-2" /><path d="M10 6h4M10 10h4M10 14h4M10 18h4" /></svg>
);
export const IconCreditCard = (p: P) => (
  <svg {...Base(p)}><rect x="2" y="5" width="20" height="14" rx="3" /><path d="M2 10h20" /></svg>
);
export const IconWrench = (p: P) => (
  <svg {...Base(p)}><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z" /></svg>
);
export const IconBell = (p: P) => (
  <svg {...Base(p)}><path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" /><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" /></svg>
);
export const IconHome = (p: P) => (
  <svg {...Base(p)}><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" /><polyline points="9 22 9 12 15 12 15 22" /></svg>
);
export const IconLogout = (p: P) => (
  <svg {...Base(p)}><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" /><polyline points="16 17 21 12 16 7" /><line x1="21" y1="12" x2="9" y2="12" /></svg>
);
export const IconCheck = (p: P) => (
  <svg {...Base(p)}><polyline points="20 6 9 17 4 12" /></svg>
);
export const IconClock = (p: P) => (
  <svg {...Base(p)}><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg>
);
export const IconAlert = (p: P) => (
  <svg {...Base(p)}><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" /><path d="M12 9v4M12 17h.01" /></svg>
);
export const IconSend = (p: P) => (
  <svg {...Base(p)}><path d="m22 2-7 20-4-9-9-4Z" /><path d="M22 2 11 13" /></svg>
);
export const IconCalendar = (p: P) => (
  <svg {...Base(p)}><rect x="3" y="4" width="18" height="18" rx="2" /><path d="M16 2v4M8 2v4M3 10h18" /></svg>
);
export const IconCar = (p: P) => (
  <svg {...Base(p)}><path d="M19 17h2c.6 0 1-.4 1-1v-3c0-.9-.7-1.7-1.5-1.9C18.7 10.6 16 10 16 10s-1.3-1.4-2.2-2.3c-.5-.4-1.1-.7-1.8-.7H5c-.6 0-1.1.4-1.4.9l-1.4 2.9A3.7 3.7 0 0 0 2 12v4c0 .6.4 1 1 1h2" /><circle cx="7" cy="17" r="2" /><path d="M9 17h6" /><circle cx="17" cy="17" r="2" /></svg>
);
export const IconBox = (p: P) => (
  <svg {...Base(p)}><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z" /><path d="m3.3 7 8.7 5 8.7-5M12 22V12" /></svg>
);
export const IconUsers = (p: P) => (
  <svg {...Base(p)}><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" /><circle cx="9" cy="7" r="4" /><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" /></svg>
);
export const IconInbox = (p: P) => (
  <svg {...Base(p)}><polyline points="22 12 16 12 14 15 10 15 8 12 2 12" /><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" /></svg>
);
export const IconPlus = (p: P) => (
  <svg {...Base(p)}><line x1="12" y1="5" x2="12" y2="19" /><line x1="5" y1="12" x2="19" y2="12" /></svg>
);
export const IconChevronLeft = (p: P) => (
  <svg {...Base(p)}><polyline points="15 18 9 12 15 6" /></svg>
);
export const IconChevronRight = (p: P) => (
  <svg {...Base(p)}><polyline points="9 18 15 12 9 6" /></svg>
);
export const IconChevronDown = (p: P) => (
  <svg {...Base(p)}><polyline points="6 9 12 15 18 9" /></svg>
);
export const IconSearch = (p: P) => (
  <svg {...Base(p)}><circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /></svg>
);
export const IconFilter = (p: P) => (
  <svg {...Base(p)}><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" /></svg>
);
export const IconSettings = (p: P) => (
  <svg {...Base(p)}><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
);
export const IconUser = (p: P) => (
  <svg {...Base(p)}><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></svg>
);
export const IconKey = (p: P) => (
  <svg {...Base(p)}><path d="m21 2-9.6 9.6" /><circle cx="8" cy="16" r="6" /><path d="m15 8 4-4" /><path d="m18 5 3 3" /></svg>
);
export const IconChart = (p: P) => (
  <svg {...Base(p)}><line x1="12" y1="20" x2="12" y2="10" /><line x1="18" y1="20" x2="18" y2="4" /><line x1="6" y1="20" x2="6" y2="16" /></svg>
);
export const IconClipboard = (p: P) => (
  <svg {...Base(p)}><path d="M9 2h6a1 1 0 0 1 1 1v2a1 1 0 0 1-1 1H9a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1z" /><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2" /></svg>
);
export const IconMegaphone = (p: P) => (
  <svg {...Base(p)}><path d="m3 11 18-5v12L3 14v-3z" /><path d="M11.6 16.8a3 3 0 1 1-5.8-1.6" /></svg>
);
export const IconShield = (p: P) => (
  <svg {...Base(p)}><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" /></svg>
);
export const IconPool = (p: P) => (
  <svg {...Base(p)}><path d="M2 20a4 4 0 0 0 4-4 4 4 0 0 1 4-4 4 4 0 0 1 4 4 4 4 0 0 0 4 4 4 4 0 0 0 4-4 4 4 0 0 0-4-4 4 4 0 0 1-4-4 4 4 0 0 0-4-4 4 4 0 0 0-4 4 4 4 0 0 1-4 4 4 4 0 0 0-4 4 4 4 0 0 0 4 4z" /><path d="M6 12V5a3 3 0 0 1 6 0" /></svg>
);
export const IconDumbbell = (p: P) => (
  <svg {...Base(p)}><path d="m6.5 6.5 11 11" /><path d="m21 21-1-1" /><path d="m3 3 1 1" /><path d="m18 22 4-4" /><path d="m2 6 4-4" /><path d="m3 10 7-7" /><path d="m14 21 7-7" /></svg>
);
export const IconDoor = (p: P) => (
  <svg {...Base(p)}><path d="M4 22V4a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v18Z" /><path d="M2 22h20" /><path d="M14 12h.01" /></svg>
);
export const IconMessage = (p: P) => (
  <svg {...Base(p)}><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" /></svg>
);
export const IconTicket = (p: P) => (
  <svg {...Base(p)}><path d="M2 9a3 3 0 0 1 0 6v2a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-2a3 3 0 0 1 0-6V7a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2Z" /><path d="M13 5v2M13 17v2M13 11v2" /></svg>
);
export const IconList = (p: P) => (
  <svg {...Base(p)}><line x1="8" y1="6" x2="21" y2="6" /><line x1="8" y1="12" x2="21" y2="12" /><line x1="8" y1="18" x2="21" y2="18" /><line x1="3" y1="6" x2="3.01" y2="6" /><line x1="3" y1="12" x2="3.01" y2="12" /><line x1="3" y1="18" x2="3.01" y2="18" /></svg>
);
export const IconLayout = (p: P) => (
  <svg {...Base(p)}><rect x="3" y="3" width="18" height="18" rx="2" /><line x1="3" y1="9" x2="21" y2="9" /><line x1="9" y1="21" x2="9" y2="9" /></svg>
);
export const IconRefresh = (p: P) => (
  <svg {...Base(p)}><polyline points="23 4 23 10 17 10" /><polyline points="1 20 1 14 7 14" /><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" /></svg>
);
export const IconMoreV = (p: P) => (
  <svg {...Base(p)}><circle cx="12" cy="12" r="1" /><circle cx="12" cy="5" r="1" /><circle cx="12" cy="19" r="1" /></svg>
);
export const IconEdit = (p: P) => (
  <svg {...Base(p)}><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" /><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" /></svg>
);
export const IconTrash = (p: P) => (
  <svg {...Base(p)}><polyline points="3 6 5 6 21 6" /><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" /></svg>
);
export const IconFile = (p: P) => (
  <svg {...Base(p)}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><polyline points="14 2 14 8 20 8" /></svg>
);
export const IconExternal = (p: P) => (
  <svg {...Base(p)}><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" /><polyline points="15 3 21 3 21 9" /><line x1="10" y1="14" x2="21" y2="3" /></svg>
);
export const IconStar = (p: P) => (
  <svg {...Base(p)}><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" /></svg>
);
export const IconPhone = (p: P) => (
  <svg {...Base(p)}><path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72c.13.96.37 1.9.72 2.81a2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45c.91.35 1.85.59 2.81.72A2 2 0 0 1 22 16.92z" /></svg>
);
export const IconX = (p: P) => (
  <svg {...Base(p)}><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
);
export const IconChevronUp = (p: P) => (
  <svg {...Base(p)}><polyline points="18 15 12 9 6 15" /></svg>
);
export const IconArrowRight = (p: P) => (
  <svg {...Base(p)}><line x1="5" y1="12" x2="19" y2="12" /><polyline points="12 5 19 12 12 19" /></svg>
);
export const IconLocation = (p: P) => (
  <svg {...Base(p)}><path d="M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0z" /><circle cx="12" cy="10" r="3" /></svg>
);
