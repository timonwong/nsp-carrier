# Goldleaf 0.10+ USB protocol notes

This document records independently observed wire behavior, not an official
protocol specification. The clean Go implementation uses these fixed
behavioral references without copying their implementations:

- `XorTroll/Goldleaf` and Quark commit
  [`5a4e86098d51ea9b9753af3cbfa7df71bf5b1234`](https://github.com/XorTroll/Goldleaf/tree/5a4e86098d51ea9b9753af3cbfa7df71bf5b1234)
- `developersu/ns-usbloader` commit
  [`9195fc42fc9de23837c015ad5dc0af2c3df85d73`](https://github.com/developersu/ns-usbloader/tree/9195fc42fc9de23837c015ad5dc0af2c3df85d73)

## Command blocks

Every command and response block is exactly 4096 bytes. Fields use
little-endian encoding and unused bytes are zero-filled.

| Offset | Width | Device request | Host response |
| --- | ---: | --- | --- |
| 0 | 4 | ASCII `GLCI` | ASCII `GLCO` |
| 4 | 4 | command ID (`uint32`) | result code (`uint32`) |
| 8 | variable | command arguments | response arguments |

Strings are a `uint32` UTF-8 byte length followed by exactly that many bytes.
The implementation bounds all decoding and encoding to the fixed block. A
malformed block, malformed argument, or unknown command is a terminal typed
protocol error.

Observed result codes are success `0`, exception caught `0xBAF1`, invalid
index `0xBAF2`, invalid file mode `0xBAF3`, and selection cancelled `0xBAF4`.
Path types are invalid `0`, file `1`, and directory `2`; file modes are read
`1`, write `2`, and append `3`.

## Read-only virtual catalog

The host advertises exactly one drive:

- label `Virtual`;
- path `VIRT`, presented by Goldleaf as `VIRT:/`;
- total size equal to the sum of the frozen catalog entries;
- free size zero.

No `HOME` drive, host filesystem root, special path, or host file picker is
exposed. `VIRT:/` is a flat directory whose files retain frozen catalog order.
Only selected `.nsp` basenames are accepted. Duplicate basenames, invalid
UTF-8, separators, control delimiters, and names too large for a `ReadFile`
command block are rejected before USB is opened.

The adapter implements the observed command IDs for drive discovery,
stat/list, start/read/end, mutations, special paths, and selection. Directory
count is zero; directory lookup and special-path lookup return invalid index;
file selection returns selection cancelled.

## File reads and progress

`ReadFile` command `9` contains a path string, source offset (`uint64`), and
size (`uint64`). A successful response contains the exact read size (`uint64`)
in its 4096-byte block, immediately followed by that many source bytes. The
adapter streams bounded chunks and records only successfully written bytes
against the catalog's stable source ID.

Goldleaf uses whole-source completion semantics. Serving a requested range is
observable progress but a partial read is never `FullyServed`. The host cannot
infer device-side installation success.

## Rejected mutations

`WriteFile`, `Create`, `Delete`, and `Rename` return exception caught without
changing the host filesystem. Each rejection produces a structured
`read-only-virtual-catalog` warning and the session continues. For `WriteFile`,
the host drains the declared inbound buffer before sending the failure response
so the next 4096-byte command remains aligned.

Reads and writes tolerate transport timeouts and short I/O. Cancellation,
disconnect, source mutation, malformed frames, and unexpected EOF return typed
errors. Reconnect always starts a fresh serving session; there is no resume
claim.
