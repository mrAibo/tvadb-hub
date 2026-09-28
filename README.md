# TVADB Hub

**TVADB Hub** is a desktop manager for Android TV and Google TV focused on making
Wireless ADB discovery, pairing, reconnection, APK management, diagnostics and
TV-oriented controls as automatic as possible.

The project is based on [ADBKit](https://github.com/Drenzzz/ADBKit) v2.0.0 and
keeps its MIT license and upstream attribution. The TV-specific work is being
implemented as separate services and UI flows so useful upstream ADBKit changes
can continue to be integrated.

## Core goal

A normal reconnect should not require users to know or copy dynamic ADB ports.

Modern Wireless Debugging advertises services through mDNS:

- `_adb-tls-pairing._tcp` for the temporary pairing endpoint.
- `_adb-tls-connect._tcp` for the authenticated connect endpoint.
- `_adb._tcp` for legacy TCP/IP ADB.

TVADB Hub discovers these automatically and treats the dynamic port as an
endpoint, not as the identity of the TV.

## Current bootstrap status

- ADBKit v2.0.0 source baseline imported.
- Upstream MIT license preserved.
- ADB mDNS parser implemented.
- Automatic pairing/connect endpoint resolution implemented.
- TLS connect preferred over legacy ADB on the same host.
- Ambiguous multi-device discovery is rejected instead of connecting randomly.
- Parser/resolver tests include the real Google TV discovery output used during
  initial development.
- Pull-request CI validates frontend checks and Go tests.

## Planned TV workflow

```text
launch
  -> discover Android/Google TV devices
  -> identify remembered TV
  -> resolve current _adb-tls-connect._tcp endpoint
  -> connect
  -> verify state
  -> READY
```

First-time setup:

```text
discover pairing service
  -> user enters only the six-digit code
  -> pair
  -> discover connect service
  -> connect
  -> remember logical TV
```

## Next milestones

1. Stable logical TV model and remembered-device selection.
2. Connection state machine and automatic reconnect/recovery.
3. TV dashboard and pairing wizard.
4. Connection diagnostics and Platform Tools version checks.
5. APK install/update/downgrade and split-APK workflow.
6. TV remote controls, screenshots, scrcpy, files, shell and logcat presets.
7. Windows portable build and installer branded as TVADB Hub.

See [docs/architecture/wireless.md](docs/architecture/wireless.md) for the
wireless discovery design.

## Upstream

Upstream project: [Drenzzz/ADBKit](https://github.com/Drenzzz/ADBKit)

Imported baseline: ADBKit v2.0.0, commit
`0908cded97caef9b7733f5de6f89f552e3d33109`.

See [UPSTREAM.md](UPSTREAM.md) and [LICENSE](LICENSE).
