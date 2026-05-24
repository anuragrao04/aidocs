import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    proxy: {
      "/v1": "http://localhost:8080",
      "/openapi.json": "http://localhost:8080",
      "/api-docs": "http://localhost:8080",
    },
  },
});
