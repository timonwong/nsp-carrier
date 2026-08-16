# Awoo USB real-device acceptance gate

Automated transcript, fuzz, race, and build checks are prerequisites only.
They do not establish real-device compatibility. This gate is independent of
the DBI Gate 0 evidence.

Status: **Passed on Awoo Installer 1.6.2**.

## Recorded environment

- Awoo Installer 1.6.2; exact revision unavailable;
- Switch firmware unconfirmed, reported as approximately version 20;
- NSP Carrier commits `820a615` through `aebbe18`;
- macOS 26.5.2 arm64, libusb 1.0.30, and gousb 1.1.3;
- USB `057e:3000`, SuperSpeed, configuration 1, interface 0, alternate 0,
  bulk IN endpoint 1, and bulk OUT endpoint 1.

## Acceptance matrix

| # | Required behavior | Result | Evidence |
| ---: | --- | --- | --- |
| 1 | Detect and claim `057e:3000` with the Awoo profile selected | Pass | Awoo 1.6.2 was claimed repeatedly with the recorded SuperSpeed configuration and endpoints. |
| 2 | Complete the host-initiated `TUL0` list exchange | Pass | The device listed and requested both the XCI and NSP supplied by the host. |
| 3 | Serve `.nsp`, `.nsz`, `.xci`, and `.xcz` selections exercised by the device | Pass | `.nsp` and `.xci` completed in the earlier runs. Awoo 1.6.2 later installed a verified 23,108-byte `.nsz` after six command-1 ranges and a verified 2,969,537,618-byte `.xcz` after eight command-1 ranges; both host sessions reached `FullyServed`, `Completed`, and `Idle`. |
| 4 | Complete both observed range-command variants where the device emits them | Pass | A bounded real-device trace showed Awoo 1.6.2 emitting command ID 1 for all six ranges and the host responding with ID 1. This version did not emit ID 2; fixed automated transcripts cover both IDs 1 and 2. |
| 5 | Serve offsets beyond 4 GiB and a large range/file | Pass | A 7,111,486,912-byte NSP completed with maximum requested offset 7,110,078,912. |
| 6 | Complete a multi-file session and device exit command | Pass | Two 121,040-byte Samba de Amigo DLC NSPs each reached `FullyServed`, followed by device exit and `Completed`. |
| 7 | Stop returns within the bounded shutdown deadline | Pass | After `a58e37d`, real-device Ctrl-C emitted `Stopping`, returned to `Idle`, and exited 0 without a timeout. |
| 8 | Cable removal becomes `Disconnected` without hang or panic | Pass | Removal after 909,120,000 XCI bytes emitted typed `Disconnected` and retained the waiting runner. |
| 9 | Reconnect creates a fresh serving-session ID without claiming resume | Pass | Session `c33c03ba-2f0c-45d9-84b6-c5c9eeec2f69` disconnected; reconnect was claimed as fresh session `97b17d1a-e5c8-4581-9dc4-bf717f50bde3`. |
| 10 | Compare the observable exchange with the pinned behavioral reference | Pass | Session `40a6d247-e79d-4eb5-8778-35c6d2f62533` matched the pinned command-1 exchange: outbound `TUL0` list, six inbound command-1 ranges with command-1 responses, then inbound command-0 exit. |

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
- A two-file session served the AiAi Accessories and AiAi Costume DLC NSPs.
  Each file completed six range requests, including non-sequential and backward
  access, reached `FullyServed`, and was followed by device exit.
- A payload-safe bounded trace of a later AiAi Accessories session recorded the
  exact Awoo 1.6.2 command sequence without file content or wire names. The
  device used command ID 1 for six ranges at offsets 0, 16, 114,960, 118,544,
  119,248, and 272, and the host responded with command ID 1 and the exact
  requested payload sizes. The device then sent command ID 0 and the runner
  completed. This matches the pinned command-1 behavioral transcript; command
  ID 2 remains covered by the separate fixed transcript because this device
  version did not emit it.
- The remaining compressed-format evidence used NSZ v5.0.0 from
  `nicoboss/nsz` commit `86cdfbe7c2d5df21b41613699026d15548fa1986`
  with `--verify --keep`. The 121,040-byte AiAi Accessories NSP source had
  SHA-256 `7059d712141b18cad2dd554c7face5ac1b6945d50eca03262689cdd9c45f7c13`;
  its 23,108-byte NSZ had SHA-256
  `1a3e410f9a90e8fa431744a746ba9bd352b0c7ca299e6ee46b14dc8c6f962497`.
  Verification reconstructed the source NSP with the same SHA-256. Awoo
  requested command ID 1 ranges `(0,16)`, `(16,256)`, `(17028,3584)`,
  `(20612,704)`, `(21316,1792)`, and `(272,16756)`, covering all 23,108 source
  bytes. Session `3151f8dc-5541-40c5-969d-03e4aa78e424` reached `FullyServed`,
  `Completed`, and `Idle`; the user confirmed installation success on Awoo.
- The 3,819,575,808-byte Rhythm Heaven Groove XCI source had SHA-256
  `baccfc2de5c52cd3554dfc66041b20c13e57e06a44cd3637ee21c964ff6989ff`;
  its block-compressed 2,969,537,618-byte XCZ had SHA-256
  `e49813862e607109cf919c58762ed99e52e82327caa38930abf7dd46c1e43e08`.
  NSZ verified every compressed or retained NCA. Awoo requested command ID 1
  ranges `(61440,16)`, `(61456,282)`, `(290460672,16)`, `(290460688,409)`,
  `(2969534034,3584)`, `(290493440,2677629010)`, `(2968122450,1281024)`, and
  `(2969403474,130560)`, serving 2,679,044,901 unique bytes with a maximum
  requested offset of 2,969,534,034. Session
  `013d5746-b6f4-4517-9ea9-e4f2d4221079` reached `FullyServed`, `Completed`,
  and `Idle`; the user confirmed installation success on Awoo.

Only the exact Awoo version and matrix that pass may be called `Verified`.
Awoo Installer 1.6.2 is `Verified` for the matrix above; other versions remain
protocol-family compatible until separately verified.
