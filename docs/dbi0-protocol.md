# DBI0 protocol notes

This document records observed behavior, not an official protocol
specification. The clean Go implementation is based on behavior in these fixed
upstream revisions:

- `rashevskyv/dbi` commit `ba104f1701d0284e626a4c99e55d2353a4369aa2`
- `rashevskyv/dbibackend-qt` commit
  `c7947e363cdc9062eba3adc50e5f939bdedd22f2`

## Transport

DBI exposes USB device `057e:3000`. The reference backend selects a USB
configuration/interface containing bulk IN and OUT endpoints. Descriptor
coordinates are treated as discovered runtime facts, not permanent constants.

## Frame

Every header is 16 bytes, little-endian:

| Offset | Width | Field |
| --- | ---: | --- |
| 0 | 4 | ASCII magic `DBI0` |
| 4 | 4 | command type |
| 8 | 4 | command ID |
| 12 | 4 | payload size |

Known command types are request `0`, response `1`, and acknowledgement `2`.
Known command IDs are exit `0`, legacy list `1`, file range `2`, and list `3`.
The reference implementations define but do not dispatch legacy list `1`, so
it is not initially supported without device evidence.

## List

For command `3`, the host returns a response header whose payload size is the
byte length of UTF-8 basenames separated by `\n` and terminated by `\n`. When
non-empty, it waits for a 16-byte acknowledgement before writing the list.

Only the frozen, checked catalog is listed. Duplicate basenames are rejected
before a session begins.

## File range

For command `2`:

1. Host acknowledges the request header.
2. Device sends a detail payload:

   | Offset | Width | Field |
   | --- | ---: | --- |
   | 0 | 4 | range size (`uint32`) |
   | 4 | 8 | file offset (`uint64`) |
   | 12 | 4 | UTF-8 basename byte length (`uint32`) |
   | 16 | variable | basename |

3. Host sends a response header carrying the range size.
4. Host waits for the final acknowledgement.
5. Host reads the frozen source at the requested offset and writes exactly the
   requested bytes in chunks no larger than 1 MiB.

A file can exceed 4 GiB; one command cannot because range size is `uint32`.
Large files are served through multiple ranges with 64-bit offsets.

## Exit

For command `0`, the host returns an empty response. This ends the host session
but is not proof of Switch-side installation success.

## Defensive rules

- Validate magic, type, command, declared size, and actual bytes read.
- Cap request detail payload at 64 KiB.
- Cap basename at 4 KiB; require valid UTF-8 and reject NUL, `/`, `\\`, `.`,
  and `..`.
- Require declared basename length to equal the remaining payload.
- Reject integer overflow and ranges beyond the frozen source size.
- Reject unsupported commands and malformed frames without panic or unbounded
  allocation.
