# Awoo USB real-device acceptance gate

Automated transcript, fuzz, race, and build checks are prerequisites only.
They do not establish real-device compatibility. This gate is independent of
the DBI Gate 0 evidence.

Status: **In progress; core transfer and lifecycle paths passed on Awoo
Installer 1.6.2**.

## Recorded environment

- Awoo Installer 1.6.2; exact revision unavailable;
- Switch firmware unconfirmed, reported as approximately version 20;
- NSP Carrier commits `820a615` through `a58e37d`;
- macOS 26.5.2 arm64, libusb 1.0.30, and gousb 1.1.3;
- USB `057e:3000`, SuperSpeed, configuration 1, interface 0, alternate 0,
  bulk IN endpoint 1, and bulk OUT endpoint 1.

## Acceptance matrix

| # | Required behavior | Result | Evidence |
| ---: | --- | --- | --- |
| 1 | Detect and claim `057e:3000` with the Awoo profile selected | Pass | Awoo 1.6.2 was claimed repeatedly with the recorded SuperSpeed configuration and endpoints. |
| 2 | Complete the host-initiated `TUL0` list exchange | Pass | The device listed and requested both the XCI and NSP supplied by the host. |
| 3 | Serve `.nsp`, `.nsz`, `.xci`, and `.xcz` selections exercised by the device | Partial | `.xci` and `.nsp` completed; `.nsz` and `.xcz` remain untested. |
| 4 | Complete both observed range-command variants where the device emits them | Partial | Real sessions completed, but the command ID was not captured in the activity evidence; fixed automated transcripts cover IDs 1 and 2. |
| 5 | Serve offsets beyond 4 GiB and a large range/file | Pass | A 7,111,486,912-byte NSP completed with maximum requested offset 7,110,078,912. |
| 6 | Complete a multi-file session and device exit command | Partial | Device exit completed single-file XCI and NSP sessions; a multi-file session remains pending. |
| 7 | Stop returns within the bounded shutdown deadline | Pass | After `a58e37d`, real-device Ctrl-C emitted `Stopping`, returned to `Idle`, and exited 0 without a timeout. |
| 8 | Cable removal becomes `Disconnected` without hang or panic | Pass | Removal after 909,120,000 XCI bytes emitted typed `Disconnected` and retained the waiting runner. |
| 9 | Reconnect creates a fresh serving-session ID without claiming resume | Pass | Session `c33c03ba-2f0c-45d9-84b6-c5c9eeec2f69` disconnected; reconnect was claimed as fresh session `97b17d1a-e5c8-4581-9dc4-bf717f50bde3`. |
| 10 | Compare the observable exchange with the pinned behavioral reference | Partial | Manual interoperability produced successful host-side sessions; a real-device packet differential against the pinned reference remains pending. |

## Observed transfers

- The 3,819,575,808-byte Rhythm Heaven Groove XCI completed its host session;
  the user confirmed Awoo reported installation success. The device requested
  non-sequential and backward ranges and legitimately skipped unneeded source
  regions, which drove the profile-specific `FullyServed` fix in `820a615`.
- The 7,111,486,912-byte Zumba NSP completed with nine range requests, seven
  non-sequential requests, four backward requests, and a maximum requested
  offset beyond 4 GiB. This is host-side completion evidence; device-side
  installation success was not separately recorded.
- Cable removal and reconnect were tested in one continuously running host.
  Reconnect did not claim resume and used a new serving-session identity.

Only the exact Awoo version and matrix that pass may be called `Verified`.
Until the remaining Partial rows pass, Awoo 1.6.2 remains protocol-family
compatible but not yet `Verified`.
