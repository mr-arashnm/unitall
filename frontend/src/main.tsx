import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { ThemeProvider } from "./themes/ThemeContext";
import "./styles.css";

// Wipe any stale demo flag from a previous session — the user must
// explicitly log in with real credentials from now on.
localStorage.removeItem("unital_demo");
sessionStorage.removeItem("unital_demo_reason");

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </StrictMode>,
);
