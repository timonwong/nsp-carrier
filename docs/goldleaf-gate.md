# Goldleaf 0.10+ real-device acceptance gate

Automated transcript, unit, fuzz, race, and build checks are prerequisites
only. They do not establish real-device compatibility.

Status: **Pending real-device validation**.

Target environment: Goldleaf 1.2.0 using its Remote PC browser. Record the
exact Goldleaf version, Switch firmware, NSP Carrier commit, host OS, libusb,
gousb, USB identity, speed, configuration, interface, and bulk endpoints.

## Acceptance matrix

| # | Required behavior | Result |
| ---: | --- | --- |
| 1 | Detect and claim `057e:3000` with Goldleaf selected | Pending |
| 2 | Show exactly one `Virtual` / `VIRT:/` drive and no `HOME` drive | Pending |
| 3 | List selected `.nsp` files in frozen catalog order and stat them correctly | Pending |
| 4 | Open and read an NSP through start/read/end, including non-zero and repeated ranges | Pending |
| 5 | Keep partial reads below `FullyServed`; reach `FullyServed` only after the whole source is served | Pending |
| 6 | Reject create, write, delete, and rename; show structured warnings and keep browsing | Pending |
| 7 | Serve a file larger than 4 GiB with 64-bit offsets | Pending |
| 8 | Complete a multi-file browsing/serving session | Pending |
| 9 | Stop within the bounded shutdown deadline | Pending |
| 10 | Cable removal becomes `Disconnected`; reconnect creates a fresh session without claiming resume | Pending |
| 11 | Compare the observable exchange with the pinned behavioral references | Pending |

Do not call any Goldleaf version `Verified` until its exact version and every
row above pass. Automated coverage establishes only implementation readiness.
