import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { applyById, getTheme, themes, type Theme } from "./themes";

export type ThemeChoice = "light" | "dark" | "system";

interface ThemeContextValue {
  /** User's choice: light, dark, or system. */
  choice: ThemeChoice;
  /** The actual applied theme (resolved if choice is "system"). */
  theme: Theme;
  /** All available themes. */
  themes: Theme[];
  /** Update the choice. Pass "light" | "dark" | "system". */
  setChoice: (c: ThemeChoice) => void;
  /** Cycle light → dark → system → light. */
  cycle: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

const STORAGE_KEY = "unital_theme";

function readChoice(): ThemeChoice {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch { /* ignore */ }
  return "system";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [choice, setChoiceState] = useState<ThemeChoice>(readChoice);
  const [theme, setTheme] = useState<Theme>(() => applyById(readChoice()));

  // Re-resolve on system preference change when in "system" mode.
  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (choice === "system") setTheme(applyById("system"));
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [choice]);

  function setChoice(c: ThemeChoice) {
    try { localStorage.setItem(STORAGE_KEY, c); } catch { /* ignore */ }
    setChoiceState(c);
    setTheme(applyById(c));
  }

  function cycle() {
    const order: ThemeChoice[] = ["light", "dark", "system"];
    const next = order[(order.indexOf(choice) + 1) % order.length];
    setChoice(next);
  }

  return (
    <ThemeContext.Provider value={{ choice, theme, themes, setChoice, cycle }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside <ThemeProvider>");
  return ctx;
}
