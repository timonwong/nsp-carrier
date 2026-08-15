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
| 1 | Detect `057e:3000` | Pass | 2026-08-15: `gate0-probe` detected the device after direct replug on bus 1, address 5, at SuperSpeed. |
| 2 | Discover and claim exactly one usable bulk IN/OUT pair | Pass | 2026-08-15: reset-on-connect and claim succeeded for configuration 1, interface 0, alternate 0, bulk IN 1 and OUT 1; probe closed cleanly. |
| 3 | Complete LIST exchange | Pass | 2026-08-15: both XCI and NSP basenames appeared in DBI; the NSP session proceeded from the listed file to installation. |
| 4 | Complete metadata range | Pass | 2026-08-15: DBI parsed the NSP metadata and completed signature verification and installation. |
| 5 | Complete non-sequential range requests | Pass | 2026-08-15: a 34-file NSP batch completed with every file reporting non-sequential and backward range access. The 1.27 GB base NSP served 1,224 range requests, including 6 non-sequential and 1 backward request. |
| 6 | Serve a file larger than 4 GiB using 64-bit offsets | Pass | 2026-08-15: fully served a 7,111,486,912-byte NSP; unique progress reached the full size, proving offsets beyond 4 GiB. |
| 7 | Sustain a complete large-file transfer | Pass | 2026-08-15: DBI installed the 7.11 GB NSP successfully; host reported `FullyServed` with 7,111,486,912 unique bytes and 7,111,502,736 wire bytes. |
| 8 | Stop returns within the bounded timeout | Pass | 2026-08-15: Ctrl-C cancelled the active XCI session and returned to `Idle` in under one second. |
| 9 | Cable removal does not hang or panic | Pending | |
| 10 | Reconnect starts a fresh working session | Pass | 2026-08-15: after ending the stale reset-on-connect session, a new no-reset session claimed the device, listed the NSP, and completed installation. |
| 11 | Handle DBI EXIT | Pass | 2026-08-15: leaving the DBI file list with a second `B` produced host `session_completed` and `Idle`. |
| 12 | Match the original Python backend on the same device | Pass | 2026-08-15: classic Python backend commit `ba104f17` reproduced the same XCI behavior on the same Switch and file: transfer completed, then DBI remained in its device-side installation phase. This differential rules out the Go host implementation as the cause of that XCI-specific stall. |

### Live observations

- 2026-08-15: the first full session served 3,818,919,936 unique bytes of a
  3,819,575,808-byte XCI before the strict catalog rejected the final aligned
  request as out of bounds. The remaining 655,872 bytes and a 1 MiB request
  imply a 392,704-byte EOF overshoot. The original Python backend permits this
  pattern by sending only the available tail bytes. A regression test now
  covers that behavior without zero-padding. The fixed build subsequently
  reached DBI's transfer and signature verification, as recorded below.
- 2026-08-15: after the aligned-tail fix, DBI reported `[TRANSFER OK]` and
  `[SIGNATURE: OK]` for the XCI, but its device-side install progress remained
  at 57 percent. The host stayed ready for the next command and did not fail.
  This is retained as an XCI-specific observation, not counted as installation
  success.
- 2026-08-15: the same Switch and XCI were then tested with the unmodified
  classic Python `dbibackend` at commit `ba104f17`. It reproduced the same
  device-side stall after transfer. No XCI-specific Go workaround will be
  attempted without a different XCI fixture or evidence that the original
  backend succeeds with this file.
- 2026-08-15: a separate 7,111,486,912-byte NSP completed installation. Host
  progress was `FullyServed`; wire bytes exceeded unique bytes by 15,824,
  proving repeated or overlapping reads. Request ordering was not captured, so
  the explicit non-sequential-order row remained Pending at that point.
- 2026-08-15: DBI subsequently installed all 34 NSPs from a 2,224,080,769-byte
  base/update/DLC catalog in 1 minute 24 seconds and reported `OK: 34/34`.
  Every file reached host-side `FullyServed`; the new request-order telemetry
  observed non-sequential and backward ranges for every file. DBI then sent
  `EXIT`, and the host returned to `Idle`.

## Decision

- **Pass:** all rows pass; Wails UI work may begin.
- **Fail:** keep work in protocol/USB core, record the exact failure, and do
  not initialize the UI as a substitute for transport feasibility.

Real test content remains local and ignored by Git. `Completed` in logs means
only host-side session completion.

On 2026-08-15 the user explicitly deferred row 9 and requested that Wails UI
work begin. This changes implementation order, not the acceptance result: row
9 remains Pending, and the USB MVP is not complete until it is exercised.
