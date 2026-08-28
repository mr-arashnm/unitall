import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    host: true,
    // dev proxy: same-origin /api calls hit the local gateway (18080 —
    // 8080 is taken by MinIO on this machine)
    proxy: { "/api": "http://localhost:18080" },
  },
  preview: {
    port: 5173,
    host: true,
    proxy: { "/api": "http://localhost:18080" },
  },
});
