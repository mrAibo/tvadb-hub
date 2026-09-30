# Architecture improvement review

Date: 2026-09-30

This document evaluates proposed DroidSphere improvements against the current
codebase. The goal is to avoid implementing generic recommendations that are
already solved, unnecessarily risky, or disproportionately expensive.

## Executive recommendation

Do **not** replace the existing ADB CLI integration wholesale with a custom ADB
protocol stack. Keep the official ADB client as the compatibility baseline and
introduce direct ADB-server protocol access only where profiling shows a clear
benefit, such as continuous device tracking.

Backend Logcat event batching is now implemented. The next high-value
engineering work should instead focus on:

1. ADB file-transfer compression exposed through the existing CLI.
2. Optional transfer integrity verification for large files.
3. Richer Logcat crash/ANR filtering and saved filters.
4. Better TV text input using clipboard/scrcpy-first fallbacks.
5. A separately versioned, signed Safe Tuning metadata feed with explicit
   licensing and rollback.

## 1. Native ADB Client Protocol

**Verdict: partial adoption only.**

**Complexity:** High for full replacement; Medium for a focused smart-socket
client.

ADB itself is already a client of the local ADB server smart socket, normally
on port 5037. A small direct client could reduce process-spawn overhead for
high-frequency read-only operations such as device tracking.

A complete replacement is not recommended because DroidSphere currently
benefits from the official ADB client's implementation of:

- server lifecycle and version negotiation;
- USB and TCP transport selection;
- Wireless debugging pairing/connect flows;
- shell protocol details and feature negotiation;
- install/install-multiple behavior;
- sync/send/recv compatibility across Android versions;
- compression negotiation and evolving ADB features.

Recommended future architecture:

- keep official `adb` CLI for mutation-heavy and compatibility-sensitive
  commands;
- consider a small internal smart-socket client for `track-devices`,
  lightweight status queries and possibly long-lived read streams;
- benchmark before migrating an operation.

This is an optimization project, not a prerequisite for correctness.

## 2. Android TV specialization

### PC keyboard / text input

**Verdict: worth implementing.**

**Complexity:** Low to Medium.

DroidSphere already has TV remote controls and scrcpy clipboard support. A TV
text-entry panel can therefore use:

1. scrcpy clipboard/paste where an active scrcpy session exists;
2. Android clipboard APIs where available;
3. `input text` only as a fallback for simple ASCII input.

This is preferable to treating `adb shell input text` as a universal UTF-8
transport, which it is not.

### HDMI-CEC / audio-output shortcuts / System UI restart

**Verdict: selective only.**

**Complexity:** Medium, with high device-specific risk.

Settings intents and vendor services differ significantly between Fire TV,
Google TV and OEM firmware. Generic one-click "restart System UI" or CEC
actions should not be exposed unless the command is verified for that device
family. Experimental/vendor-specific actions can live behind capability checks.

### Custom launcher wizard

**Verdict: useful but not near-term.**

**Complexity:** Medium to High.

Launcher replacement is popular on TV devices but is firmware-specific and can
leave a device without a usable HOME activity. A safe implementation would
need:

- launcher capability detection;
- installation/launch verification;
- current HOME-role/default resolution;
- a tested fallback launcher;
- automatic rollback if HOME cannot be resolved.

This belongs in a guarded TV-specific workflow, not generic Safe Tuning.

## 3. Safe Tuning evolution

### Export to shell / Shizuku-oriented workflows

**Verdict: shell export is worthwhile; direct Shizuku integration is optional.**

**Complexity:** Low for export; Medium to High for an Android-side companion.

DroidSphere can safely export an analyzed tuning plan as:

- a human-readable JSON plan;
- a reversible shell script with comments and restore commands.

That provides portability without requiring a DroidSphere Android app.

### Community package metadata / UAD synchronization

**Verdict: high value, but do not silently vendor UAD data.**

**Complexity:** Medium technically; Medium/High governance and licensing.

UAD-NG is GPL-3.0 and its separate preinstalled-package-list repository is
LGPL-3.0. DroidSphere is MIT. Copying a large upstream database directly into
the MIT application without a clear licensing boundary is not appropriate.

Preferred design:

- a separately versioned metadata repository;
- explicit data license and attribution;
- signed release manifests/digests;
- schema validation;
- local cache and rollback;
- no automatic destructive action after a metadata update;
- profile changes shown to the user before application.

If UAD data is consumed later, keep it as a clearly separated,
license-compliant external dataset or obtain permission for a compatible data
license.

## 4. File Manager improvements

### ADB compression

