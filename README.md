# TVADB Hub

**TVADB Hub** is a desktop manager for Android TV and Google TV built around
modern Wireless ADB. Its goal is to remove the repeated manual work around
dynamic ADB pairing/connect ports and turn common TV debugging tasks into
guided GUI workflows.

TVADB Hub is based on [ADBKit](https://github.com/Drenzzz/ADBKit) v2.0.0.
The upstream MIT license and attribution are preserved.

## What works today

### Wireless ADB

- mDNS discovery for `_adb-tls-pairing._tcp`, `_adb-tls-connect._tcp` and
  legacy `_adb._tcp`.
- Automatic resolution of dynamic pairing and connect ports.
- First-time pairing with only the six-digit code shown by Android/Google TV.
- Secure TLS connect preferred over legacy ADB TCP/IP.
- Remembered TVs with background reconnect and dynamic-port recovery.
- Multi-device-safe target selection.
- Wireless diagnostics for Platform Tools, mDNS, TCP reachability, endpoint
  type and current ADB state.
- Reconnect history/status UI.

### TV controls and device information

- TV-aware dashboard and device classification.
- D-pad, Home, Back, Menu, media, volume, mute, power, wake and sleep actions.
- Android version, build, model, manufacturer, network and other device details.
- Read-only physical-TV validation report for release verification.

### Apps

- Install APK.
- Replace/update existing APK.
- Downgrade when Android permits it.
- Install base + split APK sets through `adb install-multiple`.
- Multi-file drag-and-drop installation.
- Launch, force-stop, enable, disable and uninstall packages.
- Android TV launcher compatibility hints using `LEANBACK_LAUNCHER`.

### Debugging and screen tools

- Scrcpy control with TV Balanced, High Quality and Low Bandwidth presets.
- PNG screenshot capture through `adb exec-out screencap -p`.
- File Explorer with push, pull and multi-file transfers.
- Logcat streaming, export and TV-focused diagnostic presets.
- Interactive ADB shell plus safe read-only TV diagnostic shortcuts.

### Managed tools

TVADB Hub can download and manage its own copies of Android SDK Platform Tools
and scrcpy. Validated managed downloads are adopted automatically, while custom
binary paths remain available for advanced setups.

## First connection

On the TV:

1. Enable **Developer options**.
2. Enable **Wireless debugging**.
3. Open **Pair device with pairing code** for the first connection.

In TVADB Hub:

1. Open **Discover / Pair / Connect**.
2. Select the discovered TV.
3. Enter the six-digit pairing code.
4. TVADB Hub resolves the temporary pairing port and the separate dynamic
   connect port automatically.
5. After a successful connection the TV is remembered for later reconnects.

You should not need to manually copy the dynamic connect port during normal
use.

## Physical TV validation

Before a release, connect the target TV through Wireless ADB and run:

**Settings → Physical Google TV validation → Validate connected TV**

The validation is read-only. It checks:

- ready ADB state;
- Android/Google TV classification;
- network ADB transport;
- secure mDNS/TLS discovery;
- Wireless ADB diagnostics;
- model and Android metadata.

Use **Copy JSON report** to capture the result for a release record or bug
report. The detailed manual checklist is in
[docs/PHYSICAL_TV_VALIDATION.md](docs/PHYSICAL_TV_VALIDATION.md).

## Development

The project currently uses:

- Go 1.26
- Wails v3
- Bun
- React / TypeScript
- Vitest
- NSIS for the Windows installer

Install the Wails CLI:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Install dependencies:

```bash
make deps
```

Run development mode:

```bash
make dev
```

Run the normal checks:

```bash
make check
```

Build:

```bash
make build
```

Package the current platform:

```bash
make package
```

## Windows distribution

Pull-request CI verifies the frontend, Go tests and a real Windows package
build. The Windows build produces both a portable executable and an NSIS
installer.

The release workflow can publish:

- `TVADB-Hub-<version>-windows-amd64.exe`
- `TVADB-Hub-<version>-windows-amd64-installer.exe`
- `SHA256SUMS.txt`

Optional Authenticode signing is supported through repository secrets. See
[docs/RELEASING.md](docs/RELEASING.md).

## Architecture

Wireless debugging is intentionally modeled around the TV identity rather than
a remembered TCP port. Android's pairing and connection ports are dynamic and
can change when Wireless debugging restarts.

See [docs/architecture/wireless.md](docs/architecture/wireless.md).

## Upstream

Upstream project: [Drenzzz/ADBKit](https://github.com/Drenzzz/ADBKit)

Imported baseline: ADBKit v2.0.0, commit
`0908cded97caef9b7733f5de6f89f552e3d33109`.

See [UPSTREAM.md](UPSTREAM.md), [LICENSE](LICENSE), and
[docs/ROADMAP.md](docs/ROADMAP.md).
