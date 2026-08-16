# Goldleaf 0.10+ real-device acceptance gate

Automated transcript, unit, fuzz, race, and build checks are prerequisites
only. They do not establish real-device compatibility.

Status: **In progress; core browsing, large-file installation, read-only delete,
Stop, and disconnect/reconnect paths passed on Goldleaf 1.2.0**.

## Recorded environment

- Goldleaf 1.2.0 using its Remote PC browser; the USB serial descriptor also
  reported `1.2.0`;
- Switch firmware unconfirmed, reported as approximately version 20;
- NSP Carrier commits `18fdbdd` (adapter) and `75d949f` (disconnect
  classification);
- macOS 26.5.2 arm64, libusb 1.0.30, and gousb 1.1.3;
- USB `057e:3000`, SuperSpeed, configuration 1, interface 0, alternate 0,
  bulk IN endpoint 1, and bulk OUT endpoint 1.

## Acceptance matrix

| # | Required behavior | Result |
| ---: | --- | --- |
| 1 | Detect and claim `057e:3000` with Goldleaf selected | Pass |
| 2 | Show exactly one `Virtual` / `VIRT:/` drive and no `HOME` drive | Pass |
| 3 | List selected `.nsp` files in frozen catalog order and stat them correctly | Pass |
| 4 | Open and read an NSP through start/read/end, including non-zero and repeated ranges | Pass |
| 5 | Keep partial reads below `FullyServed`; reach `FullyServed` only after the whole source is served | Partial |
| 6 | Reject create, write, delete, and rename; show structured warnings and keep browsing | Partial |
| 7 | Serve a file larger than 4 GiB with 64-bit offsets | Pass |
| 8 | Complete a multi-file browsing/serving session | Pending |
| 9 | Stop within the bounded shutdown deadline | Pass |
| 10 | Cable removal becomes `Disconnected`; reconnect creates a fresh session without claiming resume | Pass |
| 11 | Compare the observable exchange with the pinned behavioral references | Partial |

## Observed sessions

- Goldleaf displayed the single `Virtual` catalog and the selected NSP. An
  initial 330,968-byte `Checkpoint.nsp` browse session was stopped before any
  read after its provenance could not be verified; the host correctly reported
  `NotRequested` and zero served bytes.
- The 7,111,486,912-byte Zumba NSP was installed from `VIRT:/`. Goldleaf
  displayed installation success and then offered to delete the NSP. The host
  served 7,111,461,200 wire bytes across 867 range requests, including 12
  non-sequential, six backward, and one repeated request. The maximum requested
  offset was 7,110,078,912, proving the real 64-bit path beyond 4 GiB.
- Goldleaf did not request 32,256 unique source bytes. The host therefore
  retained partial whole-source truth instead of reporting `FullyServed`, even
  though the device reported installation success. A real session that requests
  every source byte remains unobserved, so matrix row 5 is Partial.
- Goldleaf's delete action displayed success, but the host rejected the
  mutation: the local Zumba NSP retained its exact size, modification time, and
  SHA-256, and it reappeared after leaving and re-entering `VIRT:/`. Goldleaf
  1.2.0 does not surface the adapter's negative delete result in this UI path.
  The session remained usable. Create, write, and rename remain unobserved, so
  matrix row 6 is Partial.
- Cable removal during session `2e053ecb-5b72-4abe-afb1-6f5633dbc6c3`
  produced typed `Disconnected` with `transport disconnected: transfer was
  cancelled`, then returned to `WaitingForDevice` without exiting. Reconnecting
  was claimed as fresh session `01741089-1691-4c0f-8233-112a730ef38f`, which
  entered `Serving` without claiming resume.
- Ctrl-C after the device reported success emitted `Stopping`, returned to
  `Idle`, and exited 0 within the bounded shutdown path.
- Running macOS USB descriptor enumeration concurrently with the host caused
  one transient exclusive-open collision during claim. Retrying without a
  parallel enumerator claimed the same Goldleaf device immediately. Do not run
  `system_profiler` or equivalent USB enumeration during a gate claim.

The real exchange interoperated with Goldleaf 1.2.0, but a packet-level
differential against the pinned references remains pending.

Do not call any Goldleaf version `Verified` until its exact version and every
row above pass. Automated coverage establishes only implementation readiness.
