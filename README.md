<p align="center">
  <img src="docs/assets/nsp-carrier-logo.svg" alt="NSP Carrier logo" width="96" />
  <br />
  <strong style="font-size: 32px">NSP Carrier</strong>
</p>

[简体中文](README.cn.md) · English

**NSP Carrier** is a desktop app that serves `.nsp`, `.nsz`, `.xci`, `.xcz`,
and Sphaira-only `.msp` files to the installer running on your Switch over USB
— Awoo, Goldleaf, Sphaira, or DBI. Add files to the queue, pick the matching installer
profile, and serve; the installer on the Switch performs the actual install.

The app reports only what it can observe over USB: which files were served
and how far each transfer got. It is not an installer, an MTP server, or
proof that a title was installed.

![NSP Carrier desktop application](docs/assets/nsp-carrier-desktop.jpg)

Representative queue and transfer-progress state:

![NSP Carrier serving files with transfer progress](docs/assets/nsp-carrier-transfer-progress.png)

## Features

- Serves files over USB through an explicitly selected installer profile:
  - [Awoo](https://github.com/Huntereb/Awoo-Installer)
  - [Goldleaf 0.10+](https://github.com/XorTroll/Goldleaf)
  - [Sphaira 1.0+](https://github.com/NaGaa95/sphaira)
  - DBI
- File and recursive folder selection, with drag and drop.
- Queue with search and duplicate-basename validation.
- Per-file unique-byte progress, bounded activity logs, and typed errors.
- Start/Stop with a host-owned session lifecycle — a reconnect starts a
  fresh session and never claims transfer resumption.
- Auto/Light/Dark appearance.

The host reports only what it can observe: `FullyServed` means the selected
profile's byte-coverage rule was satisfied, not that the title was installed.

## Getting started

`nsp-carrier` is a desktop app. It serves the files you select to an
installer running on your Switch over USB; it never writes to the Switch, so
installation always happens from the installer's side.

You need:

- a Switch running a matching installer in USB mode — Awoo Installer 1.6.2,
  Goldleaf 0.10+, Sphaira 1.0+, or DBI;
- a USB cable between the PC and the Switch;
- a copy of the app — download it from the
  [Releases](https://github.com/timonwong/nsp-carrier/releases) page (see
  [Installing](#installing)).

Basic flow:

1. Add `.nsp`, `.nsz`, `.xci`, `.xcz`, or `.msp` files and folders to the queue
   (drag and drop works too).
2. Pick the profile that matches your installer: Awoo, Goldleaf, Sphaira, or DBI.
3. Start serving, then install from the installer on the Switch.
4. Watch per-file progress in the app. `FullyServed` means the selected
   profile's observable byte-coverage rule was satisfied — not that the title
   was installed.

A file the selected profile cannot serve blocks Start with a clear
validation error rather than being silently skipped.

Platform setup — `libusb` on macOS, a USB driver on Windows — is covered in
[Installing](#installing).

## Installing

Download the app from the
[Releases](https://github.com/timonwong/nsp-carrier/releases) page:

- **macOS:** unzip `nsp-carrier-macos-arm64.zip` and move `NSP Carrier.app`
  into Applications.
- **Windows:** unzip `nsp-carrier-windows-amd64.zip` and run
  `nsp-carrier.exe`.

Releases are not code-signed or notarised. On macOS, open the app the first
time with right-click → Open to bypass Gatekeeper.

### Platform prerequisites

#### macOS

macOS uses `libusb` for USB access. Install it with Homebrew:

```sh
brew install libusb
```

#### Windows

The Switch exposes itself as a raw USB device, so Windows needs a compatible
USB driver before `nsp-carrier` can see it. Install it with
[Zadig](https://zadig.akeo.ie/):

1. Download and run Zadig from <https://zadig.akeo.ie/>.
2. Plug the Switch into the PC with a USB cable.
3. Put your installer into USB mode — on DBI, enter its USB mode
   (DBIbackend); on Awoo, choose USB install; on Goldleaf, open the Remote PC
   browser; on Sphaira, open its USB install flow.
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
make usb-spike ARGS='--profile=sphaira --timeout=30m --verbose -- /path/to/file.nsp'
```

For bounded Awoo, Goldleaf, or Sphaira command metadata, add `--trace-protocol`. Each
session emits at most 300 records and then reports truncation. Records contain
command, direction, result, source ID, range metadata, and integrity verdicts
only; they never contain local paths, wire names, raw packets, payloads, or
checksum values.

`make gate0-probe` is DBI-specific: it builds before waiting, claims the
discovered bulk endpoints, and exits without serving file content. Once it is
ready, open `Install title from DBIbackend` on the Switch. Use the ordinary
`--profile=awoo` flow for [Awoo evidence](docs/awoo-gate.md), or
`--profile=goldleaf` with Goldleaf's Remote PC browser for the [Goldleaf
gate](docs/goldleaf-gate.md), or `--profile=sphaira` for the pending [Sphaira
gate](docs/sphaira-gate.md).

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

Pushing a tag whose name starts with `v` reuses the same build workflow and
publishes both platform zips and their checksums to a GitHub Release with
generated notes.

### Further reading

- [Architecture design](docs/design.md) and [roadmap](docs/roadmap.md)
- [DBI protocol notes](docs/dbi0-protocol.md), [Awoo protocol notes](docs/awoo-usb-protocol.md),
  [Goldleaf protocol notes](docs/goldleaf-usb-protocol.md), and [Sphaira SPH0
  notes](docs/sphaira-usb-protocol.md)
- [DBI Gate 0](docs/gate0.md), [Awoo gate](docs/awoo-gate.md), and
  [Goldleaf gate](docs/goldleaf-gate.md), plus the pending [Sphaira
  gate](docs/sphaira-gate.md)

## License

MIT.

## Credits

The DBI, Awoo, Goldleaf, and Sphaira protocol implementations were developed from
observed wire behavior, using these upstream projects as behavioral
references:

- [`rashevskyv/dbibackend-qt`](https://github.com/rashevskyv/dbibackend-qt) — DBI backend reference
- [`developersu/ns-usbloader`](https://github.com/developersu/ns-usbloader) — Awoo and Goldleaf wire references
- [`Huntereb/Awoo-Installer`](https://github.com/Huntereb/Awoo-Installer) — Awoo reference
- [`XorTroll/Goldleaf`](https://github.com/XorTroll/Goldleaf) — Goldleaf reference
- [`NaGaa95/sphaira`](https://github.com/NaGaa95/sphaira/tree/3f8303db00f33bfffa83ce0a1b750a1de14656e2) — fixed Sphaira 1.0.6 behavioral reference

Sphaira 1.0+ is currently **Compatible**, based on fixed-revision behavior and
automated tests. No exact Sphaira version is **Verified** until every row in
the real-device matrix passes. Sphaira 0.13.3 and earlier use the incompatible
legacy TUL0/TUC0 generation.
