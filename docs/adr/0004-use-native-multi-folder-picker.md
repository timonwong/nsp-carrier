---
status: accepted
---

# Use a native multi-folder picker outside Wails

Wails v2.14 exposes `OpenMultipleFilesDialog` and a single-path
`OpenDirectoryDialog`, but no folders-only multi-select API. Darwin's
`NSOpenPanel` can combine `CanChooseDirectories` with
`AllowsMultipleSelection`; Windows `IFileOpenDialog` can combine
`FOS_PICKFOLDERS` with `FOS_ALLOWMULTISELECT`. The Wails Darwin frontend
enables multiple selection only when files are allowed, and its Windows
folder dialog never sets `FOS_ALLOWMULTISELECT`.

NSP Carrier therefore opens its own folders-only native picker from
`internal/dialog` for macOS and Windows, while `Add files` continues to use
the Wails file dialog. Linux, which is not a verified target, keeps the
existing Wails single-folder dialog so the unverified platform does not
silently change. Native dialog titles stay English and are not routed
through the frontend UI locale.