**Verdict: implement before any custom Sync-v2 client.**

**Complexity:** Low.

Modern ADB already exposes compression through the CLI:

- `adb push -z any`
- `adb pull -z any`

and supports Brotli, LZ4 and Zstd where negotiated. DroidSphere can gain most
of the practical benefit by adding an Auto/None/algorithm option to the
existing transfer engine. There is no need to reimplement Sync v2 merely to
obtain compression.

### Scoped Storage fallback

**Verdict: capability-based, not "unrestricted access".**

**Complexity:** Medium.

`run-as <package>` only works for debuggable applications for which Android
permits run-as. AppOps is not a universal bypass for Android storage security,
and root/adbd-root cannot be assumed.

Recommended behavior:

- detect whether `run-as` works for the selected package;
- expose it only for eligible debuggable apps;
- optionally detect an already-rooted/adbd-root device;
- never claim DroidSphere can bypass Android's storage security model.

### Transfer checksum verification

**Verdict: worthwhile.**

**Complexity:** Low to Medium.

For large firmware archives and backups, add an optional "Verify after
transfer" mode:

- compute SHA-256 locally;
- try `sha256sum` or `toybox sha256sum` remotely;
- report "verification unavailable" rather than silently falling back to a
  weaker guarantee;
- enable by default only above a configurable size if performance becomes a
  concern.

## 5. Diagnostics, scripting and monitoring

### Macros

**Verdict: useful later, not current priority.**

**Complexity:** Medium.

A macro engine becomes a small automation platform and needs cancellation,
timeouts, target pinning, confirmation for destructive steps and versioned
serialization. It should wait until core device-management workflows are more
mature.

### Resource monitor

**Verdict: extend the existing implementation rather than build a new module.**

**Complexity:** Low to Medium.

DroidSphere already monitors CPU, RAM and network data. The useful additions
would be:

- top processes;
- storage pressure;
- optional per-app memory;
- FPS/frame statistics only when `dumpsys gfxinfo` data is meaningful.

The previous decision to avoid a large standalone Health Center remains sound.

## 6. Toolchain and scrcpy

### Isolated managed tools

**Verdict: already implemented.**

DroidSphere already detects custom/system tools and can install managed
Platform Tools and scrcpy into its per-user application data directory.
Tool Locations exposes the resolved source, version and path.

### scrcpy audio forwarding

**Verdict: already implemented.**

Current scrcpy options expose audio enable/disable, codec and bitrate controls.
Upstream scrcpy supports audio forwarding on Android 11+, with Android 12+
working without the Android-11 foreground workaround.

Future scrcpy work should focus on source selection (output/playback/mic),
audio-only sessions and clearer capability hints rather than merely adding an
"audio enabled" switch.

## 7. Security review status

The external security audit was valid in the areas it highlighted, but several
items were already protected before the review and the remainder are hardened
in DroidSphere 0.1.1.

See `docs/SECURITY_AUDIT.md` for the repository-specific findings.

Particularly important:

- no `sh -c` / `cmd.exe /c` wrapper is used for normal ADB operations;
- package names and Wireless endpoints are now strictly validated at the Go
  boundary;
- process-tree cancellation is centralized;
- archive extraction rejects traversal and unsafe entry types;
- managed downloads are pinned and verified;
- Safe Tuning blocks protected/dangerous packages in Go, not only in React;
- Logcat state is bounded and frontend updates are batched;
- CI runs vet, reachable-vulnerability scanning and race detection.

Backend Logcat batching from that audit is now complete: parsed entries are
emitted in bounded batches on a short interval, with a final flush on stream
shutdown. Remaining Logcat work is product-level filtering/highlighting rather
than the previous per-line IPC pressure issue.

## Recommended order

| Priority | Improvement | Complexity | Recommendation |
| --- | --- | --- | --- |
| Done | Backend Logcat event batching | Medium | Completed |
| P1 | ADB push/pull compression options | Low | Do |
| P1 | Transfer SHA-256 verification | Low-Medium | Do |
| P2 | TV text-entry panel | Low-Medium | Do |
| P2 | Signed/versioned Safe Tuning metadata feed | Medium-High | Do carefully |
| P2 | scrcpy audio-source/audio-only UX | Low | Do |
| P3 | Focused ADB smart-socket client | Medium | Prototype + benchmark |
| P3 | Custom launcher wizard | Medium-High | TV-specific, guarded |
| P3 | Macro engine | Medium | Later |
| Avoid | Full replacement of official ADB CLI | High | No, unless profiling proves need |
| Avoid | Generic scoped-storage "bypass" | High/risky | No |

