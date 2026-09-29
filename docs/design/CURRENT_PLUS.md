# Current+ UI direction

Current+ is the selected DroidSphere interface direction. It evolves the existing
bottom-dock design instead of replacing it with a generic desktop sidebar.

## Design principles

1. Keep the bottom dock as the primary navigation identity.
2. Keep the active device visible at all times through a compact top status bar.
3. Make connection state, address and refresh actions discoverable without
   opening Device Manager.
4. Keep the Dashboard focused on the current device: health, quick actions and
   recent activity.
5. Keep host-machine configuration in Settings: binary versions, resolved
   executable paths, managed tool directory and diagnostics.
6. Prefer progressive disclosure for advanced controls instead of placing every
   option on the main screen.
7. Preserve dark/light support and build future skins from shared design tokens,
   not duplicated layouts.

## Current+ first pass

- Persistent DroidSphere / active-device top bar.
- Existing bottom dock retained.
- Settings shows full resolved locations for ADB, Fastboot and scrcpy.
- Settings shows the managed tools directory.
- Paths can be copied without expanding individual binary editors.

## Planned visual evolution

- Health Center with foreground app, Wi-Fi quality, richer CPU/RAM/storage and
  network diagnostics.
- Recent Activity card on the Dashboard.
- Compact and comfortable density modes.
- Skin engine on top of the same layout: TVADB Blue, OLED Black, Google TV and
  Graphite.
- Optional glass/glow intensity setting.

## Planned workflow improvements

- Safe, device-specific tuning profiles with backup, dry-run and exact restore.
- Dual-pane PC <-> Android file manager with search, favorites and transfer queue.
- Scrcpy Pro controls for recording, audio, clipboard, virtual display and
  gamepad where supported.
- Fleet/group actions for multiple devices.
- Remote macros and reusable device action sequences.
- Sanitized diagnostics/support bundle generation.
