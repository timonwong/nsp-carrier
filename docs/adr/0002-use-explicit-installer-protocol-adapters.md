---
status: accepted
---

# Use explicit installer protocol adapters

NSP Carrier supports DBI, Awoo USB, and the Goldleaf 0.10+ protocol family as
separate clean-room adapters behind one host-session seam, reusing the frozen
catalog, host-observable state, raw USB transport, and safe connection
lifecycle. The user selects an installer profile before Start because all
three installers use USB device `057e:3000` but have incompatible handshakes;
probing one protocol can corrupt another protocol's opening exchange.

Goldleaf exposes only a read-only virtual catalog drive: it does not expose the
host home directory and rejects create, write, delete, and rename operations.
Awoo Network remains deferred with HTTP. RCM payload injection and split-file
creation or merging are outside the product boundary. Upstream projects are
fixed-revision behavioral references only; their GPL implementations are not
copied into the MIT codebase.

The application and diagnostics CLI call one deep host-session runner. Its
small external interface accepts an installer profile, frozen source catalog,
USB link, and observer; adapter selection and wire behavior remain behind the
seam. The frozen source catalog is protocol-neutral, while each profile owns
its validation, capabilities, wire namespace, and protocol adapter. No shared
cross-protocol command model is introduced.

Go owns an immutable profile registry and exposes typed capabilities to every
caller; Wails does not duplicate protocol knowledge. Adapters report served
byte ranges against stable source IDs, while the host-session runner owns the
shared progress and file-state model. It also owns reconnect behavior: a
disconnect may retain the frozen source catalog and selected profile, but each
reconnect creates a fresh serving session and never promises transfer resume.

Goldleaf filesystem mutations receive protocol-defined negative responses and
structured warnings without ending an otherwise valid session. Malformed
frames and commands that cannot be handled safely remain terminal protocol
errors.

The installer profile can change only while the application is Idle. Changing
it revalidates the selected queue without silently changing selection, and the
last profile is persisted; missing or legacy settings default to DBI. Protocol
family compatibility and exact-version real-device verification are separate
claims. Compatible but unverified versions may run with a warning, while only
known-incompatible versions are blocked.
