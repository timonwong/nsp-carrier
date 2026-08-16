# nsp-carrier

`nsp-carrier` is a clean Go implementation of the PC-side DBI raw-USB file
service with a Wails v2 + Svelte/TypeScript desktop UI.

The project is not an MTP implementation and cannot prove that a title was
installed successfully. It can only report host-observable USB session and
file-serving state.

## Current phase

Gate 0 and the Wails USB MVP have passed real-Switch validation. The protocol,
macOS USB path, and desktop UI have completed LIST, range, large-file,
multi-file, reconnect, Stop, EXIT, upstream-differential, full UI transfer, and
deliberate cable-removal checks. Public packaging and signing remain deferred.

The current UI provides:

- native file and recursive folder selection plus file drop;
- a selectable, searchable queue with duplicate-basename conflict detection;
- Start/Stop and canonical Go-owned session state;
- per-file unique-byte progress, bounded activity logs, and typed errors;
- Auto, Light, and Dark appearance;
- the explicit distinction between `FullyServed` and device-side installation.

See [the design](docs/design.md), [protocol notes](docs/dbi0-protocol.md),
[Gate 0](docs/gate0.md), and [roadmap](docs/roadmap.md).

## Developer USB spike

On macOS, install the CGO build prerequisites:

```sh
brew install libusb pkgconf
```

Run the retained diagnostics CLI with local content paths:

```sh
make check
make gate0-probe
make usb-spike ARGS='--timeout=30m --verbose -- /path/to/file.nsp /path/to/folder'
```

`make gate0-probe` builds before it starts waiting. Once it prints that the
host is ready, open `Install title from DBIbackend` on the Switch; probe mode
claims the discovered bulk endpoints and exits without serving file content.

The CLI recursively builds and freezes the catalog, waits for USB device
`057e:3000`, discovers exactly one bulk IN/OUT endpoint pair, and serves DBI0
until DBI exits, the device disconnects, or the context is cancelled. Use
`--json` for newline-delimited structured logs. Real content files are ignored
by Git and must never be committed.

Passing automated tests or compiling this CLI does not pass Gate 0; the
hardware acceptance matrix in `docs/gate0.md` remains authoritative.

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
