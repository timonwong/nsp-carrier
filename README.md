<p align="center">
  <img src="docs/assets/nsp-carrier-logo.svg" alt="NSP Carrier logo" width="96" />
  <br />
  <strong style="font-size: 32px">NSP Carrier</strong>
</p>

[简体中文](README.cn.md) · English

**NSP Carrier** is a desktop app that serves `.nsp`, `.nsz`, `.xci`, `.xcz`,
and Sphaira-only `.msp` files to the installer running on your Switch over USB
— Awoo, Goldleaf, Sphaira, or DBI. Add files to the queue, pick the matching installer
profile, and serve; the installer on the Switch performs the actual install.

The app reports only what it can observe over USB: which files were served
and how far each transfer got. It is not an installer, an MTP server, or
proof that a title was installed.

![NSP Carrier desktop application](docs/assets/nsp-carrier-desktop.jpg)

Representative queue and transfer-progress state:

![NSP Carrier serving files with transfer progress](docs/assets/nsp-carrier-transfer-progress.png)

## Features

- Serves files over USB through an explicitly selected installer profile:
  - [Awoo](https://github.com/Huntereb/Awoo-Installer)
  - [Goldleaf 0.10+](https://github.com/XorTroll/Goldleaf)
  - [Sphaira 1.0+](https://github.com/NaGaa95/sphaira)
  - DBI
- File and recursive folder selection, with drag and drop.
- Queue with search and duplicate-basename validation.
- Per-file unique-byte progress, bounded activity logs, and typed errors.
- Start/Stop with a host-owned session lifecycle — a reconnect starts a
  fresh session and never claims transfer resumption.
- Auto/Light/Dark appearance.
- English and Simplified Chinese UI, selectable from the top-right language menu.

The host reports only what it can observe: `FullyServed` means the selected
profile's byte-coverage rule was satisfied, not that the title was installed.

## Getting started

`nsp-carrier` is a desktop app. It serves the files you select to an
installer running on your Switch over USB; it never writes to the Switch, so
installation always happens from the installer's side.

You need:

- a Switch running a matching installer in USB mode — Awoo Installer 1.6.2,
  Goldleaf 0.10+, Sphaira 1.0+, or DBI;
- a USB cable between the PC and the Switch;
- a copy of the app — download it from the
  [Releases](https://github.com/timonwong/nsp-carrier/releases) page (see
  [Installing](#installing)).

Basic flow:

1. Add `.nsp`, `.nsz`, `.xci`, `.xcz`, or `.msp` files and folders to the queue
   (drag and drop works too).
2. Pick the profile that matches your installer: Awoo, Goldleaf, Sphaira, or DBI.
3. Start serving, then install from the installer on the Switch.
4. Watch per-file progress in the app. `FullyServed` means the selected
   profile's observable byte-coverage rule was satisfied — not that the title
   was installed.

A file the selected profile cannot serve blocks Start with a clear
validation error rather than being silently skipped.

Platform setup — `libusb` on macOS, a USB driver on Windows — is covered in
[Installing](#installing).

## Installing

Download the app from the
[Releases](https://github.com/timonwong/nsp-carrier/releases) page:

- **macOS:** unzip `nsp-carrier-macos-arm64.zip` and move `NSP Carrier.app`
  into Applications.
- **Windows:** unzip `nsp-carrier-windows-amd64.zip` and run
  `nsp-carrier.exe`.

Releases are not code-signed or notarised. On macOS, open the app the first
time with right-click → Open to bypass Gatekeeper.

### Platform prerequisites

#### macOS

macOS uses `libusb` for USB access. Install it with Homebrew:

```sh
brew install libusb
```

#### Windows

The Switch exposes itself as a raw USB device, so Windows needs a compatible
USB driver before `nsp-carrier` can see it. Windows 10 and later may already
have a suitable built-in WinUSB binding, so try starting `nsp-carrier` first.
If it cannot see the device or the binding is incompatible, use
[Zadig](https://zadig.akeo.ie/):

1. Download and run Zadig from <https://zadig.akeo.ie/>.
2. Plug the Switch into the PC with a USB cable.
3. Put your installer into USB mode — on DBI, enter its USB mode
   (DBIbackend); on Awoo, choose USB install; on Goldleaf, open the Remote PC
   browser; on Sphaira, open its USB install flow.
4. In Zadig, open *Options → List All Devices* so the Switch appears.
5. Select the Switch from the dropdown — the vendor ID is `057E` (Nintendo)
   and the product often appears as `DBI`, `USB composite device`, or
   `057E:3000`. Choose the matching entry.
6. Pick **libusbK** as the target driver (or *WinUSB* if libusbK is not
   available).
7. Click *Replace Driver* (or *Install Driver*) and wait for it to finish.
8. Start `nsp-carrier` and confirm it sees the device before serving.

For source development instructions, see [DEVELOP.md](DEVELOP.md).

## License

MIT.

## Credits

The DBI, Awoo, Goldleaf, and Sphaira protocol implementations were developed from
observed wire behavior, using these upstream projects as behavioral
references:

- [`rashevskyv/dbibackend-qt`](https://github.com/rashevskyv/dbibackend-qt) — DBI backend reference
- [`developersu/ns-usbloader`](https://github.com/developersu/ns-usbloader) — Awoo and Goldleaf wire references
- [`Huntereb/Awoo-Installer`](https://github.com/Huntereb/Awoo-Installer) — Awoo reference
- [`XorTroll/Goldleaf`](https://github.com/XorTroll/Goldleaf) — Goldleaf reference
- [`NaGaa95/sphaira`](https://github.com/NaGaa95/sphaira/tree/3f8303db00f33bfffa83ce0a1b750a1de14656e2) — fixed Sphaira 1.0.6 behavioral reference

Sphaira 1.0+ is currently **Compatible**, based on fixed-revision behavior and
automated tests. No exact Sphaira version is **Verified** until every row in
the real-device matrix passes. Sphaira 0.13.3 and earlier use the incompatible
legacy TUL0/TUC0 generation.
