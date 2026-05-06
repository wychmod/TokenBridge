import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const isWailsDev = process.env.VITE_WAILS_DEV === "1";

export default defineConfig({
  base: process.env.VITE_BASE || "/admin/",
  plugins: [
    react(),
    {
      name: "localgateway-wails-api-fallback",
      enforce: "pre",
      configureServer(server) {
        if (!isWailsDev) {
          return;
        }
        server.middlewares.use((req, res, next) => {
          const path = req.url ?? "";
          if (path.includes("/admin/api") || path.startsWith("/api")) {
            res.statusCode = 404;
            res.end();
            return;
          }
          next();
        });
      }
    }
  ],
  server: {
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
    proxy: isWailsDev
      ? undefined
      : {
          "/admin/api": {
            target: "http://127.0.0.1:18743",
            changeOrigin: true
          }
        }
  }
});
