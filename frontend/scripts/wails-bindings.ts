import { spawnSync } from "node:child_process";
import { chmod, readdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const projectRoot = dirname(frontendRoot);
const generatedRoot = join(frontendRoot, "wailsjs");
const formatter = join(frontendRoot, "node_modules", "oxfmt", "bin", "oxfmt");

function run(command: string, args: string[], cwd: string): void {
  const result = spawnSync(command, args, { cwd, stdio: "inherit" });
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`${command} exited with status ${result.status ?? "unknown"}`);
  }
}

function generatedStatus(): string {
  const result = spawnSync(
    "git",
    ["status", "--porcelain=v1", "--untracked-files=all", "--", "frontend/wailsjs"],
    { cwd: projectRoot, encoding: "utf8" },
  );
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(result.stderr.trim() || "git status failed");
  }
  return result.stdout.trim();
}

async function normalizeModes(directory: string): Promise<void> {
  const entries = await readdir(directory, { withFileTypes: true });
  await Promise.all(
    entries.map(async (entry) => {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        await normalizeModes(path);
      } else if (entry.isFile()) {
        await chmod(path, 0o644);
      }
    }),
  );
}

async function generateBindings(): Promise<void> {
  run(process.env.WAILS_BIN || "wails", ["generate", "module"], projectRoot);
  run(process.execPath, [formatter, "wailsjs"], frontendRoot);
  await normalizeModes(generatedRoot);
}

async function main(): Promise<void> {
  const command = process.argv[2];
  if (command !== "generate" && command !== "check") {
    throw new Error("usage: wails-bindings.ts <generate|check>");
  }

  if (command === "check") {
    const before = generatedStatus();
    if (before) {
      throw new Error(
        "binding freshness check refused: frontend/wailsjs already has worktree changes",
      );
    }
  }

  await generateBindings();

  if (command === "check" && generatedStatus()) {
    throw new Error(
      "Wails bindings are stale; canonical regenerated changes were left in frontend/wailsjs",
    );
  }
}

await main();
