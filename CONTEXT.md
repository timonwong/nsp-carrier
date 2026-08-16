# NSP Carrier

NSP Carrier is an Omni host for NS installers that exposes selected local
content while reporting only what the host can observe.

## Language

**Omni host for NS installers**:
The PC-side application that exposes selected local files through multiple
installer protocols without performing or confirming installation itself.
_Avoid_: DBI USB file service, PC installer, uploader

**Installer protocol**:
A wire contract between the Omni host and a Switch-side installer,
such as DBI0, Awoo, or Goldleaf.
_Avoid_: NS-USBloader protocol

**Installer profile**:
The explicit user-selected installer protocol used for the next serving
session: DBI, Awoo USB, or Goldleaf.
_Avoid_: Mode, target, auto-detected protocol

**Profile capabilities**:
The host-enforced properties of an installer profile, including its supported
content formats, transport, wire namespace, and filesystem semantics.
_Avoid_: Attributes, feature flags

**Compatible**:
An installer version belongs to the protocol family an installer profile is
designed to serve, but has not necessarily passed a real-device gate.
_Avoid_: Supported, verified

**Verified**:
An exact installer version has passed its profile's recorded real-device
acceptance matrix.
_Avoid_: Compatible, probably works

**Frozen catalog**:
The immutable snapshot of selected files and their identities used by one
serving session.
_Avoid_: Queue snapshot, live queue

**Serving session**:
One host-observable connection lifecycle with its own identity and terminal
state; a reconnect creates a new serving session.
_Avoid_: Install session, resumable session

**Fully served**:
The host has successfully sent every byte requested for a file during a
serving session; it does not mean the file was installed.
_Avoid_: Installed, installation succeeded, uploaded

**Virtual catalog drive**:
The read-only Goldleaf drive that projects the frozen catalog without exposing
the host home directory or allowing filesystem mutation.
_Avoid_: Home drive, shared folder
