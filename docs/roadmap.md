# Roadmap

## Gate 0

- DBI0 codec, catalog, state model, and transport boundary
- scripted fake transport and automated verification
- retained `cmd/usb-spike` diagnostics CLI
- gousb/libusb adapter on Apple Silicon macOS
- real-Switch acceptance matrix (passed 2026-08-16)

## USB MVP after Gate 0

- Wails v2.14 with Svelte 5 and TypeScript (implemented)
- English macOS utility UI (implemented)
- queue, selection, search, progress, logs, typed errors, Start/Stop
  (implemented; real-Switch full transfer passed 2026-08-16)
- Light/Dark/Auto theme (implemented)
- deliberate cable-removal and reconnect validation (passed 2026-08-16;
  reconnect starts a fresh DBI session and does not resume an interrupted install)
- no public packaging yet

## Installer protocol expansion

- explicit installer profiles: DBI, Awoo USB, and Goldleaf (implemented)
- independent clean-room protocol adapters over the shared USB transport
  (DBI, Awoo, and Goldleaf implemented)
- one shared host-session runner used by Wails and `usb-spike` (implemented)
- protocol-neutral frozen source catalog with profile-owned wire projections
  (implemented)
- profile capabilities as the source of truth for format validation and UI
  (implemented)
- persisted Idle-only profile selection with DBI as the migration default
  (implemented)
- distinct compatible, verified, and known-incompatible version reporting
- Awoo USB protocol compatibility (automated evidence implemented; Awoo 1.6.2
  real-device XCI, >4 GiB NSP, Stop, cable-removal, and reconnect checks passed;
  multi-file and real command differential passed; remaining format evidence
  pending)
- Goldleaf 0.10+ read-only virtual catalog adapter (automated evidence
  implemented; Goldleaf 1.2.0 browsing, >4 GiB NSP, delete rejection, Stop,
  cable-removal, fresh-session reconnect, whole-source, and multi-file checks
  passed; create/write/delete/rename rejection and real command differential
  passed; exact version 1.2.0 verified)
- scripted protocol fixtures and separate real-device acceptance matrices
- no automatic protocol detection on the shared `057e:3000` USB identity

## Deferred HTTP

HTTP, including Awoo Network transfer, is explicitly deferred until after the
USB installer protocol expansion. Before implementation, its design must
resolve:

- opt-in LAN exposure and bind-address policy;
- authentication or pairing;
- directory-listing visibility;
- HTTP byte ranges and concurrent-reader correctness;
- firewall and port-conflict UX;
- progress semantics distinct from USB;
- TLS expectations and threat model.

The reference Qt implementation's unauthenticated `0.0.0.0` listener and
shared seekable file handle are not acceptable designs.

## Other deferred work

- `.dbi` preset save/load
- `.bat` export
- single-instance file handoff
- taskbar/dock integration
- queue persistence and explicit save/load
- read-only serving from existing split-file chunk directories
- public distribution, bundled libusb, codesigning, notarization, auto-update
- validated Windows and Linux support
- i18n

## Out of scope

- RCM payload injection
- split-file creation
- split-file merging
