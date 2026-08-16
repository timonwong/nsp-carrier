# nsp-carrier

`nsp-carrier` is a Go host for NS installers, with a Wails v2 +
Svelte/TypeScript desktop UI. It exposes selected local files through an
explicitly selected DBI, Awoo USB, or Goldleaf profile.

The host reports only USB session and file-serving state that it can observe.
It is not an MTP implementation, a Switch-side installer, or proof that a
title was installed successfully.

## Status

- **DBI:** Gate 0 and the Wails USB MVP passed real-Switch validation. A
  reconnect starts a fresh serving session; it does not resume an interrupted
  device-side install.
- **Awoo USB:** Awoo Installer 1.6.2 is `Verified` for the recorded acceptance
  matrix, including real-device `.nsp`, `.nsz`, `.xci`, and `.xcz` transfers.
  Other versions remain protocol-family compatible until separately verified.
- **Goldleaf:** Goldleaf 1.2.0 is `Verified` for the recorded acceptance
  matrix. Other 0.10+ versions are protocol-family compatible until separately
  verified.
- **Distribution:** Public packaging, signing, notarisation, and installers
  are deferred.

The UI supports file and recursive folder selection, drag and drop, explicit
profile selection, queue search and duplicate-basename validation, Start/Stop,
Go-owned session state, per-file unique-byte progress, bounded activity logs,
typed errors, and Auto/Light/Dark appearance. `FullyServed` means the host
sent every byte requested for a file; it does not mean the device installed it.

## Further reading

- [Architecture design](docs/design.md) and [roadmap](docs/roadmap.md)
- [DBI protocol notes](docs/dbi0-protocol.md), [Awoo protocol notes](docs/awoo-usb-protocol.md),
  and [Goldleaf protocol notes](docs/goldleaf-usb-protocol.md)
- [DBI Gate 0](docs/gate0.md), [Awoo gate](docs/awoo-gate.md), and
  [Goldleaf gate](docs/goldleaf-gate.md)

## Build prerequisites

On macOS, install the CGO and frontend dependencies:

```sh
brew install libusb pkgconf
make deps
make ui-install
```

`make check` runs the Go and frontend tests, race checks, fuzz smoke tests,
static analysis, and local builds.

## Developer USB CLI

Build and run the retained diagnostics CLI with local content paths:

```sh
make build
make usb-spike ARGS='--profile=dbi --timeout=30m --verbose -- /path/to/file.nsp /path/to/folder'
make usb-spike ARGS='--profile=awoo --timeout=30m --verbose -- /path/to/file.nsp'
make usb-spike ARGS='--profile=goldleaf --timeout=30m --verbose -- /path/to/file.nsp'
```

For bounded Awoo or Goldleaf command metadata, add `--trace-protocol`. Each
session emits at most 300 records and then reports truncation. Records contain
command, direction, result, source ID, and range metadata only; they never
contain local paths, wire names, or content payloads.

`make gate0-probe` is DBI-specific: it builds before waiting, claims the
discovered bulk endpoints, and exits without serving file content. Once it is
ready, open `Install title from DBIbackend` on the Switch. Use the ordinary
`--profile=awoo` flow for [Awoo evidence](docs/awoo-gate.md), or
`--profile=goldleaf` with Goldleaf's Remote PC browser for the [Goldleaf
gate](docs/goldleaf-gate.md).

The CLI recursively builds and freezes the catalog, waits for USB device
`057e:3000`, discovers exactly one bulk IN/OUT endpoint pair, and serves the
selected profile until it exits, disconnects, or is cancelled. Use `--json`
for newline-delimited structured logs. Real content files are ignored by Git
and must never be committed.

Passing automated tests or compiling this CLI does not pass a real-device
gate; each profile's acceptance document remains independently authoritative.

## Desktop app

Install the pinned Wails v2 CLI and verify the local macOS toolchain:

```sh
make wails-install
make wails-doctor
```

Run or build the app:

```sh
make app-dev
make app-build
```

The bundle is written to `build/bin/NSP Carrier.app`. The local bundle is
unsigned; public signing, notarisation, and installers remain deferred. The
bundle identifier is `im.theo.nsp-carrier`.

## Continuous integration

GitHub Actions builds and checks two desktop targets for pull requests, pushes
to `main`, and manual runs:

- Windows amd64 on a native x64 runner;
- macOS arm64 on an Apple Silicon runner.

Each job uploads a seven-day zip artifact and a SHA-256 checksum. The Windows
zip includes `libusb-1.0.dll`; the macOS app embeds `libusb` and is ad-hoc
signed after bundling. These CI artifacts are not publicly code-signed or
notarised. Windows users must configure a compatible USB driver such as
WinUSB separately. Real-device acceptance is recorded on macOS arm64; Linux
is not built or supported.

## License

MIT. The implementation is clean-room: upstream projects are behavioral
references only.
