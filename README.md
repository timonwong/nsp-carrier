# nsp-carrier

[简体中文](README.cn.md) · English

`nsp-carrier` is a Go host for NS installers, with a Wails v2 +
Svelte/TypeScript desktop UI. It exposes selected local files through an
explicitly selected DBI, Awoo, or Goldleaf profile.

The host reports only USB session and file-serving state that it can observe.
It is not an MTP implementation, a Switch-side installer, or proof that a
title was installed successfully.

![NSP Carrier desktop application](docs/assets/nsp-carrier-desktop.jpg)

Representative queue and transfer-progress state:

![NSP Carrier serving files with transfer progress](docs/assets/nsp-carrier-transfer-progress.png)

## Status

- **DBI:** Gate 0 and the Wails USB MVP passed real-Switch validation. A
  reconnect starts a fresh serving session; it does not resume an interrupted
  device-side install.
- **Awoo:** Awoo Installer 1.6.2 is `Verified` for the recorded acceptance
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

## Getting started

`nsp-carrier` is a desktop app. It serves the files you select to an
installer running on your Switch over USB; it never writes to the Switch, so
installation always happens from the installer's side.

You need:

- a Switch running a matching installer in USB mode — DBI, Awoo Installer
  1.6.2, or Goldleaf 0.10+;
- a USB cable between the PC and the Switch;
- a copy of the app. Public installers are not shipped yet, so for now build
  it from source (see [Developing](#developing)) or use a CI build.

Basic flow:

1. Add `.nsp`, `.nsz`, `.xci`, or `.xcz` files and folders to the queue
   (drag and drop works too).
2. Pick the profile that matches your installer: DBI, Awoo, or Goldleaf.
3. Start serving, then install from the installer on the Switch.
4. Watch per-file progress in the app. `FullyServed` means the host sent every
   byte the installer asked for — not that the title was installed.

A file the selected profile cannot serve blocks Start with a clear
validation error rather than being silently skipped.

Platform setup — `libusb` on macOS, a USB driver on Windows — is covered in
[Installing](#installing).

## Installing

### macOS

macOS uses `libusb` for USB access. Install it with Homebrew:

```sh
brew install libusb
```

### Windows

The Switch exposes itself as a raw USB device, so Windows needs a compatible
USB driver before `nsp-carrier` can see it. Install it with
[Zadig](https://zadig.akeo.ie/):

1. Download and run Zadig from <https://zadig.akeo.ie/>.
2. Plug the Switch into the PC with a USB cable.
3. Put your installer into USB mode — on DBI, enter its USB mode
   (DBIbackend); on Awoo, choose USB install; on Goldleaf, open the Remote PC
   browser.
4. In Zadig, open *Options → List All Devices* so the Switch appears.
5. Select the Switch from the dropdown — the vendor ID is `057E` (Nintendo)
   and the product often appears as `DBI`, `USB composite device`, or
   `057E:3000`. Choose the matching entry.
6. Pick **libusbK** as the target driver (or *WinUSB* if libusbK is not
   available).
7. Click *Replace Driver* (or *Install Driver*) and wait for it to finish.
8. Start `nsp-carrier` and confirm it sees the device before serving.

## Developing

Everything below is for building or testing `nsp-carrier` from source; end
users don't need it.

### Build prerequisites

On macOS, install the CGO and frontend dependencies:

```sh
brew install libusb pkgconf
make deps
make ui-install
```

`make check` runs the Go and frontend tests, race checks, fuzz smoke tests,
static analysis, and local builds.

### Developer USB CLI

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

### Desktop app

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

### Continuous integration

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

### Further reading

- [Architecture design](docs/design.md) and [roadmap](docs/roadmap.md)
- [DBI protocol notes](docs/dbi0-protocol.md), [Awoo protocol notes](docs/awoo-usb-protocol.md),
  and [Goldleaf protocol notes](docs/goldleaf-usb-protocol.md)
- [DBI Gate 0](docs/gate0.md), [Awoo gate](docs/awoo-gate.md), and
  [Goldleaf gate](docs/goldleaf-gate.md)

## License

MIT.

## Credits

The DBI, Awoo, and Goldleaf protocol implementations were developed from
observed wire behavior, using these upstream projects as behavioral
references:

- [`rashevskyv/dbibackend-qt`](https://github.com/rashevskyv/dbibackend-qt) — DBI backend reference
- [`developersu/ns-usbloader`](https://github.com/developersu/ns-usbloader) — Awoo and Goldleaf wire references
- [`Huntereb/Awoo-Installer`](https://github.com/Huntereb/Awoo-Installer) — Awoo reference
- [`XorTroll/Goldleaf`](https://github.com/XorTroll/Goldleaf) — Goldleaf reference
