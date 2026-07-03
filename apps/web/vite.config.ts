import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  envPrefix: ["VITE_", "WEB_"],
  plugins: [react()],
  server: {
    port: 5173,
  },
});
