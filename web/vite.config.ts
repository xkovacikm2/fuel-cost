import { defineConfig } from "vitest/config";

export default defineConfig({
  root: ".",
  server: {
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/up": "http://localhost:8080",
      "/manifest": "http://localhost:8080",
      "/service-worker": "http://localhost:8080",
    },
  },
  build: {
    outDir: "../server/internal/webui/dist",
    emptyOutDir: true,
  },
  test: {
    environment: "jsdom",
    clearMocks: true,
  },
});
