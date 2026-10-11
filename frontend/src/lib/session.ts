export type Theme = "auto" | "light" | "dark";

export type Tone = "idle" | "live" | "ok" | "err" | "warn";

export type ItemStatusKey =
  | "Queued"
  | "Requested"
  | "Serving"
  | "FullyServed"
  | "NotRequested"
  | "Interrupted"
  | "Failed"
  | "Invalid";

const fileStates = new Set<string>([
  "Queued",
  "Requested",
  "Serving",
  "FullyServed",
  "NotRequested",
  "Interrupted",
  "Failed",
]);

interface ItemLike {
  selected: boolean;
  status: string;
  validationErrors: unknown[] | null;
}

// A selected item that the active profile rejects blocks Start, so its
// validation outranks whatever transfer state the backend last reported.
export function itemStatusKey(item: ItemLike): ItemStatusKey {
  if (item.selected && (item.validationErrors?.length ?? 0) > 0) return "Invalid";
  return fileStates.has(item.status) ? (item.status as ItemStatusKey) : "Queued";
}

export function itemTone(key: ItemStatusKey): Tone {
  switch (key) {
    case "Serving":
      return "live";
    case "FullyServed":
      return "ok";
    case "Interrupted":
    case "Failed":
      return "err";
    case "Invalid":
      return "warn";
    default:
      return "idle";
  }
}

export function sessionTone(snapshot: { state: string; validationErrors: unknown[] | null }): Tone {
  switch (snapshot.state) {
    case "WaitingForDevice":
    case "Connected":
    case "Serving":
      return "live";
    case "Completed":
      return "ok";
    case "Failed":
      return "err";
    case "Disconnected":
      return "warn";
    case "Idle":
      return (snapshot.validationErrors?.length ?? 0) > 0 ? "warn" : "idle";
    default:
      return "idle";
  }
}

export function isSessionEnded(state: string): boolean {
  return state === "Completed" || state === "Failed";
}

// The backend reports cumulative unique bytes, not a rate, so the rate is
// derived from recent samples. Samples are taken on every snapshot and on a
// timer, which lets the rate fall to zero when the installer stalls.
export class ThroughputMeter {
  private samples: { bytes: number; at: number }[] = [];

  constructor(private readonly windowMs = 5000) {}

  sample(bytes: number, at: number): void {
    const last = this.samples[this.samples.length - 1];
    if (last && bytes < last.bytes) this.samples = [];
    this.samples.push({ bytes, at });
    while (this.samples.length > 2 && at - this.samples[1].at >= this.windowMs) {
      this.samples.shift();
    }
  }

  reset(): void {
    this.samples = [];
  }

  bytesPerSecond(): number {
    if (this.samples.length < 2) return 0;
    const first = this.samples[0];
    const last = this.samples[this.samples.length - 1];
    const elapsed = last.at - first.at;
    if (elapsed < 1000) return 0;
    return ((last.bytes - first.bytes) * 1000) / elapsed;
  }
}

export function etaSeconds(remainingBytes: number, bytesPerSecond: number): number | null {
  if (bytesPerSecond <= 0 || remainingBytes <= 0) return null;
  return Math.round(remainingBytes / bytesPerSecond);
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  const amount = value / 1024 ** index;
  const digits = index === 0 || amount >= 100 ? 0 : amount >= 10 ? 1 : 2;
  return `${amount.toFixed(digits)} ${units[index]}`;
}

export function formatDuration(totalSeconds: number): string {
  const seconds = Math.max(0, Math.round(totalSeconds));
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = String(seconds % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`;
}

export function extensionOf(name: string): string {
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot + 1).toUpperCase() : "";
}

export function interpolate(template: string, values: Record<string, string | number>): string {
  return template.replace(/\{(\w+)\}/g, (_, key: string) => String(values[key] ?? ""));
}
