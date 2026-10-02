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

Backend Logcat event batching, ADB CLI transfer compression, optional
post-transfer SHA-256 verification, safe TV text entry, scrcpy audio-source
controls, Logcat crash/ANR diagnostics with saved filters, Logcat app/PID
filtering with session-local pinned events, the guarded TV custom launcher wizard
and the opt-in signed Safe Tuning metadata feed infrastructure are now
implemented. The architecture-only P3 items should still be benchmarked or
guarded before adoption.

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

A read-only `host:devices-l` smart-socket prototype and benchmark harness now
exist under `internal/adbproto` and `cmd/droidsphere-adb-bench`. They are
intentionally not wired into production discovery yet; the benchmark must show
a repeatable benefit across supported hosts/transports before that boundary is
changed.

This is an optimization project, not a prerequisite for correctness.

## 2. Android TV specialization

### PC keyboard / text input

**Status: implemented.**

**Complexity:** Low to Medium.

The Device Manager TV Remote now includes a text-entry field for the focused
Android TV/streaming-device input. The backend uses this order:

1. the existing scrcpy clipboard bridge plus Android PASTE when the selected
   device has an active scrcpy session;
2. Android clipboard set + PASTE when available;
3. `input text` only as a printable-ASCII fallback.

Unicode is never silently routed through `input text`. Percent-containing or
non-printable/non-ASCII text therefore fails clearly if clipboard paste is not
available. Because the ADB client joins `adb shell` arguments without escaping,
all user-controlled clipboard/text values are explicitly POSIX-shell quoted
before they enter the remote command string.

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

**Status: signed/versioned feed infrastructure implemented; external catalog remains opt-in.**

**Complexity:** Medium technically; Medium/High governance and licensing.

UAD-NG is GPL-3.0 and its separate preinstalled-package-list repository is
LGPL-3.0. DroidSphere is MIT. Copying a large upstream database directly into
the MIT application without a clear licensing boundary is not appropriate.

Implemented feed boundary:

- separately hosted/versioned metadata supported over HTTPS;
- explicit feed-level and profile-level source/license attribution;
- Ed25519-signed envelopes with a pinned public key and visible SHA-256 payload digest;
- strict schema/profile validation and a non-overridable core-package safety floor;
- trust-config-namespaced current/previous verified caches with explicit rollback;
- monotonic revision checks that reject silent downgrade and revision collisions;
- no automatic metadata download and no destructive action after an update;
- profile/package changes remain visible in the normal analysis/review flow before application.

DroidSphere intentionally ships no default external catalog or private signing
key. The maintainer signing tool and feed format are documented in
`docs/SAFE_TUNING_FEED.md`.

If UAD data is consumed later, keep it as a clearly separated,
license-compliant external dataset or obtain permission for a compatible data
license.

## 4. File Manager improvements

### ADB compression

**Status: implemented.**

**Complexity:** Low.

DroidSphere now uses the official ADB CLI compression flags for both push and
pull. Auto uses `-z any` when the installed ADB advertises compression support;
Off uses `-Z` where available; explicit Zstd, LZ4 and Brotli preferences are
used only when advertised. Older ADB versions fall back to ordinary transfers
without unsupported flags. No custom Sync-v2 implementation is required.

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

**Status: implemented.**

**Complexity:** Low to Medium.

DroidSphere now provides an optional **Verify after transfer** setting. For
regular files it computes SHA-256 on the host, then tries `sha256sum` and
`toybox sha256sum` on Android and compares the complete digests.

- matching digests are reported as verified;
- a digest mismatch fails the transfer result instead of being presented as
  success;
- if neither Android `sha256sum` nor `toybox sha256sum` is available, the
  result is reported explicitly as verification unavailable;
- malformed hash output, permission failures and other hashing execution
  failures are rejected as verification failures rather than being mislabeled
  as unavailable;
- directories and non-regular host paths are reported as verification
  unavailable rather than being followed or treated as verified;
- hashing shares the transfer cancellation context.

The primary dual-pane transfer buttons also use the shared transfer pipeline
again, restoring the existing progress/cancellation overlay for those actions.

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

**Status: audio-source and audio-only UX implemented.**

DroidSphere keeps the existing codec/bitrate controls and now also exposes
`output`, `playback` and `mic` sources plus an audio-only mode implemented
with upstream scrcpy's `--no-video --no-control` combination.

Compatibility is handled explicitly:

- audio forwarding is presented as Android 11+;
- Android 11 users are reminded that the device must be unlocked when capture
  starts;
- the `playback` source is gated to Android 13+ when the SDK is known;
- the Go backend validates source names and incompatible `audio_only +
  no_audio` requests rather than trusting the WebView;
- legacy settings and TVADB-compatible backups normalize to `output`.

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
shutdown. Product-level crash/ANR classification, highlighting and locally
saved filters are also implemented without changing the stream lifecycle.
Logcat 2.0 phase 2 is also implemented: backend entries are enriched with best-effort
process names from bounded ADB `ps` snapshots, the frontend can filter by exact PID
or process-name substring, and users can keep up to 100 session-local pinned event
snapshots without persisting log contents.

## Recommended order

| Priority | Improvement | Complexity | Recommendation |
| --- | --- | --- | --- |
| Done | Backend Logcat event batching | Medium | Completed |
| Done | ADB push/pull compression options | Low | Completed |
| Done | Transfer SHA-256 verification | Low-Medium | Completed |
| Done | TV text-entry panel | Low-Medium | Completed |
| Done | Logcat crash/ANR filtering + saved filters | Low-Medium | Completed |
| Done | Signed/versioned Safe Tuning metadata feed infrastructure | Medium-High | Completed, opt-in |
| Done | scrcpy audio-source/audio-only UX | Low | Completed |
| P3 | Focused ADB smart-socket client | Medium | Prototype added; benchmark before adoption |
| Done | Custom launcher wizard | Medium-High | Completed, TV-specific and guarded (PR #56) |
| P3 | Macro engine | Medium | Later |
| Avoid | Full replacement of official ADB CLI | High | No, unless profiling proves need |
| Avoid | Generic scoped-storage "bypass" | High/risky | No |

