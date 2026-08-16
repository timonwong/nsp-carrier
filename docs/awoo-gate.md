# Awoo USB real-device acceptance gate

Automated transcript, fuzz, race, and build checks are prerequisites only.
They do not establish real-device compatibility. This gate is independent of
the DBI Gate 0 evidence.

Status: **Pending real-device execution**.

## Environment to record

- exact Awoo Installer version and revision, if available;
- Switch firmware;
- NSP Carrier commit;
- OS, architecture, libusb, and gousb versions;
- USB configuration, interface, alternate setting, and bulk endpoints.

## Acceptance matrix

| # | Required behavior | Result | Evidence |
| ---: | --- | --- | --- |
| 1 | Detect and claim `057e:3000` with the Awoo profile selected | Pending | |
| 2 | Complete the host-initiated `TUL0` list exchange | Pending | |
| 3 | Serve `.nsp`, `.nsz`, `.xci`, and `.xcz` selections exercised by the device | Pending | |
| 4 | Complete both observed range-command variants where the device emits them | Pending | |
| 5 | Serve offsets beyond 4 GiB and a large range/file | Pending | |
| 6 | Complete a multi-file session and device exit command | Pending | |
| 7 | Stop returns within the bounded shutdown deadline | Pending | |
| 8 | Cable removal becomes `Disconnected` without hang or panic | Pending | |
| 9 | Reconnect creates a fresh serving-session ID without claiming resume | Pending | |
| 10 | Compare the observable exchange with the pinned behavioral reference | Pending | |

Only the exact Awoo version and matrix that pass may be called `Verified`.
Until then the profile remains protocol-family compatible but unverified.
