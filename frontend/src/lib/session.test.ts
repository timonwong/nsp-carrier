import { describe, expect, it } from "vitest";

import {
  ThroughputMeter,
  etaSeconds,
  formatBytes,
  formatDuration,
  itemStatusKey,
  sessionTone,
} from "./session";

const item = (patch: Record<string, unknown> = {}) => ({
  selected: true,
  status: "Queued",
  validationErrors: [] as unknown[],
  ...patch,
});

describe("itemStatusKey", () => {
  it("reports a selected item with validation errors as invalid, whatever its transfer status", () => {
    expect(itemStatusKey(item({ validationErrors: [{ code: "unsupported-extension" }] }))).toBe(
      "Invalid",
    );
  });

  it("keeps the transfer status for an unselected item with validation errors", () => {
    expect(
      itemStatusKey(
        item({ selected: false, validationErrors: [{ code: "unsupported-extension" }] }),
      ),
    ).toBe("Queued");
  });

  it("passes through known backend file states", () => {
    for (const status of [
      "Queued",
      "Requested",
      "Serving",
      "FullyServed",
      "NotRequested",
      "Interrupted",
      "Failed",
    ]) {
      expect(itemStatusKey(item({ status }))).toBe(status);
    }
  });

  it("falls back to Queued for an unknown backend state instead of rendering a raw enum", () => {
    expect(itemStatusKey(item({ status: "SomethingNew" }))).toBe("Queued");
  });
});

describe("sessionTone", () => {
  const snapshot = (state: string, validationErrors: unknown[] = []) => ({
    state,
    validationErrors,
  });

  it.each([
    ["WaitingForDevice", "live"],
    ["Connected", "live"],
    ["Serving", "live"],
    ["Completed", "ok"],
    ["Failed", "err"],
    ["Disconnected", "warn"],
    ["Stopping", "idle"],
    ["Idle", "idle"],
  ])("maps %s to %s", (state, tone) => {
    expect(sessionTone(snapshot(state))).toBe(tone);
  });

  it("warns while idle when the selection is blocked by validation", () => {
    expect(sessionTone(snapshot("Idle", [{ code: "unsupported-extension" }]))).toBe("warn");
  });
});

describe("ThroughputMeter", () => {
  it("returns zero until it has samples spanning at least one second", () => {
    const meter = new ThroughputMeter();
    expect(meter.bytesPerSecond()).toBe(0);
    meter.sample(0, 0);
    meter.sample(1000, 500);
    expect(meter.bytesPerSecond()).toBe(0);
  });

  it("averages over the sliding window", () => {
    const meter = new ThroughputMeter(5000);
    meter.sample(0, 0);
    meter.sample(10_000, 1000);
    meter.sample(30_000, 2000);
    expect(meter.bytesPerSecond()).toBe(15_000);
  });

  it("drops samples older than the window", () => {
    const meter = new ThroughputMeter(2000);
    meter.sample(0, 0);
    meter.sample(1_000_000, 1000);
    meter.sample(1_000_000, 4000);
    meter.sample(1_004_000, 6000);
    expect(meter.bytesPerSecond()).toBe(2000);
  });

  it("restarts when served bytes go backwards (new session)", () => {
    const meter = new ThroughputMeter();
    meter.sample(0, 0);
    meter.sample(50_000, 1000);
    meter.sample(0, 2000);
    expect(meter.bytesPerSecond()).toBe(0);
  });

  it("forgets everything on reset", () => {
    const meter = new ThroughputMeter();
    meter.sample(0, 0);
    meter.sample(50_000, 1000);
    meter.reset();
    expect(meter.bytesPerSecond()).toBe(0);
  });
});

describe("formatting", () => {
  it("formats bytes with binary units", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(186 * 1024 ** 2)).toBe("186 MB");
    expect(formatBytes(3.42 * 1024 ** 3)).toBe("3.42 GB");
    expect(formatBytes(Number.NaN)).toBe("0 B");
  });

  it("estimates remaining time only when the rate is known", () => {
    expect(etaSeconds(1000, 0)).toBeNull();
    expect(etaSeconds(0, 100)).toBeNull();
    expect(etaSeconds(1000, 100)).toBe(10);
  });

  it("formats durations as m:ss or h:mm:ss", () => {
    expect(formatDuration(9)).toBe("0:09");
    expect(formatDuration(69)).toBe("1:09");
    expect(formatDuration(3723)).toBe("1:02:03");
  });
});
