# Design

## Product boundary

`nsp-carrier` is a macOS-first, cross-platform-structured replacement for
the PC-side DBI backend. It exposes selected local files to DBI over the
vendor-specific DBI0 bulk USB protocol.

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
internal/dbi        protocol codec and session state machine
internal/files      frozen virtual file catalog
internal/transport  USB-independent stream boundary and fake transport
internal/usb        gousb/libusb adapter
internal/app        future orchestration and frontend event projection
frontend            future Wails v2 + Svelte/TypeScript UI
```

The Go backend is the source of truth. A future frontend sends commands and
renders typed snapshots/events; it does not infer session or completion state.
Progress events will be throttled to roughly 10-20 Hz.

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
  DBI catalog.
- Preserve addition order; MVP has no manual reordering.
- Freeze the catalog at Start. Record path, size, modification time, and file
  identity. Reject a range if its source changed, disappeared, or shortened.
- Never pad an unexpected EOF or silently substitute another file.

The catalog uses stable internal IDs and absolute paths. The DBI wire identity
is necessarily the basename.

## Progress

Track both:

- `uniqueServedBytes`: union of successfully served byte intervals, used for
  bounded progress;
- `wireBytes`: actual USB payload bytes, used for throughput and diagnostics.

Repeated or out-of-order ranges must not push progress above 100%. Overall
progress includes files DBI actually requested for serving. Checked files not
requested by the end of a session become `NotRequested`, not `Failed`.

## Cancellation and failure

- Idle USB polling timeouts are normal.
- Short reads/writes continue until complete or cancelled.
- Context cancellation is the primary Stop mechanism; device reset is a
  bounded fallback.
- User Stop returns to `Idle` without auto-restart.
- Unexpected disconnect retains the frozen catalog and waits for a new
  session, but does not claim transfer resumption.
- Malformed frames, unsupported commands, source changes, and unexpected EOF
  fail the current session with typed errors.
- A future app close during serving requires confirmation and bounded shutdown.

## Future UI scope

After Gate 0 passes, the Wails v2 UI will provide file/folder addition, drag
and drop, queue checkboxes, removal, clear, search, Start/Stop, connection and
session state, unique-byte progress, structured logs, typed errors, and
Light/Dark/Auto theme. The UI language is English. The future application
display name is `NSP Carrier` and its bundle ID is `im.theo.nsp-carrier`.

Settings persist theme, window geometry, recent file/folder directories, and
UI preferences. The queue and absolute paths are not restored implicitly.
Logs are bounded in memory and exported only on request; diagnostic export
hides full paths by default and performs no telemetry.

## Test seams

Tests exercise public behavior at these agreed seams:

1. DBI frame and command codec.
2. Frozen file catalog and range validation.
3. Session service through a transport interface and scripted fake transport.
4. Manual real-device compatibility via Gate 0.

The transport fake covers short I/O, timeouts, cancellation, disconnects, and
malformed input. Tests do not couple to private helpers or include real content
files.
