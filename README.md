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
- ADB mDNS discovery and automatic dynamic pairing/connect endpoint resolution.
- One-code pairing flow: enter the six-digit TV code; TVADB Hub resolves both ports.
- TLS connect preferred over legacy ADB on the same host.
- Ambiguous multi-device discovery is rejected instead of connecting randomly.
- Successfully connected TVs are remembered and reconnected automatically.
- Background recovery follows changed dynamic ports and can survive an IP change
  when the remembered mDNS identity is still advertised.
- Built-in Wireless ADB diagnostics check Platform Tools, mDNS, TCP reachability,
  endpoint type, target selection, and current ADB state.
- Parser/resolver tests include the real Google TV discovery output used during
  initial development.
- Pull-request CI validates frontend checks and Go tests; a Windows portable
  build is generated as a workflow artifact.

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

1. Enrich remembered TVs with Android properties after connection for a stronger
   identity than host/mDNS alone.
2. TV dashboard and remote-control surface.
3. APK install/update/downgrade and split-APK workflow.
4. TV-specific screenshots, scrcpy, files, shell and logcat presets.
5. Production Windows installer and release automation.

See [docs/architecture/wireless.md](docs/architecture/wireless.md) for the
wireless discovery design.

## Upstream

Upstream project: [Drenzzz/ADBKit](https://github.com/Drenzzz/ADBKit)

Imported baseline: ADBKit v2.0.0, commit
`0908cded97caef9b7733f5de6f89f552e3d33109`.

See [UPSTREAM.md](UPSTREAM.md) and [LICENSE](LICENSE).
