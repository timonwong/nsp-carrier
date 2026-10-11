# Build assets

Wails v3 uses this directory for platform metadata and build tasks. Application
outputs are written to `bin/`. Public signing, notarisation, and installers
remain out of scope for the USB MVP.

`appicon.svg` is the editable source for the application icon. Wails generates
macOS and Windows icons from the rendered 1024×1024 RGBA `appicon.png`.
