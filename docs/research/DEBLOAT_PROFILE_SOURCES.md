# Safe Tuning profile sources

TVADB Hub Safe Tuning is intentionally built around **reversible user-scoped
changes**, explicit risk levels and device/brand matching. A package appearing
in a source list is not treated as proof that it is safe on every firmware.

## Sources used for built-in package metadata

### 26zl/tv-tweak — MIT

Source: https://github.com/26zl/tv-tweak

Used for the device-specific profiles:

- Amazon Fire TV Stick 4K Max (`karat` / `AFTKRT`), verified upstream on
  Fire OS 8.1.8.0.
- Sharp 4K Google TV / MediaTek MT9676 (`maniatika`), verified upstream on
  Android 14.

The project was particularly useful for its safety model: profile-specific
`keep.conf` lists, `pm disable-user` as the primary action, serial-pinned
backups, exact restore of only changes made by the tool, and explicit warnings
that generic Android TV package lists can break DRM or input handling.

### PixelCode01/UIBloatwareRegistry — MIT

Source: https://github.com/PixelCode01/UIBloatwareRegistry

Used for conservative brand profiles and risk annotations for:

- Samsung / One UI
- Xiaomi / Redmi / POCO
- Google Pixel / AOSP-like devices
- OnePlus / OxygenOS

Only packages with a clear purpose/risk classification were imported into the
initial TVADB Hub profiles. Dangerous entries are intentionally informational
and cannot be selected in Safe Tuning.

### farag2/ADB-Debloating — MIT

Source: https://github.com/farag2/ADB-Debloating

Used as a secondary cross-check for package names and labels, especially common
Samsung and Xiaomi optional packages. It is not the sole basis for a
TVADB Hub "safe" classification.

### seun-novodev/android-tv-debloat-toolkit — MIT

Source: https://github.com/seun-novodev/android-tv-debloat-toolkit

Used for the initial TCL Android/Google TV profile. TVADB Hub deliberately
changes the default behaviour from user-0 uninstall to the more reversible
`pm disable-user --user 0`; user-0 uninstall remains an explicit advanced
mode.

## Sources studied but not copied into built-in data

### Universal Android Debloater Next Generation / UAD lists

- https://github.com/Universal-Debloater-Alliance/universal-android-debloater-next-generation
- https://github.com/Universal-Debloater-Alliance/universal-android-preinstalled-lists

UAD was reviewed for architecture and UX ideas: risk tiers, package-state
snapshots, restore, multi-user awareness and explicit warnings. Its GPL/LGPL
licensed code/data is **not vendored into TVADB Hub's MIT source tree**.

Other repositories without a clearly compatible license are likewise treated
as research references only.

## TVADB Hub safety rules

1. The default operation is `pm disable-user --user 0`.
2. A snapshot is written **before** any change.
3. Exact-device TV profiles maintain protected keep-lists for known launcher,
   DRM, input, remote and playback components.
4. `safe` entries may be preselected; `caution` entries are never
   preselected and require explicit acknowledgement.
5. `dangerous` and `blocked` entries are informational only and cannot be
   changed through Safe Tuning.
6. Advanced `uninstall-user` uses
   `pm uninstall -k --user 0` only for preinstalled/system packages, so
   `cmd package install-existing --user 0` can be used during restore.
7. Profiles are filtered against the packages actually installed on the
   selected Android device.
8. No tuning/debloat action runs automatically on connect or profile match.

## Extending profiles

Prefer adding a small, evidence-backed profile over a large universal list.
For device-specific TV profiles, explicitly protect at least:

- launcher/home;
- System UI and Settings;
- package installer and permission controller;
- DRM/Widevine/vendor playback services;
- TV input / HDMI / tuner stack;
- remote/input-method services;
- WebView;
- the streaming apps that were used for validation.

Every new imported data source must have a compatible license recorded in this
document and in `THIRD_PARTY_NOTICES.md`.
