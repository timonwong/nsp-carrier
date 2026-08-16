# Goldleaf 0.10+ real-device acceptance gate

Automated transcript, unit, fuzz, race, and build checks are prerequisites
only. They do not establish real-device compatibility.

Status: **Pass for Goldleaf 1.2.0 in the recorded environment. Every matrix row
has real-device or pinned-reference evidence.**

## Recorded environment

- Goldleaf 1.2.0 using its Remote PC browser; the USB serial descriptor also
  reported `1.2.0`;
- Goldleaf tag `1.2.0` commit `c43b31caa935338f43313394daa0f59810803507`
  was used to confirm its Remote PC UI shortcuts;
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
| 5 | Keep partial reads below `FullyServed`; reach `FullyServed` only after the whole source is served | Pass |
| 6 | Reject create, write, delete, and rename; show structured warnings and keep browsing | Pass |
| 7 | Serve a file larger than 4 GiB with 64-bit offsets | Pass |
| 8 | Complete a multi-file browsing/serving session | Pass |
| 9 | Stop within the bounded shutdown deadline | Pass |
| 10 | Cable removal becomes `Disconnected`; reconnect creates a fresh session without claiming resume | Pass |
| 11 | Compare the observable exchange with the pinned behavioral references | Pass |

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
  though the device reported installation success.
- A later multi-file session `1ebf9845-a60f-4873-9ae4-9ff6a48e4c3b`
  served two 121,040-byte Samba de Amigo DLC NSPs. Goldleaf reported
  installation success for both files in that one serving session.
  AiAi Accessories reached `FullyServed` after 14 range requests, including
  seven non-sequential, four backward, and one repeated request; it served
  121,040 unique bytes and 127,632 wire bytes. AiAi Costume reached
  `FullyServed` after 15 range requests, including eight non-sequential, four
  backward, and two repeated requests; it served 121,040 unique bytes and
  128,144 wire bytes. Both maximum requested offsets were 119,248. This proves
  the positive whole-source transition while preserving the earlier partial
  large-file result, and passes matrix rows 5 and 8.
- Goldleaf's delete action displayed success, but the host rejected the
  mutation: the local Zumba NSP retained its exact size, modification time, and
  SHA-256, and it reappeared after leaving and re-entering `VIRT:/`. Goldleaf
  1.2.0 does not surface the adapter's negative delete result in this UI path.
  The session remained usable. The multi-file session repeated this behavior
  after each installation: both delete requests emitted structured read-only
  warnings and the second file still installed successfully.
- The remaining mutations were exercised through the Goldleaf 1.2.0 Remote PC
  UI shortcuts documented by its exact tagged source. Rename in session
  `ccd4727d-0177-4a80-91ca-d69a713ada1c` sent command 14 and received
  `0xBAF1`; the original file retained size, modification time, and SHA-256 and
  reappeared after refresh. In session
  `3fbfa8fa-e698-43b4-b808-7d556c3987b0`, the `L` create-file shortcut sent
  command 12 for `test.txt` and received `0xBAF1`. Copying a small SD-card
  settings file and pressing `X` to paste sent StartFile in write mode
  (`0xBAF3`), WriteFile command 10 (`0xBAF1`), and EndFile in write mode
  (`0xBAF3`). Neither target appeared after refresh, structured create/write/
  rename warnings were emitted, and browsing continued. Goldleaf displayed
  success for these actions despite the negative protocol results; this is a
  Goldleaf 1.2.0 UI false positive, not a host mutation.
- Cable removal during session `2e053ecb-5b72-4abe-afb1-6f5633dbc6c3`
  produced typed `Disconnected` with `transport disconnected: transfer was
  cancelled`, then returned to `WaitingForDevice` without exiting. Reconnecting
  was claimed as fresh session `01741089-1691-4c0f-8233-112a730ef38f`, which
  entered `Serving` without claiming resume.
- Payload-safe bounded traces captured the real Goldleaf 1.2.0 exchange without
  file content, wire names, or local paths. Session
  `7b83e6b0-e02a-4d41-97d5-ba3c1b7cddbd` matched drive discovery and catalog
  commands 1, 2, 3, 4, 5, 6, and 15 with success result 0. Session
  `502c95de-6423-4db9-900e-d3b1e279c5ba` matched start/read/end commands 8, 9,
  and 11, including exact payload sizes and 64-bit ranges, and matched delete
  command 13 with read-only result `0xBAF1`. These command IDs, directions,
  results, and payload semantics match the pinned behavioral transcripts.
- Ctrl-C after the device reported success emitted `Stopping`, returned to
  `Idle`, and exited 0 within the bounded shutdown path.
- Running macOS USB descriptor enumeration concurrently with the host caused
  one transient exclusive-open collision during claim. Retrying without a
  parallel enumerator claimed the same Goldleaf device immediately. Do not run
  `system_profiler` or equivalent USB enumeration during a gate claim.

Goldleaf 1.2.0 is `Verified` only for the exact version and matrix recorded
above. Other 0.10+ versions remain protocol-family compatible unless they gain
their own complete real-device matrix.
