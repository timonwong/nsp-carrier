# Sphaira 1.x SPH0 USB protocol notes

This document records independently reviewed wire behavior, not an official
protocol specification. The MIT Go adapter is clean-room code based on the
fixed Sphaira 1.0.6 behavioral reference
[`3f8303db00f33bfffa83ce0a1b750a1de14656e2`](https://github.com/NaGaa95/sphaira/tree/3f8303db00f33bfffa83ce0a1b750a1de14656e2).
No GPL implementation or test fixture is copied, translated, or vendored.

Sphaira 0.13.3 and earlier use the incompatible TUL0/TUC0 generation. SPH0
replaced that flow before Sphaira 1.0.0; the current all-packet CRC32C form was
established by Sphaira commit `22e965521a1fe3723cabd77d5521ae591c28fabf`.

## Packet

Every control packet is exactly 24 bytes: six little-endian `uint32` fields.

| Offset | Width | Field |
| --- | ---: | --- |
| 0 | 4 | magic `0x53504830` (wire bytes `30 48 50 53`) |
| 4 | 4 | command/result, or range offset high 32 bits |
| 8 | 4 | argument, or range offset low 32 bits |
| 12 | 4 | argument, or requested size |
| 16 | 4 | argument, or payload CRC32C |
| 20 | 4 | header CRC32C over bytes 0–19 |

CRC32C uses the Castagnoli polynomial; `123456789` produces `e3069283`.
Bad packet length, magic, or header CRC is terminal and receives no response.
A structurally valid unknown command, invalid index/range, or source failure
receives result `1` (`RESULT_ERROR`) before the session terminates.

## Device-initiated flow

1. Sphaira sends its initial command. The host validates it before writing.
2. The host sends result `0` (`RESULT_OK`) with the filename-list byte length,
   then a newline-separated, newline-terminated list of unique flat basenames
   in frozen catalog order.
3. Sphaira sends OPEN command `1` with a zero-based file index.
4. The host replies with the source size. The low 32 bits are in argument 4;
   the high 16 bits and flags occupy argument 3. Stream flags are always zero.
5. Sphaira sends range packets until it sends the exact close packet: zero
   offset, zero size, and zero payload CRC. The host ACKs file close.
6. Only final QUIT command `0` followed by a successful ACK gracefully
   completes the serving session.

The list accepts `.nsp`, `.nsz`, `.xci`, `.xcz`, and `.msp`. Empty or unsafe
UTF-8 names, NUL/newline/path separators, duplicate basenames, lists or indexes
outside `uint32`, and sources larger than the 48-bit size field are rejected
before USB is opened. `.rar` and SPH0 stream mode are not supported.

## Ranges, progress, and errors

Offsets are 64-bit and requested sizes are `uint32`, capped at 16 MiB. A valid
request may end beyond EOF; the response carries the actual short length and
payload CRC32C, followed by exactly that many bytes. Offset beyond EOF,
arithmetic overflow, non-close zero size, and oversized ranges are rejected.

Only successfully written payload bytes enter progress. Repeated and
out-of-order intervals are unioned, and a source is `FullyServed` only when
that union covers the entire frozen source. File close does not override
partial progress. Source identity, size, and modification time are rechecked
for every range; mutation and unexpected EOF fail the session.

Stop uses context cancellation and bounded USB shutdown. Cable removal is
`Disconnected`, not completion or protocol failure. Reconnect creates a fresh
serving-session ID and does not claim resume.

Payload-safe traces are capped at 300 records per session. They may contain
direction, operation, command/result, source ID, index/range sizes, and boolean
integrity verdicts. They never contain local paths, wire names, raw packets,
payload bytes, checksum values, or other content fingerprints.

The synthetic transcript in `internal/sphaira/testdata` pins handshake, list,
open, non-zero range, payload response, close, and quit behavior with explicit
clean-room derivation metadata.
