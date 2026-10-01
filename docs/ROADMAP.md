# DroidSphere roadmap

DroidSphere started as a TV-focused Wireless ADB tool and has grown into a
cross-platform Android device manager for phones, tablets, TVs and streaming
devices.

## Shipped foundation

- [x] USB and Wireless ADB device discovery
- [x] Android 11+ pairing-code workflow and mDNS discovery
- [x] Remembered devices and reconnect from the Current+ device selector
- [x] Device dashboard, device details and performance snapshots
- [x] Android TV remote controls where the selected device is a TV
- [x] TV text-entry panel with Unicode clipboard paste and safe ASCII fallback
- [x] APK install/update/downgrade and split APK installation
- [x] App launch, force-stop, enable/disable, uninstall and APK extraction
- [x] Safe Tuning profiles with risk levels, snapshots and exact restore
- [x] Dual-pane host ↔ Android file manager
- [x] scrcpy presets, recording controls, screenshot capture and clipboard support
- [x] scrcpy audio source selection and audio-only sessions with Android capability hints
- [x] Interactive shell and live Logcat
- [x] Backend Logcat IPC batching with bounded interval/size flushes
- [x] Capability-aware ADB push/pull compression with Auto/Off/algorithm preferences
- [x] Optional SHA-256 transfer verification with explicit unavailable/mismatch reporting
- [x] Fastboot/flash workflows
- [x] Managed ADB/Fastboot/scrcpy tools with visible resolved paths
- [x] Portable JSON settings backup
- [x] Connection Doctor for ADB/toolchain/authorization/transport diagnostics
- [x] Windows installer + native Windows/Linux/macOS CI builds
- [x] Release-candidate packages committed under `distribution/` for Windows, Linux and macOS

## Next: files and backups

For the accepted implementation sequence and actual completion state, see
[`STATUS.md`](../STATUS.md) and
[live execution issue #43](https://github.com/mrAibo/tvadb-hub/issues/43).
Audit remediation precedes the next P3 product feature: the guarded,
capability-aware TV custom launcher wizard. Macros remain later. The sections
below are the wider product backlog, not a competing immediate priority list.

- [ ] File preview for images and text
- [ ] Folder sync with preview, include/exclude filters and conflict policy
- [ ] Transfer queue with retry/resume history
- [ ] App backup/restore for base + split APK sets
- [ ] Optional user-data backup where Android permissions/API level allow it

## Next: diagnostics and developer workflow

- [x] Read-only ADB smart-socket `host:devices-l` prototype + benchmark harness; production discovery remains CLI-backed
- [x] Logcat 2.0 phase 1: crash/ANR filtering, highlighting and saved filters
- [x] Logcat 2.0 phase 2: app/PID filtering and pinned events
- [ ] Permission/AppOps inspector with a read-only default mode
- [ ] Device report export for support/debugging
- [ ] Device-side screen recording workflow independent of scrcpy

## Later

- [ ] Quick Share integration for ordinary file exchange without ADB
- [ ] Optional device macros/automation
- [x] Signed/versioned Safe Tuning metadata feed infrastructure with rollback
- [ ] Curated external community package/profile dataset with an explicit licensing boundary
- [ ] Linux package publishing (AppImage/DEB/RPM)
- [ ] Signed and notarized macOS releases
- [ ] New DroidSphere application icon and full visual-brand pass

## Safety principles

- Prefer reversible operations over destructive ones.
- Snapshot before tuning changes.
- Keep dangerous packages non-actionable by default.
- Require the Android authorization model rather than bypassing it.
- Keep target-device identity visible before destructive actions.
