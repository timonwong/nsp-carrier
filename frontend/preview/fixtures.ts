// Snapshots shaped like appcore.ViewSnapshot, one per UI state. Profiles
// mirror internal/host/profile.go so screenshots show real capabilities.
const MB = 1024 ** 2;
const GB = 1024 ** 3;

const profiles = [
  {
    id: "awoo",
    displayName: "Awoo",
    protocolFamily: "Awoo USB",
    supportedExtensions: [".nsp", ".nsz", ".xci", ".xcz"],
    verifiedVersions: ["1.6.2"],
  },
  {
    id: "goldleaf",
    displayName: "Goldleaf 0.10+",
    protocolFamily: "Goldleaf 0.10+",
    supportedExtensions: [".nsp"],
    verifiedVersions: ["1.2.0"],
  },
  {
    id: "sphaira",
    displayName: "Sphaira",
    protocolFamily: "Sphaira SPH0",
    supportedExtensions: [".nsp", ".nsz", ".xci", ".xcz", ".msp"],
    verifiedVersions: [],
  },
  {
    id: "dbi",
    displayName: "DBI",
    protocolFamily: "DBI0",
    supportedExtensions: [".nsp", ".nsz", ".xci", ".xcz"],
    verifiedVersions: [],
  },
].map((profile) => ({
  transport: "usb",
  wireNamespace: "flat-basename",
  filesystemAccess: "none",
  compatibleVersions: [],
  knownIncompatibleVersions: [],
  adapterAvailable: true,
  ...profile,
}));

const files = [
  { id: "1", name: "homebrew-menu-2.4.1.nsp", path: "~/Switch/Homebrew", size: 186 * MB },
  { id: "2", name: "demo-base-v0.nsp", path: "~/Switch/Library/Demo", size: 3.42 * GB },
  { id: "3", name: "demo-update-v196608.nsz", path: "~/Switch/Library/Demo", size: 812 * MB },
  { id: "4", name: "demo-dlc-pack.nsz", path: "~/Switch/Library/Demo/DLC", size: 1.18 * GB },
  { id: "5", name: "sample-cart.xci", path: "~/Switch/Carts", size: 5.9 * GB },
];

type Item = (typeof files)[number] & Record<string, unknown>;

function item(file: (typeof files)[number], patch: Record<string, unknown> = {}): Item {
  return {
    selected: true,
    conflict: false,
    status: "Queued",
    progress: 0,
    uniqueServedBytes: 0,
    wireBytes: 0,
    requested: false,
    validationErrors: [],
    ...file,
    ...patch,
  };
}

const log = (time: string, level: string, message: string) => ({ time, level, message });
const baseLogs = [
  log("15:59:34", "info", "Added 5 file(s)"),
  log("15:59:36", "info", "Installer profile set to DBI"),
];
const sessionLogs = [
  ...baseLogs,
  log("16:00:02", "info", "Waiting for DBI with 4 selected file(s)"),
  log("16:00:09", "info", "USB device connected"),
  log("16:00:10", "info", "Installer requested homebrew-menu-2.4.1.nsp"),
  log("16:00:15", "info", "homebrew-menu-2.4.1.nsp fully served"),
  log("16:00:15", "info", "Installer requested demo-base-v0.nsp"),
  log("16:00:16", "info", "Installer requested demo-update-v196608.nsz"),
];

function snapshot(patch: Record<string, unknown>) {
  const next = {
    state: "Idle",
    sessionId: "",
    items: [] as Item[],
    logs: [] as unknown[],
    activeProfile: "dbi",
    profiles,
    lastError: "",
    hasConflict: false,
    wireBytes: 0,
    ...patch,
  };
  const selected = next.items.filter((i) => i.selected);
  const validationErrors = selected.flatMap((i) => i.validationErrors as unknown[]);
  const requested = selected.filter((i) => i.status !== "Queued" && i.status !== "NotRequested");
  const requestedBytes = requested.reduce((sum, i) => sum + i.size, 0);
  const uniqueServedBytes = next.items.reduce((sum, i) => sum + (i.uniqueServedBytes as number), 0);
  return {
    ...next,
    selectedCount: selected.length,
    selectedBytes: selected.reduce((sum, i) => sum + i.size, 0),
    requestedBytes,
    uniqueServedBytes,
    overallProgress: requestedBytes ? (uniqueServedBytes / requestedBytes) * 100 : 0,
    validationErrors,
    canStart: next.state === "Idle" && selected.length > 0 && validationErrors.length === 0,
    canStop: next.state !== "Idle",
  };
}

