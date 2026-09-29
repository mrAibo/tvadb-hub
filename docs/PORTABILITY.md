# Cross-platform portability

DroidSphere is architecturally portable to Windows, macOS and Linux because the
desktop shell is Wails v3, the backend is Go, and the UI is React/TypeScript.
The project does **not** need a rewrite for macOS or Linux.

## Current status

| Area | Windows | Linux | macOS |
| --- | --- | --- | --- |
| Wails desktop runtime | supported | supported | supported |
| ADB / Fastboot managed download | supported | supported | supported |
| scrcpy managed download | supported | amd64/arm64 | amd64/arm64 |
| binary auto-detection | supported | supported | supported |
| native file/folder dialogs | supported | supported | supported |
| open tool location in file manager | Explorer | xdg-open | Finder |
| JSON settings backup | supported | supported | supported |
| compile CI gate | yes | yes | yes |
| packaged release pipeline | EXE + NSIS | not yet release-published | not yet release-published |
| release signing | optional Authenticode | not configured | not configured/notarized |

## What is already cross-platform in the code

### Managed Android tools

`internal/download/service.go` selects official Google Platform Tools
downloads by `runtime.GOOS` for Windows, Linux and macOS. scrcpy downloads are
also selected by OS and by `amd64` / `arm64`.

This means a macOS/Linux user does not need Android Studio merely to obtain
ADB. DroidSphere can keep the same managed-tool UX used on Windows.

### Paths and host integration

Go's `filepath` package and Wails native dialogs are already used throughout
the project. Tool Locations opens the host file manager with:

- Windows: `explorer.exe`
- macOS: `open`
- Linux: `xdg-open`

Settings backup is platform-neutral JSON. Imported backups can contain
machine-specific binary paths; those paths are intentionally visible and can
be re-detected or corrected on a different machine.

### File transfers

Device transfers use the ADB CLI and are not tied to Windows. The dual-pane
file-manager work adds host filesystem browsing through Go, so path parsing is
also kept out of browser-only JavaScript and follows the host OS rules.

## Build requirements

### macOS

Wails v3 requires the Xcode Command Line Tools. macOS supplies the native
WebKit runtime, so there is no WebView runtime bundle to ship.

Development build:

```bash
xcode-select --install
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
make frontend-install
make macos
```

Official Wails documentation:
https://v3.wails.io/quick-start/installation/

### Linux

Current Wails v3 desktop builds use GTK4 + WebKitGTK 6.0 by default. On
Ubuntu/Debian systems with the current stack:

```bash
sudo apt update
sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
make frontend-install
make linux
```

Older supported distributions that only provide WebKit2GTK 4.1 need Wails'
legacy GTK3 build tag. Wails documents that path as transitional.

Official Wails documentation:
https://v3.wails.io/guides/build/linux/

## CI strategy

PR CI now has three desktop compilation gates:

1. Windows package build (portable EXE + NSIS installer).
2. Linux native desktop build on Ubuntu.
3. macOS native desktop build on a GitHub macOS runner.

A feature is therefore much less likely to introduce a Windows-only compile
dependency unnoticed.

## Remaining work for distributable Linux builds

The repository already contains Wails Linux tasks for AppImage, DEB, RPM and
Arch packages. Before publishing those artifacts, validate:

1. package metadata and desktop integration;
2. runtime WebKitGTK expectations for each target distribution;
3. AppImage behaviour on at least Ubuntu and one non-Ubuntu distribution;
4. optional package signing;
5. managed ADB/scrcpy download + execution on amd64 and arm64.

The old lightweight AppImage helper still referenced the historical ADBKit
name/version. This portability package corrects it to DroidSphere and reads the
project version from the Makefile.

## Remaining work for distributable macOS builds

The repository already has Wails tasks for native Intel/Apple Silicon builds,
universal binaries, app bundles, Developer ID signing and notarization.

The current repository should treat **compilation** and **distribution** as
separate milestones:

1. keep the macOS compile gate green;
2. generate/validate the macOS bundle assets (`Info.plist`, icon assets);
3. build an actual `.app` bundle on macOS;
4. test on Apple Silicon and, if desired, Intel;
5. configure Apple Developer ID signing;
6. configure notarization credentials;
7. publish a DMG/ZIP only after Gatekeeper testing.

Unsigned local builds are useful for development, but a public macOS release
should be signed and notarized to avoid normal Gatekeeper friction.

Official Wails macOS packaging documentation:
https://v3.wails.io/guides/build/macos/

## Difficulty assessment

### Linux: low to moderate

The application code is already suitable for Linux. Most remaining work is
distribution engineering and runtime validation across distributions rather
than feature rewrites.

### macOS: moderate

The application code is also already suitable for macOS. The extra effort is
primarily Apple distribution infrastructure: app-bundle validation, signing,
notarization and testing on Apple Silicon/Intel.

### Feature-development rule

New host-specific functionality should follow the same pattern used by Tool
Locations and the file manager:

- keep OS branching in Go;
- use `runtime.GOOS` / `filepath`;
- use Wails native dialogs;
- avoid shell syntax that only works on one host;
- add a platform-neutral unit test where possible;
- let the three-platform CI matrix catch compile regressions.
