<p align="center">
  <img src="docs/assets/nsp-carrier-logo.svg" alt="NSP Carrier logo" width="96" />
  <br />
  <strong style="font-size: 32px">NSP Carrier</strong>
</p>

[简体中文](README.cn.md) · English

**NSP Carrier** is a desktop app that sends Switch game files to an installer
on your Switch over USB. It works with Awoo, Goldleaf, Sphaira, and DBI, on
macOS and Windows.

It only serves files. The installer on the Switch does the install, and
NSP Carrier never writes to the Switch — served is not the same as installed.

![NSP Carrier desktop application](docs/assets/nsp-carrier-desktop.png)

## Usage

1. On the Switch, open your installer and put it in USB mode.
2. Connect the Switch to your computer with a USB cable.
3. In NSP Carrier, pick the profile that matches the installer.
4. Add files or folders to the queue. Drag and drop works; folders are
   scanned recursively.
5. Click **Start**, then confirm the install on the Switch.

The app shows per-file progress while the installer reads each file. The
interface is available in English and Simplified Chinese, with light and dark
themes; switch both from the bottom of the sidebar.

![NSP Carrier serving files with transfer progress](docs/assets/nsp-carrier-transfer-progress.png)

## Supported installers

| Profile | Formats | Verified on real hardware |
| --- | --- | --- |
| [Awoo](https://github.com/Huntereb/Awoo-Installer) | `.nsp` `.nsz` `.xci` `.xcz` | 1.6.2 |
| [Goldleaf](https://github.com/XorTroll/Goldleaf) 0.10+ | `.nsp` | 1.2.0 |
| [Sphaira](https://github.com/NaGaa95/sphaira) 1.0+ | `.nsp` `.nsz` `.xci` `.xcz` `.msp` | — (compatible, not yet verified) |
| DBI | `.nsp` `.nsz` `.xci` `.xcz` | — |

- If a queued file is not supported by the selected profile, **Start** is
  blocked with an error for that file. Files are never skipped silently.
- Sphaira 0.13.3 and earlier use an older, incompatible protocol and are not
  supported.

## Install

Download the latest build from
[Releases](https://github.com/timonwong/nsp-carrier/releases):

| Platform | File | Notes |
| --- | --- | --- |
| macOS (Apple Silicon) | `nsp-carrier-macos-arm64.zip` | Unzip, move `NSP Carrier.app` to Applications. `libusb` is bundled. |
| Windows (x64) | `nsp-carrier-windows-amd64.zip` | Unzip, run `nsp-carrier.exe`. May need a USB driver (below). |

Builds are not code-signed or notarised.

### macOS: first launch

macOS blocks the app the first time you open it. Open it once, then go to
**System Settings → Privacy & Security** and click **Open Anyway**. On older
macOS versions, right-click the app and choose **Open** instead.

### Windows: USB driver

Windows 10 and later often bind a working WinUSB driver automatically, so
try NSP Carrier first. If it cannot see the Switch, install a driver with
[Zadig](https://zadig.akeo.ie/):

1. Connect the Switch and put the installer in USB mode
   (DBI: DBIbackend · Awoo: USB install · Goldleaf: Remote PC · Sphaira: USB install).
2. Run Zadig and enable **Options → List All Devices**.
3. Select the Switch. Its vendor ID is `057E` (Nintendo); the name is often
   `DBI`, `USB composite device`, or `057E:3000`.
4. Choose **libusbK** as the driver (or **WinUSB** if libusbK is missing),
   then click **Replace Driver** or **Install Driver**.
5. Start NSP Carrier and check that it sees the device.

## What the app reports

- **Served** means the installer read every byte the selected profile
  requires. It does not mean the title was installed.
- If the cable is unplugged or the installer restarts, the next connection
  starts a new session. Transfers are not resumed.
- Linux builds are compiled in CI but are not published and have not been
  tested on a real device.

## Building from source

See [DEVELOP.md](DEVELOP.md) for build steps, the developer USB CLI, and
protocol notes.

## License

[MIT](LICENSE).

## Credits

The USB protocols were implemented from observed wire behavior, using these
projects as references:

- [`rashevskyv/dbibackend-qt`](https://github.com/rashevskyv/dbibackend-qt) — DBI backend
- [`developersu/ns-usbloader`](https://github.com/developersu/ns-usbloader) — Awoo and Goldleaf wire formats
- [`Huntereb/Awoo-Installer`](https://github.com/Huntereb/Awoo-Installer) — Awoo
- [`XorTroll/Goldleaf`](https://github.com/XorTroll/Goldleaf) — Goldleaf
- [`NaGaa95/sphaira`](https://github.com/NaGaa95/sphaira/tree/3f8303db00f33bfffa83ce0a1b750a1de14656e2) — Sphaira 1.0.6, pinned revision