const queued = files.map((f) => item(f));
const unsupported = (name: string) => ({
  sourceId: name,
  name,
  code: "unsupported-extension",
  message: `${name}: Goldleaf 0.10+ does not support ${name.slice(name.lastIndexOf("."))} files`,
});

export function servingStep(step: number) {
  const base = files[1].size * 0.42 + step * 19.2 * MB;
  return snapshot({
    state: "Serving",
    sessionId: "session-7f3a91c2",
    logs: sessionLogs,
    items: [
      item(files[0], { status: "FullyServed", progress: 100, uniqueServedBytes: files[0].size }),
      item(files[1], {
        status: "Serving",
        progress: (base / files[1].size) * 100,
        uniqueServedBytes: base,
        requested: true,
      }),
      item(files[2], { status: "Requested", requested: true }),
      item(files[3]),
      item(files[4], { selected: false }),
    ],
  });
}

const fixtures: Record<string, ReturnType<typeof snapshot>> = {
  empty: snapshot({}),
  ready: snapshot({ items: queued, logs: baseLogs }),
  invalid: snapshot({
    activeProfile: "goldleaf",
    logs: baseLogs,
    items: [
      item(files[0]),
      item(files[1]),
      item(files[2], { validationErrors: [unsupported(files[2].name)] }),
      item(files[3], { selected: false }),
      item(files[4], { validationErrors: [unsupported(files[4].name)] }),
    ],
  }),
  waiting: snapshot({
    state: "WaitingForDevice",
    sessionId: "session-7f3a91c2",
    items: queued,
    logs: sessionLogs.slice(0, 3),
  }),
  serving: servingStep(0),
  completed: snapshot({
    state: "Completed",
    sessionId: "session-7f3a91c2",
    logs: [...sessionLogs, log("16:02:44", "info", "Session completed")],
    items: [
      item(files[0], { status: "FullyServed", progress: 100, uniqueServedBytes: files[0].size }),
      item(files[1], { status: "FullyServed", progress: 100, uniqueServedBytes: files[1].size }),
      item(files[2], { status: "FullyServed", progress: 100, uniqueServedBytes: files[2].size }),
      item(files[3], { status: "NotRequested" }),
      item(files[4], { selected: false }),
    ],
  }),
  failed: snapshot({
    state: "Failed",
    sessionId: "session-7f3a91c2",
    lastError: "USB transfer failed: device disconnected during bulk write",
    logs: [
      ...sessionLogs.slice(0, 6),
      log("16:01:31", "error", "USB transfer failed: device disconnected during bulk write"),
    ],
    items: [
      item(files[0], { status: "FullyServed", progress: 100, uniqueServedBytes: files[0].size }),
      item(files[1], {
        status: "Interrupted",
        progress: 63.5,
        uniqueServedBytes: files[1].size * 0.635,
      }),
      item(files[2]),
      item(files[3]),
      item(files[4], { selected: false }),
    ],
  }),
  many: snapshot({
    logs: baseLogs,
    items: Array.from({ length: 24 }, (_, index) =>
      item(
        {
          ...files[index % files.length],
          id: `m${index}`,
          name: files[index % files.length].name.replace(/(\.\w+)$/, `-${index + 1}$1`),
        },
        { selected: index % 7 !== 3 },
      ),
    ),
  }),
};

export function fixture(state: string) {
  return fixtures[state] ?? fixtures.serving;
}

export const fixtureNames = Object.keys(fixtures);
