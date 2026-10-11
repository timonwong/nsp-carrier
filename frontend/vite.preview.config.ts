import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

// Serves the real App against fixture snapshots so UI states can be rendered
// and screenshotted without the Go backend or a Switch.
const root = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  root,
  plugins: [svelte()],
  resolve: {
    conditions: ["browser"],
    alias: [
      {
        find: /^.*\/bindings\/github\.com\/timonwong\/nsp-carrier\/desktopapp(\.js)?$/,
        replacement: join(root, "preview/desktopapp.ts"),
      },
      { find: /^@wailsio\/runtime$/, replacement: join(root, "preview/runtime.ts") },
      {
        find: /^real-wails-runtime$/,
        replacement: join(root, "node_modules/@wailsio/runtime/dist/index.js"),
      },
    ],
  },
  clearScreen: false,
  logLevel: "warn",
});
