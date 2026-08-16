# Awoo USB protocol notes

This document records independently observed wire behavior, not an official
protocol specification. The clean Go implementation uses these fixed
behavioral references without copying their implementations:

- `developersu/ns-usbloader` commit
  [`9195fc42fc9de23837c015ad5dc0af2c3df85d73`](https://github.com/developersu/ns-usbloader/tree/9195fc42fc9de23837c015ad5dc0af2c3df85d73)
- `Huntereb/Awoo-Installer` commit
  [`28364422c35efbf0c0ddff196458a6f49b8dff44`](https://github.com/Huntereb/Awoo-Installer/tree/28364422c35efbf0c0ddff196458a6f49b8dff44)

## Opening list

The host initiates the exchange with a 16-byte little-endian header followed
by the catalog payload:

| Offset | Width | Field |
| --- | ---: | --- |
| 0 | 4 | ASCII magic `TUL0` |
| 4 | 4 | UTF-8 catalog byte length (`uint32`) |
| 8 | 8 | zero padding |

The payload is a newline-separated, newline-terminated list of flat
basenames. Duplicate basenames, invalid UTF-8, path separators, and names
containing control delimiters are rejected before USB is opened. The same
wire-name predicate is used for outbound catalog entries and inbound range
requests. The encoded list is counted first and capped at 4 MiB before
allocation.

## Command header

The Switch sends a 32-byte little-endian command header:

| Offset | Width | Field |
| --- | ---: | --- |
| 0 | 4 | ASCII magic `TUC0` |
| 4 | 1 | command type (`0` request, `1` response) |
| 5 | 3 | zero padding |
| 8 | 4 | command ID (`uint32`) |
| 12 | 8 | payload or response size (`uint64`) |
| 20 | 12 | zero reserved bytes |

Known device request IDs are exit `0`, default file range `1`, and alternate
file range `2`. Both range variants use the same payload and host response.
Unknown commands and non-zero reserved bytes are terminal protocol errors.

## File range

The range command payload is:

| Offset | Width | Field |
| --- | ---: | --- |
| 0 | 8 | requested size (`uint64`) |
| 8 | 8 | source offset (`uint64`) |
| 16 | 8 | UTF-8 basename byte length (`uint64`) |
| 24 | 8 | zero padding |
| 32 | variable | basename |

The host validates the complete bounded payload, frozen source identity, and
exact requested range. It responds with a 32-byte `TUC0` response header using
range command ID `1` and `dataSize` equal to the requested size, then streams
exactly that many source bytes. Unlike DBI's observed aligned-tail exception,
Awoo range requests that cross EOF are rejected.

Successfully written chunks are reported to the shared host progress tracker
against the stable source ID. The adapter does not infer installation state.

## Exit and defensive behavior

Exit `0` has no payload or host response and completes only the host serving
session. Reads and writes tolerate transport timeouts and short I/O; context
cancellation, disconnect, source mutation, invalid UTF-8/names, invalid
lengths, and unexpected EOF return typed errors without padding or unbounded
allocation.
