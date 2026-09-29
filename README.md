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
- Remembered TVs/devices with background reconnect and dynamic-port recovery.
- Current+ device dropdown for switching connected devices and reconnecting remembered devices.
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

### Safe Tuning

- Reversible debloat/tuning profiles for Android TV, phones and tablets.
- Device-specific Fire TV, Sharp Google TV and TCL profiles.
- Brand profiles for Samsung, Xiaomi/Redmi/POCO, Pixel and OnePlus, plus a
  conservative generic Android fallback.
- Safe/caution/dangerous/blocked risk classification.
- `pm disable-user --user 0` as the default action.
- Per-device snapshots and exact restore of changes made by TVADB Hub.
- Advanced reversible user-0 uninstall for eligible preinstalled packages.
- Profile-specific keep-lists protect known launcher, DRM, input, remote and
  playback packages on validated TV profiles.

### Debugging and screen tools

- Scrcpy control with TV Balanced, High Quality and Low Bandwidth presets.
- PNG screenshot capture through `adb exec-out screencap -p`.
- Dual-pane File Manager with local PC/macOS/Linux browsing on the left and Android browsing on the right.
- Direct PC ↔ Android file and folder transfers with progress, retries and cancellation.
- Logcat streaming, export and TV-focused diagnostic presets.
- Interactive ADB shell plus safe read-only TV diagnostic shortcuts.

### Managed tools

TVADB Hub can download and manage its own copies of Android SDK Platform Tools
and scrcpy. Validated managed downloads are adopted automatically, while custom
binary paths remain available for advanced setups.

Settings → Tool locations shows the resolved version, source and exact path for
ADB, Fastboot, scrcpy and managed tools, with copy/open-location actions.

Settings can export/import a portable JSON backup containing preferences,
binary paths, device nicknames, Scrcpy presets, remembered wireless devices and
window state. Machine-specific paths can be re-detected after import on another
computer.

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

Build the current platform:

```bash
make build
```

Explicit desktop targets:

```bash
make windows
make linux
make macos
```

Package the current platform:

```bash
make package
```

## Cross-platform status

PR CI compiles TVADB Hub on Windows, Linux and macOS. The application code,
managed Platform Tools downloads and managed scrcpy downloads are already
OS-aware; Linux/macOS do not require an application rewrite.

Windows remains the current published release target. Linux package publishing
and macOS signed/notarized distribution are separate release-engineering
milestones.

See [docs/PORTABILITY.md](docs/PORTABILITY.md) for build requirements and the
remaining distribution work.

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
