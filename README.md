# nsp-carrier

`nsp-carrier` is a clean Go implementation of the PC-side DBI raw-USB file
service with a Wails v2 + Svelte/TypeScript desktop UI.

The project is not an MTP implementation and cannot prove that a title was
installed successfully. It can only report host-observable USB session and
file-serving state.

## Current phase

The protocol and macOS USB path have passed real-device LIST, range, large-file,
multi-file, reconnect, Stop, EXIT, and upstream-differential checks. The Wails
USB MVP is now in progress. The deliberate cable-removal row remains pending
and must pass before the USB MVP is declared complete.

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
bundle identifier is `im.theo.nsp-carrier`; public signing, notarisation,
bundled libusb, and installers remain deferred.

## License

MIT. The implementation is clean-room: upstream projects are behavioral
references only.
