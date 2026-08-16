---
status: accepted
---

# Keep protocol and session ownership in Go

NSP Carrier keeps protocol codecs, the frozen catalog, serving-session state,
progress, cancellation, and USB lifecycle in a transport-independent Go core;
Wails and Svelte carry user intents and render typed snapshots rather than
owning or inferring that state. This preserves a headless real-device Gate 0,
makes protocol behavior testable with scripted transports, and prevents UI or
libusb details from becoming the application's source of truth.
