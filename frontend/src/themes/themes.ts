// Theme registry — Telegram-style. Each theme is a flat object of CSS
// custom properties. Adding a new theme means adding one entry here;
// no component changes required.

export interface Theme {
  id: string;
  name: string;
  /** "light" or "dark" — used for system-matching, also sets data-theme */
  mode: "light" | "dark";
  /** Order: first item is the default. */
  vars: Record<string, string>;
}

// ---------- Light (default) ----------
const light: Theme = {
  id: "light",
  name: "Light",
  mode: "light",
  vars: {
    "--c-bg": "#F5F3EF",
    "--c-card": "#FFFFFF",
    "--c-card-glass": "rgba(255, 255, 255, 0.72)",
    "--c-surface-alt": "#EDEAE4",
    "--c-overlay": "rgba(28, 27, 26, 0.42)",
    "--c-fg": "#1C1B1A",
    "--c-fg-2": "#6B6560",
    "--c-fg-3": "#9E9890",
    "--c-primary": "#1A7C6E",
    "--c-primary-strong": "#147066",
    "--c-primary-soft": "#E6F4F1",
    "--c-primary-ink": "#FFFFFF",
    "--c-accent": "#D4840A",
    "--c-accent-soft": "#FEF3D7",
    "--c-destructive": "#C0392B",
    "--c-destructive-soft": "#FCE8E5",
    "--c-success": "#2E7D4F",
    "--c-success-soft": "#DFF1E5",
    "--c-info": "#2D6CB7",
    "--c-info-soft": "#DDE9F7",
    "--c-warning": "#B7791F",
    "--c-warning-soft": "#FCEFC7",
    "--c-border": "#DDD9D3",
    "--c-border-strong": "#C9C4BC",
    "--c-border-subtle": "rgba(0, 0, 0, 0.04)",
    "--c-scrollbar": "rgba(28, 27, 26, 0.18)",
    "--c-scrollbar-hover": "rgba(28, 27, 26, 0.32)",
    "--c-toast-bg": "#1C1B1A",
    "--c-toast-fg": "#F8F6F1",
    "--c-pill-neutral-bg": "#EEEAE2",
    "--c-pill-neutral-fg": "#6B6560",
    "--c-pill-accent-fg": "#8A5500",
    "--c-badge-muted-bg": "#EEEAE2",
    "--c-badge-muted-fg": "#6B6560",
    "--c-badge-staff-bg": "#FFE9D6",
    "--c-badge-staff-fg": "#A04B00",
    "--c-badge-manager-bg": "#F1E6FA",
    "--c-badge-manager-fg": "#6D2BA0",
    "--shadow-card": "0 1px 2px rgba(28, 27, 26, 0.04), 0 8px 24px -12px rgba(28, 27, 26, 0.12)",
    "--shadow-float": "0 6px 20px -8px rgba(28, 27, 26, 0.18)",
    "--shadow-modal": "0 24px 64px -12px rgba(28, 27, 26, 0.4)",
    "--shadow-lift": "0 12px 32px -8px rgba(28, 27, 26, 0.18)",
    "--shadow-glow": "0 16px 40px -18px rgba(26, 124, 110, 0.55)",
  },
};

// ---------- Dark ----------
const dark: Theme = {
  id: "dark",
  name: "Dark",
  mode: "dark",
  vars: {
    "--c-bg": "#0F0F0E",
    "--c-card": "#1A1A19",
    "--c-card-glass": "rgba(26, 26, 25, 0.72)",
    "--c-surface-alt": "#25241F",
    "--c-overlay": "rgba(0, 0, 0, 0.62)",
    "--c-fg": "#E8E5DF",
    "--c-fg-2": "#A9A39A",
    "--c-fg-3": "#6F6A62",
    "--c-primary": "#2DBDA9",
    "--c-primary-strong": "#1F8F82",
    "--c-primary-soft": "#1A3530",
    "--c-primary-ink": "#0A1715",
    "--c-accent": "#E8A040",
    "--c-accent-soft": "#3A2C12",
    "--c-destructive": "#E15A4D",
    "--c-destructive-soft": "#3A1812",
    "--c-success": "#4CAF6A",
    "--c-success-soft": "#13261C",
    "--c-info": "#5B8FD6",
    "--c-info-soft": "#102035",
    "--c-warning": "#D4A03A",
    "--c-warning-soft": "#34260E",
    "--c-border": "#2A2926",
    "--c-border-strong": "#3B3A36",
    "--c-border-subtle": "rgba(255, 255, 255, 0.04)",
    "--c-scrollbar": "rgba(255, 255, 255, 0.14)",
    "--c-scrollbar-hover": "rgba(255, 255, 255, 0.28)",
    "--c-toast-bg": "#E8E5DF",
    "--c-toast-fg": "#1C1B1A",
    "--c-pill-neutral-bg": "#25241F",
    "--c-pill-neutral-fg": "#A9A39A",
    "--c-pill-accent-fg": "#E8C474",
    "--c-badge-muted-bg": "#25241F",
    "--c-badge-muted-fg": "#A9A39A",
    "--c-badge-staff-bg": "#3A2412",
    "--c-badge-staff-fg": "#E8B070",
    "--c-badge-manager-bg": "#251A35",
    "--c-badge-manager-fg": "#C29DE8",
    "--shadow-card": "0 1px 2px rgba(0, 0, 0, 0.4), 0 8px 24px -12px rgba(0, 0, 0, 0.5)",
    "--shadow-float": "0 6px 20px -8px rgba(0, 0, 0, 0.55)",
    "--shadow-modal": "0 24px 64px -12px rgba(0, 0, 0, 0.7)",
    "--shadow-lift": "0 12px 32px -8px rgba(0, 0, 0, 0.6)",
    "--shadow-glow": "0 16px 40px -18px rgba(45, 189, 169, 0.45)",
  },
};

// ---------- Registry ----------
export const themes: Theme[] = [light, dark];

/** Resolve a theme by id; falls back to light. */
export function getTheme(id: string | null | undefined): Theme {
  return themes.find((t) => t.id === id) ?? light;
}

/** Apply a theme's vars to a CSSStyleDeclaration-compatible target (e.g. document.documentElement.style). */
export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  // Always reset the data-theme attribute so token overrides hit.
  root.setAttribute("data-theme", theme.mode);
  for (const [k, v] of Object.entries(theme.vars)) {
    root.style.setProperty(k, v);
  }
}

/** Apply OS preference when "system" is selected. */
export function systemTheme(): "light" | "dark" {
  if (typeof window === "undefined") return "light";
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

/** Apply the right theme based on a "system" or explicit id. */
export function applyById(id: string | null | undefined): Theme {
  const resolved = id === "system" ? getTheme(systemTheme()) : getTheme(id);
  applyTheme(resolved);
  return resolved;
}
