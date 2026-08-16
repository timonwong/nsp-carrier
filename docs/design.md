# Design

## Product boundary

`nsp-carrier` is a macOS-first, cross-platform-structured Omni host for NS
installers. It exposes selected local files through an explicitly selected
installer profile.

It is not:

- an MTP implementation;
- a Switch-side installer;
- able to claim that installation succeeded;
- initially a public distribution.

The initial target is `darwin/arm64` on the development Mac. Windows and Linux
remain architectural targets, not validated platforms.

## Architecture

```text
cmd/usb-spike       retained developer diagnostics CLI
internal/host       deep host-session runner, profile registry, and capabilities
internal/dbi        DBI0 protocol adapter and codec
internal/awoo       Awoo USB protocol adapter and codec
internal/goldleaf   Goldleaf 0.10+ read-only virtual-drive adapter and codec
internal/files      queue discovery and protocol-neutral frozen source catalog
internal/transport  USB-independent stream boundary and fake transport
internal/usb        gousb/libusb adapter
internal/app        queue/session orchestration and frontend snapshots
desktop_app.go      narrow Wails adapter for dialogs, events, and user intents
frontend            Wails v2 + Svelte/TypeScript presentation
```

The Go backend is the source of truth. The frontend sends intent-level commands
and renders typed snapshots/events; it does not infer session or completion
state. The application and diagnostics CLI call the same host-session runner.
Progress snapshots are emitted at no more than 10 Hz.

The host-session runner exposes one small interface. DBI, Awoo, and Goldleaf
remain independent adapters behind its internal seam; there is no shared
cross-protocol command model. The raw USB transport and safe connection
lifecycle are shared below the adapter seam.

## Installer profiles

Profile capabilities are the single source of truth for UI presentation,
Start validation, CLI help, and tests.

The immutable Go registry describes each profile's stable ID, display name,
protocol family, transport, supported extensions, wire namespace, filesystem
access, and verified installer versions. Wails receives this as typed backend
state and does not maintain a second capability table.

| Profile | Transport | Start-eligible formats | Wire view | Filesystem mutation |
| --- | --- | --- | --- | --- |
| DBI | USB | `.nsp`, `.nsz`, `.xci`, `.xcz` | flat basenames | none |
| Awoo | USB | `.nsp`, `.nsz`, `.xci`, `.xcz` | flat basenames | none |
| Goldleaf 0.10+ | USB | `.nsp` | flat read-only `VIRT:/` | rejected |

All profiles are selected explicitly before Start. A selected file that is not
supported by the active profile blocks Start with an item-specific validation
error; it is never silently omitted.

The profile selector is enabled only while `Idle`. A change immediately
revalidates the selected queue but never silently checks, unchecks, or removes
an item. The last profile is persisted; missing or legacy settings default to
DBI so upgrades preserve existing behavior.

Protocol-family compatibility and exact-version verification are distinct.
Compatible but unverified installer versions may run with a visible warning;
only versions known to be incompatible are blocked. When a version can be
identified reliably, the UI compares it with the profile's verified and
incompatible version sets. When it cannot, the host does not guess. Product
claims and acceptance documents name the exact versions verified on real
hardware.

## Session model

```text
Idle
  -> WaitingForDevice
  -> Connected
  -> Serving
  -> Completed | Disconnected | Failed
  -> Stopping
  -> Idle
```

Every connection has a new session ID. Events carry the session ID so stale
events cannot mutate a later session. `Completed` means only that the host-side
session completed.

User-visible file states are `Queued`, `Requested`, `Serving`, `FullyServed`,
`NotRequested`, `Interrupted`, and `Failed`. The UI must not use `Installed` or
`Installation succeeded`.

## File catalog

- Accept `.nsp`, `.nsz`, `.xci`, and `.xcz`, case-insensitively.
- Recursively scan added directories without following symbolic links.
- Deduplicate identical absolute paths.
- Keep distinct paths with the same basename visible, but reject Start until
  basename conflicts are resolved.
- New entries are checked by default; only checked entries enter the frozen
  source catalog.
- Preserve addition order; MVP has no manual reordering.
- Freeze the catalog at Start. Record path, size, modification time, and file
  identity. Reject a range if its source changed, disappeared, or shortened.
- Never pad an unexpected EOF or silently substitute another file.

The frozen source catalog uses stable internal IDs and absolute paths without
encoding an installer protocol's naming rules. Each profile projects its own
wire view and validates it before Start. DBI and Awoo use flat basenames;
Goldleaf initially uses a flat virtual catalog drive. Duplicate basenames are
therefore rejected by all three profiles, but the restriction belongs to each
wire projection rather than the shared catalog.

## Progress

Track both:

- `uniqueServedBytes`: union of successfully served byte intervals, used for
  bounded progress;
- `wireBytes`: actual USB payload bytes, used for throughput and diagnostics.

Repeated or out-of-order ranges must not push progress above 100%. Overall
progress includes files the active installer protocol actually requested for
serving. Checked files not requested by the end of a session become
`NotRequested`, not `Failed`.

Each adapter reports successfully served byte ranges against stable source
IDs. The host-session runner owns interval union, progress snapshots, and the
shared `Requested`, `Serving`, and `FullyServed` transitions. A partial
Goldleaf read is not `FullyServed`; no profile may translate that state into an
installation claim.

## Cancellation and failure

- Idle USB polling timeouts are normal.
- Short reads/writes continue until complete or cancelled.
- Context cancellation is the primary Stop mechanism. Bounded shutdown stops
  new I/O, cancels the connection-owned transfer context, and waits for every
  in-flight transfer before releasing USB resources. If a transfer does not
  drain before the deadline, keep its libusb context and event loop alive until
  it exits; never reset or close a device underneath a pending transfer.
- User Stop returns to `Idle` without auto-restart.
- Unexpected disconnect retains the frozen source catalog and selected profile
  while waiting for a new device connection. Every reconnect creates a fresh
  serving session and does not claim transfer resumption.
- Malformed frames, unknown commands that cannot be handled safely, source
  changes, and unexpected EOF fail the current session with typed errors.
- A Goldleaf create, write, delete, or rename request against the virtual
  catalog drive receives a protocol-defined failure and a structured warning;
  this expected rejection does not fail the serving session.
- A future app close during serving requires confirmation and bounded shutdown.

## UI scope

The Wails v2 UI provides file/folder addition, drag and drop, queue checkboxes,
removal, clear, search, Start/Stop, connection and session state, unique-byte
progress, structured logs, typed errors, and Light/Dark/Auto theme. The UI
language is English. The application display name is `NSP Carrier` and its
bundle ID is `im.theo.nsp-carrier`.

Settings persist theme, window geometry, recent file/folder directories, and
UI preferences. The queue and absolute paths are not restored implicitly.
Logs are bounded in memory and exported only on request; diagnostic export
hides full paths by default and performs no telemetry.

## Test seams

Tests exercise public behavior at these agreed seams:

1. Per-profile frame and command codecs.
2. Frozen file catalog and range validation.
3. The host-session runner through a transport interface and scripted fake
   transport.
4. Separate manual real-device compatibility gates for DBI, Awoo, and
   Goldleaf.

The transport fake covers short I/O, timeouts, cancellation, disconnects, and
malformed input. Tests do not couple to private helpers or include real content
files.
