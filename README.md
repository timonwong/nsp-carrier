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

## License

MIT. The implementation is clean-room: upstream projects are behavioral
references only.
