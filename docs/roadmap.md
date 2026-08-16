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

## Deferred HTTP

HTTP is explicitly deferred until after the USB MVP. Before implementation,
its design must resolve:

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
- public distribution, bundled libusb, codesigning, notarization, auto-update
- validated Windows and Linux support
- i18n
