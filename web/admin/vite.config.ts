import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: process.env.VITE_BASE || "/admin/",
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
    proxy: {
      "/admin/api": {
        target: "http://127.0.0.1:18743",
        changeOrigin: true
      }
    }
  }
});
