<p align="center">
  <img src="docs/assets/nsp-carrier-logo.svg" alt="NSP Carrier logo" width="96" />
  <br />
  <strong style="font-size: 32px">NSP Carrier</strong>
</p>

简体中文 · [English](README.md)

**NSP Carrier** 是一款桌面应用，通过 USB 把你选定的 `.nsp`、`.nsz`、`.xci`、`.xcz` 与仅 Sphaira 支持的 `.msp` 文件提供给 Switch 上运行的安装器——Awoo、Goldleaf、Sphaira 或 DBI。将文件加入队列、选择匹配的安装器 profile 并开始服务；真正的安装由 Switch 端的安装器完成。

应用只报告它经 USB 能够观测到的内容：哪些文件已服务、每次传输进行到哪一步。它不是安装器、不是 MTP 服务器，也不能证明某个 title 已安装。

![NSP Carrier 桌面应用](docs/assets/nsp-carrier-desktop.jpg)

具有代表性的队列与传输进度状态：

![NSP Carrier 提供文件服务并显示传输进度](docs/assets/nsp-carrier-transfer-progress.png)

## 功能

- 通过显式选择的安装器 profile 经 USB 提供文件服务：
  - [Awoo](https://github.com/Huntereb/Awoo-Installer)
  - [Goldleaf 0.10+](https://github.com/XorTroll/Goldleaf)
  - [Sphaira 1.0+](https://github.com/NaGaa95/sphaira)
  - DBI
- 文件与递归文件夹选择，支持拖放。
- 队列支持搜索与重复文件名校验。
- 逐文件唯一字节进度、有界活动日志与类型化错误。
- 开始/停止，宿主持有会话生命周期——重新连接会开启全新会话，绝不声称恢复传输。
- 自动/浅色/深色外观。

宿主只报告其能够观测到的内容：`FullyServed`（已完整服务）表示所选 profile 的可观测字节覆盖规则已满足，并不表示 title 已安装。

## 快速开始

`nsp-carrier` 是一款桌面应用。它通过 USB 把你选定的文件提供给 Switch 上运行的安装器；它绝不会写入 Switch，因此安装始终在安装器一侧进行。

你需要：

- 一台在 USB 模式下运行匹配安装器的 Switch —— Awoo Installer 1.6.2、Goldleaf 0.10+、Sphaira 1.0+ 或 DBI；
- 一根连接 PC 与 Switch 的 USB 线；
- 一份应用副本——从 [Releases](https://github.com/timonwong/nsp-carrier/releases) 页面下载（见[安装](#安装)）。

基本流程：

1. 向队列添加 `.nsp`、`.nsz`、`.xci`、`.xcz` 或 `.msp` 文件与文件夹（也支持拖放）。
2. 选择与你的安装器匹配的 profile：Awoo、Goldleaf、Sphaira 或 DBI。
3. 开始服务，然后在 Switch 的安装器中安装。
4. 在应用中查看逐文件进度。`FullyServed`（已完整服务）表示所选 profile 的可观测字节覆盖规则已满足——并不表示 title 已安装。

所选 profile 无法服务的文件会以明确的校验错误阻止 Start，而不会被静默跳过。

平台相关配置——macOS 的 `libusb`、Windows 的 USB 驱动——见[安装](#安装)。

## 安装

从 [Releases](https://github.com/timonwong/nsp-carrier/releases) 页面下载应用：

- **macOS：** 解压 `nsp-carrier-macos-arm64.zip`，将 `NSP Carrier.app` 移入 Applications。
- **Windows：** 解压 `nsp-carrier-windows-amd64.zip`，运行 `nsp-carrier.exe`。

发布产物未做代码签名与公证。macOS 首次打开时，用右键 → 打开（Open）来绕过 Gatekeeper。

### 平台前置条件

#### macOS

macOS 通过 `libusb` 访问 USB。使用 Homebrew 安装：

```sh
brew install libusb
```

#### Windows

Switch 以裸 USB 设备的形式暴露，Windows 需要先安装兼容的 USB 驱动，`nsp-carrier` 才能看到它。使用 [Zadig](https://zadig.akeo.ie/) 安装：

1. 从 <https://zadig.akeo.ie/> 下载并运行 Zadig。
2. 用 USB 线把 Switch 连接到 PC。
3. 让安装器进入 USB 模式——DBI 进入其 USB 模式（DBIbackend）；Awoo 选择“通过 USB 安装”；Goldleaf 打开 Remote PC 浏览器；Sphaira 打开其 USB 安装流程。
4. 在 Zadig 中打开 *Options → List All Devices*，让 Switch 显示出来。
5. 从下拉列表选择 Switch 设备——厂商 ID 为 `057E`（Nintendo），常见显示为 `DBI`、`USB composite device` 或 `057E:3000`；选择匹配的那一项。
6. 目标驱动选择 **libusbK**（若不可用也可选 *WinUSB*）。
7. 点击 *Replace Driver*（或 *Install Driver*），等待安装完成。
8. 启动 `nsp-carrier`，确认应用能看到设备后再开始服务。

## 许可

MIT。

## 致谢

DBI、Awoo、Goldleaf 与 Sphaira 协议实现基于观测到的线上行为开发，以下上游项目作为行为参考：

- [`rashevskyv/dbibackend-qt`](https://github.com/rashevskyv/dbibackend-qt) —— DBI 后端参考
- [`developersu/ns-usbloader`](https://github.com/developersu/ns-usbloader) —— Awoo 与 Goldleaf 线上参考
- [`Huntereb/Awoo-Installer`](https://github.com/Huntereb/Awoo-Installer) —— Awoo 参考
- [`XorTroll/Goldleaf`](https://github.com/XorTroll/Goldleaf) —— Goldleaf 参考
- [`NaGaa95/sphaira`](https://github.com/NaGaa95/sphaira/tree/3f8303db00f33bfffa83ce0a1b750a1de14656e2) —— 固定的 Sphaira 1.0.6 行为参考

Sphaira 1.0+ 当前仅为 **Compatible**：依据是固定 revision 的行为证据与自动化测试。只有真实设备矩阵全部通过后，某个精确 Sphaira 版本才能标记为 **Verified**。Sphaira 0.13.3 及更早版本使用不兼容的旧 TUL0/TUC0 协议代际。
