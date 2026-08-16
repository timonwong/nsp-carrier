# Build assets

Wails v2 uses this directory for platform metadata and generated application
bundles. Public signing, notarisation, installers, and release packaging remain
out of scope for the USB MVP.

`appicon.svg` is the editable source for the application icon. Wails consumes
the rendered 1024×1024 RGBA `appicon.png` when packaging native bundles.
