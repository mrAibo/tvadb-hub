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
- [x] APK install/update/downgrade and split APK installation
- [x] App launch, force-stop, enable/disable, uninstall and APK extraction
- [x] Safe Tuning profiles with risk levels, snapshots and exact restore
- [x] Dual-pane host ↔ Android file manager
- [x] scrcpy presets, recording controls, screenshot capture and clipboard support
- [x] Interactive shell and live Logcat
- [x] Fastboot/flash workflows
- [x] Managed ADB/Fastboot/scrcpy tools with visible resolved paths
- [x] Portable JSON settings backup
- [x] Windows installer + native Windows/Linux/macOS CI builds

## Next: files and backups

- [ ] File preview for images and text
- [ ] Folder sync with preview, include/exclude filters and conflict policy
- [ ] Transfer queue with retry/resume history
- [ ] App backup/restore for base + split APK sets
- [ ] Optional user-data backup where Android permissions/API level allow it

## Next: diagnostics and developer workflow

- [ ] Connection Doctor covering USB authorization, Wireless ADB, mDNS, ports,
      ADB/Fastboot/scrcpy versions and actionable fixes
- [ ] Logcat 2.0: app/PID filter, crash/ANR highlighting, saved filters and pinned events
- [ ] Permission/AppOps inspector with a read-only default mode
- [ ] Device report export for support/debugging
- [ ] Device-side screen recording workflow independent of scrcpy

## Later

- [ ] Quick Share integration for ordinary file exchange without ADB
- [ ] Optional device macros/automation
- [ ] Package/profile community catalog with signed metadata
- [ ] Linux package publishing (AppImage/DEB/RPM)
- [ ] Signed and notarized macOS releases
- [ ] New DroidSphere application icon and full visual-brand pass

## Safety principles

- Prefer reversible operations over destructive ones.
- Snapshot before tuning changes.
- Keep dangerous packages non-actionable by default.
- Require the Android authorization model rather than bypassing it.
- Keep target-device identity visible before destructive actions.
