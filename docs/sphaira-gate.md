# Sphaira 1.0.6 real-device acceptance gate

Automated transcript, unit, fuzz, race, and build checks are prerequisites
only. They establish Compatible behavior, not real-device verification.

Status: **In progress. Sphaira 1.0+ is Compatible; no exact Sphaira version is
Verified.**

## Required environment record

- Sphaira 1.0.6 tag and exact revision
  `3f8303db00f33bfffa83ce0a1b750a1de14656e2`;
- Switch firmware 20.x;
- macOS 26.5.2 arm64 with Go 1.26.6;
- libusb 1.0.30 and gousb 1.1.3;
- USB `057e:3000` at SuperSpeed, configuration 1, interface 0, alternate
  setting 0, bulk IN endpoint 1, and bulk OUT endpoint 1.

## Acceptance matrix

| # | Required behavior | Result |
| ---: | --- | --- |
| 1 | Claim `057e:3000` with Sphaira selected, without probing another protocol | Pass |
| 2 | Complete the device-initiated opening exchange and preserve list/index order | Pass |
| 3 | Serve `.nsp`, `.nsz`, `.xci`, `.xcz`, and `.msp` individually | Partial: NSP and XCI pass; NSZ, XCZ, and MSP pending |
| 4 | Serve non-sequential and repeated ranges with valid payload integrity | Pass |
| 5 | Serve an offset above 4 GiB and a short final range | Partial: offset above 4 GiB passes; short final response pending |
| 6 | Complete a multi-file session; close files without completing early; complete only on QUIT/ACK | Pass |
| 7 | Stop within the bounded shutdown deadline | Pass |
| 8 | Classify cable removal as `Disconnected` | Pass |
| 9 | Reconnect with a fresh serving-session ID and no resume claim | Pass |
| 10 | Compare the exchange with the official Sphaira sender on the same device and source | Pending |

## Recorded evidence

- A 7,111,486,912-byte Zumba NSP installed successfully. Session
  `05b20f56-2bd0-4524-9ceb-14a1db0c38d9` reached `FullyServed`, `Completed`,
  and `Idle`, with offsets above 4 GiB and non-sequential/backward reads.
- Two 121,040-byte Samba de Amigo DLC NSPs installed successfully in one
  session. Session `b43d6740-8d55-4327-9c95-fb95109cbd3a` opened indexes 0
  and 1, independently closed both files, then completed through QUIT/ACK.
- Stop session `88613864-0b39-4d41-9fba-c7c10a3a3f60` returned through
  `Stopping` to `Idle` in about 112 ms without hanging.
- Cable-removal session `ff882db5-40ef-42d6-853a-63b5d4eb683a` reached typed
  `Disconnected`. After reconnect, fresh session
  `e882fb88-8db7-471d-8a8a-f9387f040a8a` started from a new handshake with no
  resume claim.
- A 3,819,575,808-byte Rhythm Heaven Groove XCI installed successfully.
  Session `e169465e-c69c-4c99-8884-6ad556bf7cab` accepted the XCI parser's
  non-close zero-length read, continued serving, and reached `FullyServed`,
  `Completed`, and `Idle`. Its 840 requests included seven non-sequential,
  three backward, and one repeated request.

For each format, record both host-observable transfer state and the separate
device-side installation result. Missing any row or format keeps Sphaira 1.0.6
Compatible but not Verified. Sphaira 0.13.3 and earlier are known incompatible
with this SPH0 profile.
