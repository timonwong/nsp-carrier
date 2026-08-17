# Sphaira 1.0.6 real-device acceptance gate

Automated transcript, unit, fuzz, race, and build checks are prerequisites
only. They establish Compatible behavior, not real-device verification.

Status: **Pending. Sphaira 1.0+ is Compatible; no exact Sphaira version is
Verified.**

## Required environment record

- Sphaira 1.0.6 tag and exact revision
  `3f8303db00f33bfffa83ce0a1b750a1de14656e2`;
- Switch firmware;
- host OS and architecture;
- libusb and gousb versions;
- USB configuration, interface, alternate setting, and bulk endpoints.

## Acceptance matrix

| # | Required behavior | Result |
| ---: | --- | --- |
| 1 | Claim `057e:3000` with Sphaira selected, without probing another protocol | Pending |
| 2 | Complete the device-initiated opening exchange and preserve list/index order | Pending |
| 3 | Serve `.nsp`, `.nsz`, `.xci`, `.xcz`, and `.msp` individually | Pending |
| 4 | Serve non-sequential and repeated ranges with valid payload integrity | Pending |
| 5 | Serve an offset above 4 GiB and a short final range | Pending |
| 6 | Complete a multi-file session; close files without completing early; complete only on QUIT/ACK | Pending |
| 7 | Stop within the bounded shutdown deadline | Pending |
| 8 | Classify cable removal as `Disconnected` | Pending |
| 9 | Reconnect with a fresh serving-session ID and no resume claim | Pending |
| 10 | Compare the exchange with the official Sphaira sender on the same device and source | Pending |

For each format, record both host-observable transfer state and the separate
device-side installation result. Missing any row or format keeps Sphaira 1.0.6
Compatible but not Verified. Sphaira 0.13.3 and earlier are known incompatible
with this SPH0 profile.
