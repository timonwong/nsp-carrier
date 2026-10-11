// Renders every UI state from preview/fixtures.ts to PNGs in preview/out/.
// Usage: npm run preview:shots (needs `npx playwright install chromium` once).
import { mkdir, rm } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
import { createServer } from "vite";

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const out = join(frontendRoot, "preview", "out");

const states = ["empty", "ready", "invalid", "waiting", "serving", "completed", "failed", "many"];
const shots: { state: string; theme: string; locale: string; activity: boolean }[] = [];
for (const state of states) shots.push({ state, theme: "light", locale: "en", activity: false });
for (const state of ["empty", "invalid", "serving", "completed"])
  shots.push({ state, theme: "dark", locale: "en", activity: false });
for (const state of ["invalid", "serving", "failed"])
  shots.push({ state, theme: "light", locale: "zh-CN", activity: false });
shots.push({ state: "failed", theme: "light", locale: "en", activity: true });
shots.push({ state: "serving", theme: "dark", locale: "en", activity: true });

const server = await createServer({
  configFile: join(frontendRoot, "vite.preview.config.ts"),
  server: { port: 0, strictPort: false },
});
await server.listen();
const base = server.resolvedUrls?.local[0];
if (!base) throw new Error("preview server did not report a URL");

await rm(out, { recursive: true, force: true });
await mkdir(out, { recursive: true });

const browser = await chromium.launch();
try {
  for (const shot of shots) {
    const context = await browser.newContext({
      viewport: { width: 1180, height: 760 },
      deviceScaleFactor: 2,
      colorScheme: shot.theme === "dark" ? "dark" : "light",
    });
    await context.addInitScript((s) => {
      localStorage.setItem("nsp-carrier-theme", s.theme);
      localStorage.setItem("nsp-carrier-locale", s.locale);
      localStorage.setItem("nsp-carrier-activity-open", s.activity ? "1" : "0");
    }, shot);
    const page = await context.newPage();
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (message.type() === "error") errors.push(message.text());
    });
    await page.goto(`${base}preview/index.html?state=${shot.state}`);
    await page.waitForSelector(".app-shell [role='radio']");
    await page.waitForTimeout(shot.state === "serving" ? 3200 : 300);
    const name = [shot.state, shot.theme, shot.locale, shot.activity ? "activity" : ""]
      .filter(Boolean)
      .join("-");
    await page.screenshot({ path: join(out, `${name}.png`), animations: "disabled" });
    await context.close();
    if (errors.length) throw new Error(`${name}: ${errors.join("; ")}`);
    console.log(`${name}.png`);
  }
} finally {
  await browser.close();
  await server.close();
}
