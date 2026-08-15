# Gate 0: Go USB feasibility

Gate 0 determines whether Go + gousb/libusb is a viable USB core on the
development Apple Silicon Mac. Wails UI work cannot begin until every required
item passes on a real Switch in DBI's `Install title from DBIbackend` mode.

## Baseline environment

- OS: macOS 26.5.2 (`darwin/arm64`)
- Go: 1.26.6
- libusb: 1.0.30 from Homebrew
- USB binding: `github.com/google/gousb`
- Device: Nintendo vendor/product `057e:3000`
- DBI and firmware versions: record during execution

## Automated prerequisites

- `go test ./...`
- `go test -race` for packages not requiring real USB
- codec fuzz corpus smoke run
- fake transport covers short I/O, timeout, cancel, disconnect, and malformed
  frames
- `go vet ./...`
- no real NSP/NSZ/XCI/XCZ fixtures

Status on 2026-08-15: **Pass** on the baseline development Mac.

- Unit tests and `go test -race ./...` passed.
- `go vet ./...` passed.
- Header and range parser fuzz smoke runs passed.
- A sparse-file test read a range beyond the 4 GiB boundary.
- `cmd/usb-spike` built as a Mach-O arm64 executable.
- `otool -L` confirmed dynamic linkage to Homebrew
  `/opt/homebrew/opt/libusb/lib/libusb-1.0.0.dylib`.

This status satisfies only the automated prerequisite. It does not change any
hardware row below from Pending.

## Hardware acceptance matrix

Run `make gate0-probe` before opening DBI's 60-second backend window. The
script completes the build first, then waits for and claims the USB device
without requiring content files.

Record Pass/Fail, evidence, and observed errors for every item:

| # | Required behavior | Result | Evidence |
| ---: | --- | --- | --- |
| 1 | Detect `057e:3000` | Pending | |
| 2 | Discover and claim exactly one usable bulk IN/OUT pair | Pending | |
| 3 | Complete LIST exchange | Pending | |
| 4 | Complete metadata range | Pending | |
| 5 | Complete non-sequential range requests | Pending | |
| 6 | Serve a file larger than 4 GiB using 64-bit offsets | Pending | |
| 7 | Sustain a complete large-file transfer | Pending | |
| 8 | Stop returns within the bounded timeout | Pending | |
| 9 | Cable removal does not hang or panic | Pending | |
| 10 | Reconnect starts a fresh working session | Pending | |
| 11 | Handle DBI EXIT | Pending | |
| 12 | Match the original Python backend on the same device | Pending | |

## Decision

- **Pass:** all rows pass; Wails UI work may begin.
- **Fail:** keep work in protocol/USB core, record the exact failure, and do
  not initialize the UI as a substitute for transport feasibility.

Real test content remains local and ignored by Git. `Completed` in logs means
only host-side session completion.
