import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Default build.outDir points at the Go embed directory for local dev
// (`npm run build -- --watch`). The Docker image overrides it with
// `--outDir /web-dist` and copies the result into internal/web/dist in stage 2.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
});
