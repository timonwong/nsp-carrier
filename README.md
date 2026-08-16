# nsp-carrier

`nsp-carrier` is an Omni host for NS installers, implemented cleanly in Go
with a Wails v2 + Svelte/TypeScript desktop UI.

The project is not an MTP implementation and cannot prove that a title was
installed successfully. It can only report host-observable USB session and
file-serving state.

## Current phase

The DBI profile's Gate 0 and Wails USB MVP have passed real-Switch validation.
The DBI0 protocol, macOS USB path, and desktop UI have completed LIST, range,
large-file, multi-file, reconnect, Stop, EXIT, upstream-differential, full UI
transfer, and deliberate cable-removal checks. Public packaging and signing
remain deferred.

The Awoo USB profile has an independent clean-room adapter with pinned
transcript, unit, fuzz, race, and differential coverage. Awoo Installer 1.6.2
has passed real-device NSP, NSZ, XCI, XCZ, Stop, cable-removal,
fresh-session reconnect, multi-file, and command-differential checks, so this
exact version is `Verified` for the recorded acceptance matrix.
The Goldleaf 0.10+ read-only `VIRT:/` adapter is implemented with
automated protocol coverage. Goldleaf 1.2.0 has passed real-device virtual
catalog browsing, a greater-than-4-GiB NSP installation, read-only delete, Stop,
fresh-session disconnect/reconnect, whole-source serving, and multi-file
installation. Create, write, rename, and pinned-reference differential checks
also passed, so exact version 1.2.0 is `Verified`; other 0.10+ versions remain
protocol-family compatible until independently verified.

The current UI provides:

- native file and recursive folder selection plus file drop;
- explicit DBI, Awoo USB, or Goldleaf profile selection with persisted DBI
  migration fallback and profile-specific validation;
- a selectable, searchable queue with duplicate-basename conflict detection;
- Start/Stop and canonical Go-owned session state;
- per-file unique-byte progress, bounded activity logs, and typed errors;
- Auto, Light, and Dark appearance;
- the explicit distinction between `FullyServed` and device-side installation.

See [the design](docs/design.md), [DBI protocol notes](docs/dbi0-protocol.md),
[Awoo protocol notes](docs/awoo-usb-protocol.md), [DBI Gate 0](docs/gate0.md),
[Awoo gate](docs/awoo-gate.md), [Goldleaf protocol notes](docs/goldleaf-usb-protocol.md),
[Goldleaf gate](docs/goldleaf-gate.md), and [roadmap](docs/roadmap.md).

## Developer USB spike

On macOS, install the CGO build prerequisites:

```sh
brew install libusb pkgconf
```

Run the retained diagnostics CLI with local content paths:

```sh
make check
make gate0-probe
make usb-spike ARGS='--profile=dbi --timeout=30m --verbose -- /path/to/file.nsp /path/to/folder'
make usb-spike ARGS='--profile=awoo --timeout=30m --verbose -- /path/to/file.nsp'
make usb-spike ARGS='--profile=goldleaf --timeout=30m --verbose -- /path/to/file.nsp'
```

For bounded Awoo or Goldleaf command metadata, add `--trace-protocol`. Each
session emits at most 300 records and then reports truncation. Records contain
command, direction, result, source ID, and range metadata only; they never
contain local paths, wire names, or content payloads.

`make gate0-probe` is DBI-specific: it builds before waiting, then claims the
discovered bulk endpoints and exits without serving file content. Once it is
ready, open `Install title from DBIbackend` on the Switch. Use the ordinary
`--profile=awoo` CLI flow and [Awoo gate](docs/awoo-gate.md) for Awoo evidence.
Use `--profile=goldleaf` and open Goldleaf's Remote PC browser for the separate
[Goldleaf gate](docs/goldleaf-gate.md).

The CLI recursively builds and freezes the catalog, waits for USB device
`057e:3000`, discovers exactly one bulk IN/OUT endpoint pair, and serves the
explicitly selected profile until it exits, disconnects, or is cancelled. Use
`--json` for newline-delimited structured logs. Real content files are ignored
by Git and must never be committed.

Passing automated tests or compiling this CLI does not pass a real-device
gate; each profile's acceptance document remains independently authoritative.

## Wails UI

Install the pinned Wails v2 CLI and verify the local macOS toolchain:

```sh
make wails-install
make wails-doctor
```

Then run or build the app:

```sh
make ui-install
make app-dev
make app-build
```

The production development bundle is written to
`build/bin/NSP Carrier.app`. It is self-signed for local testing only. The
bundle identifier is `im.theo.nsp-carrier`; public signing, notarisation, and
installers remain deferred.

## Continuous integration

GitHub Actions builds and checks the two currently supported desktop targets
for pull requests, pushes to `main`, and manual runs:

- Windows amd64 on a native x64 runner;
- macOS arm64 on an Apple Silicon runner.

Each job uploads a seven-day zip artifact and a SHA-256 checksum. The Windows
zip includes `libusb-1.0.dll`; the macOS app embeds `libusb` and is ad-hoc
signed after bundling. These CI artifacts are not publicly code-signed or
notarised. Windows users must configure a compatible USB driver such as
WinUSB separately. Linux is not currently built or supported.

## License

MIT. The implementation is clean-room: upstream projects are behavioral
references only.
