# ya-dbibackend

`ya-dbibackend` is a clean Go implementation of the PC-side DBI raw-USB
file service, with a Wails v2 desktop UI planned after the USB feasibility
gate passes on real hardware.

The project is not an MTP implementation and cannot prove that a title was
installed successfully. It can only report host-observable USB session and
file-serving state.

## Current phase

Development is deliberately staged:

1. Specify and test the DBI0 protocol core.
2. Prove the Go/libusb transport on an Apple Silicon Mac with a real Switch.
3. Build the Wails v2 + Svelte/TypeScript UI only after Gate 0 passes.

See [the design](docs/design.md), [protocol notes](docs/dbi0-protocol.md),
[Gate 0](docs/gate0.md), and [roadmap](docs/roadmap.md).

## Developer USB spike

On macOS, install the CGO build prerequisites:

```sh
brew install libusb pkgconf
```

Run the retained diagnostics CLI with local content paths:

```sh
go run ./cmd/usb-spike --timeout=30m --verbose -- /path/to/file.nsp /path/to/folder
```

The CLI recursively builds and freezes the catalog, waits for USB device
`057e:3000`, discovers exactly one bulk IN/OUT endpoint pair, and serves DBI0
until DBI exits, the device disconnects, or the context is cancelled. Use
`--json` for newline-delimited structured logs. Real content files are ignored
by Git and must never be committed.

Passing automated tests or compiling this CLI does not pass Gate 0; the
hardware acceptance matrix in `docs/gate0.md` remains authoritative.

## License

MIT. The implementation is clean-room: upstream projects are behavioral
references only.
