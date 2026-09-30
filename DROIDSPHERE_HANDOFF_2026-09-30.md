# DroidSphere handoff — 2026-09-30

> Historical PR #29 checkpoint. Superseded for continuation by
> [DROIDSPHERE_HANDOFF_2026-09-30_P3.md](DROIDSPHERE_HANDOFF_2026-09-30_P3.md)
> and [live execution issue #43](https://github.com/mrAibo/tvadb-hub/issues/43).
> The P1/P2 backlog below is historical, not a request to redo merged work.

This is the canonical handoff for continuing work on **mrAibo/tvadb-hub**
(product name: **DroidSphere**) in a fresh ChatGPT session.

Do not ask the user to re-explain the project. Read this file and the referenced
repository documents, synchronize `main`, inspect any commits newer than the
checkpoint below, and continue from the actual repository state.

---

## 1. Repository and checkpoint

Repository:

```text
mrAibo/tvadb-hub
```

Product name:

```text
DroidSphere
```

Current canonical branch:

```text
main
```

Checkpoint at handoff creation:

```text
854b65075d07e3c2bb73388419db509466d3c91b
```

Checkpoint commit message:

```text
Merge pull request #29 from mrAibo/fix/base-ui-menu-group-error

Hotfix Base UI crash and harden DroidSphere 0.1.1
```

Current version:

```text
0.1.1
```

The historical GitHub repository slug remains `tvadb-hub`. The application,
binary names, package names and current user-facing branding are **DroidSphere**.

### New-session synchronization

Run before modifying anything:

```bash
git fetch origin
git checkout main
git pull --ff-only origin main
git rev-parse HEAD
```

Expected SHA at handoff creation:

```text
854b65075d07e3c2bb73388419db509466d3c91b
```

If `main` has advanced beyond that SHA, inspect every intervening commit before
making changes and preserve valid newer work.

Do **not** continue development from the old branch
`fix/base-ui-menu-group-error`: at handoff creation it is already merged,
contains no unique diff versus `main`, and is one commit behind `main`.

---

## 2. Files to read first in the new session

Read these completely before changing code:

1. `DROIDSPHERE_HANDOFF_2026-09-30.md`
2. `README.md`
3. `docs/ROADMAP.md`
4. `docs/SECURITY_AUDIT.md`
5. `docs/ARCHITECTURE_REVIEW.md`
6. `docs/PORTABILITY.md`
7. `docs/research/DEBLOAT_PROFILE_SOURCES.md`
8. `docs/design/CURRENT_PLUS.md`

The two most important review documents are:

- `docs/SECURITY_AUDIT.md` — repository-specific verification and hardening
  after the external security/code review.
- `docs/ARCHITECTURE_REVIEW.md` — evaluation of proposed future improvements,
  including what should and should not be implemented.

---

## 3. Major work already completed

### A. Product evolution and rebrand

The project began as a TV-focused Wireless ADB application and is now a
cross-platform Android device manager for:

- phones;
- tablets;
- Android TV / Google TV;
- Fire TV / streaming devices;
- Fastboot targets.

The user-facing product was rebranded from **TVADB Hub** to **DroidSphere**.

Completed branding work includes:

- application/window title;
- About page;
- Setup Wizard;
- build metadata;
- Windows EXE and NSIS installer names;
- Linux package metadata;
- macOS bundle identity;
- release/CI artifact names;
- README and documentation;
- settings backup filename.

Compatibility intentionally preserved:

- internal Go module still uses historical `ADBKit` imports;
- the existing legacy app-data location is retained so existing users do not
  lose settings;
- legacy `tvadb-hub-settings` JSON backups remain import-compatible;
- updater still points at repository `mrAibo/tvadb-hub` until the GitHub
  repository itself is explicitly renamed.

### B. Current+ UI

The selected design direction is **Current+**.

Do not replace it with a generic permanent sidebar.

Current+ principles:

- existing bottom dock remains primary navigation;
- compact persistent top device/status bar;
- active device always visible;
- connected and remembered devices available from a dropdown;
- remembered Wireless ADB devices can be reconnected from that dropdown;
- host configuration belongs in Settings;
- future skins change visual tokens, not application layout.

### C. Tool Locations and settings backup

Settings now exposes the actual resolved:

- ADB version/source/path;
- Fastboot version/source/path;
- scrcpy version/source/path;
- managed tools directory.

Actions include copy path and open host folder.

Settings backup supports JSON export/import for:

- preferences;
- binary paths;
- device nicknames;
- scrcpy presets;
- remembered Wireless ADB devices;
- window state.

New default file name:

```text
droidsphere-settings.json
```

Old TVADB backup format remains importable.

### D. Safe Tuning / Debloat

Safe Tuning supports Android TV and general Android devices.

Built-in profile families include:

- Amazon Fire TV;
- Sharp Google TV;
- TCL Android/Google TV;
- Samsung / One UI;
- Xiaomi / Redmi / POCO;
- Google Pixel / AOSP-like;
- OnePlus / OxygenOS;
- conservative generic Android fallback.

Safety behavior:

- default operation: `pm disable-user --user 0`;
- snapshot before changes;
- exact restore of DroidSphere-made changes;
- advanced user-0 uninstall only for eligible preinstalled/system packages;
- `safe` may be preselected;
- `caution` requires explicit acknowledgement;
- `dangerous` and `blocked` are non-actionable;
- TV profiles protect launcher, DRM, input, remote and playback components.

The Go backend enforces these restrictions; they are not frontend-only.

Source/license research is documented in
`docs/research/DEBLOAT_PROFILE_SOURCES.md`.

### E. Dual-pane File Manager

Files workspace is now a true dual-pane manager:

- left: Windows/macOS/Linux host filesystem;
- right: Android filesystem;
- PC → Android push;
- Android → PC pull;
- files and whole directories;
- multiple selection;
- remote rename/move/delete/new folder;
- transfer progress, cancellation and retries;
- protected/scoped-storage diagnostics;
- native host path semantics implemented in Go.

### F. Connection Doctor

Connection Doctor is implemented in Settings.

It diagnoses:

- DroidSphere setup state;
- ADB binary availability/version/source/path;
- multiple valid ADB installations;
- Fastboot and scrcpy readiness;
- ADB daemon startup;
- device inventory;
- ambiguous multi-device target selection;
- active target;
- ADB authorization;
- `unauthorized` / `offline` states;
- USB vs Wireless ADB;
- current Wireless ADB TCP endpoint;
- Wireless mDNS and related diagnostics;
- Android device metadata.

It returns Pass / Warning / Fail / Info checks with concrete remediation text.

An explicit **Restart ADB server** action exists for host-side ADB recovery. It
does not change Android device settings.

### G. Physical-device validation

The old TV-only release validation was generalized to **Physical Android device
validation**.

Phones, tablets, TVs and streaming devices can all satisfy the release gate.
TV classification is additional information rather than a mandatory condition.

### H. Cross-platform support

The codebase is now compile-validated on:

- Windows;
- Linux;
- macOS.

No application rewrite is required for macOS/Linux.

PR CI includes native desktop builds for all three hosts.

macOS public distribution is not Apple-notarized yet.

### I. README

README was substantially rewritten.

It includes:

- product scope;
- device classes;
- features;
- safety model;
- How to install;
- Windows/macOS/Linux installation instructions;
- four Current+ UI previews;
- platform status;
- development/build instructions;
- next-feature shortlist;
- project history and attribution.

---

## 4. Base UI production crash — FIXED

The user reported this runtime failure:

```text
Unexpected Application Error!
Base UI error #31
```

Root cause:

`DropdownMenuLabel` maps to Base UI `Menu.GroupLabel`, and Current+
rendered it directly inside `DropdownMenuContent`.

Base UI requires `Menu.GroupLabel` to be inside `Menu.Group` or
`Menu.RadioGroup`.

Fix in PR #29:

- both Current+ dropdown label sections are wrapped in `DropdownMenuGroup`;
- a frontend regression test actually opens the device dropdown;
- a router-level application-owned `errorElement` was added so a future route
  exception no longer shows React Router's raw developer stack page.

Do **not** re-implement this fix; it is already merged into `main`.

---

## 5. External security/code audit — current status

The user supplied a structured audit covering:

1. command injection/subprocess handling;
2. process lifecycle/zombie leaks;
3. path traversal/symlink safety;
4. managed toolchain integrity;
5. Logcat memory/IPC pressure;
6. Safe Tuning reversibility;
7. Wails/WebView IPC safety;
8. static analysis/race detector/fuzzing recommendations.

The repository was reviewed against the actual implementation rather than
blindly applying generic recommendations.

### 5.1 Command injection — hardened

Verified:

- normal ADB/Fastboot/scrcpy operations do not use `sh -c`,
  `cmd.exe /c` or equivalent shell wrappers;
- executable + args are passed separately through Go process APIs.

0.1.1 adds:

- strict Android package-name validation: `[A-Za-z0-9._]+`;
- strict Wireless ADB endpoint parsing;
- ports limited to 1..65535;
- pairing code must be exactly six digits;
- CR/LF/control-style endpoint input rejected.

The interactive Terminal is intentionally an explicit developer shell.

### 5.2 Process lifecycle — hardened

0.1.1 centralizes process-tree termination.

- Unix children use their own process group; cancellation kills the group.
- Windows uses `taskkill /T /F` with direct process termination fallback.
- executor commands, Logcat, scrcpy sessions, recording and PTY cancellation
  use this hardened lifecycle.
- Terminal sessions own a cancellable context; closing a session cancels an
  in-flight command.
- stdout/stderr are drained concurrently.

Remaining optional hardening:

- native Windows Job Objects would be stronger than `taskkill /T`, but are not
  required for the current trusted ADB/Fastboot/scrcpy toolchain.

### 5.3 File path/archive safety — existing protections verified

Managed archive extraction already:

- rejects absolute paths;
- rejects parent traversal;
- verifies final extraction path stays under destination;
- rejects unsafe/non-regular ZIP entries;
- rejects TAR symlinks and unexpected entry types;
- uses restrictive permissions, not `0777`.

Android file mutations normalize remote paths and protect broad/system trees.

Remaining optional improvement:

- strict mode could refuse user-selected local symlinks during host transfers.

### 5.4 Managed downloads — hardened

0.1.1 fails closed on managed download validation.

Platform Tools 37.0.1:

- pinned to Google's published upstream digest;
- exact upstream size also verified.

scrcpy 4.1:

- managed assets pinned to upstream SHA-256 values;
- non-existent Linux ARM64 v4.1 asset is no longer fabricated; unsupported
  architecture returns an explicit error.

Managed binaries live under DroidSphere's per-user application data area, not
a shared temp execution directory.

### 5.5 Logcat memory pressure — bounded, one improvement remains

Existing/current safeguards:

- frontend queues entries and flushes every 100 ms;
- virtualized React log view;
- bounded rolling buffer;
- min 1,000;
- default 5,000;
- hard maximum 50,000;
- legacy oversized values are clamped;
- invalid IPC preference updates are rejected by Go.

**Remaining P1 improvement:** backend Logcat reader still emits one Wails event
per log line. Batch backend events before IPC during high-volume logging.

### 5.6 Safe Tuning reversibility — verified/hardened

Backend enforcement verified:

- selected package must exist in active profile;
- keep-list/protected packages rejected;
- dangerous/blocked rejected;
- caution acknowledgement enforced;
- advanced uninstall restricted to eligible system apps.

0.1.1 durability hardening:

- temp snapshot/config file is synced before rename;
- best-effort directory sync after rename;
- snapshot mode `0600`.

### 5.7 IPC/WebView — hardened

- no `dangerouslySetInnerHTML` for shell/package/Logcat output;
- React text escaping used;
- mutation-sensitive Go handlers validate inputs;
- router-level recovery screen added for UI exceptions.

### 5.8 Continuous security checks — added

PR CI now includes:

- frontend lint/typecheck/tests;
- normal Go tests;
- `go vet ./...`;
- `govulncheck ./...`;
- Go race detector for concurrency-heavy backend packages;
- Windows/Linux/macOS desktop builds.

Fuzz seeds were added for:

- mDNS parsing;
- Logcat parsing.

Longer fuzz campaigns remain optional/scheduled work.

See `docs/SECURITY_AUDIT.md` for the authoritative item-by-item status.

---

## 6. Architecture review — decisions already made

`docs/ARCHITECTURE_REVIEW.md` evaluates proposed future engineering changes.

Key decisions:

### Do not do

- Do not replace the official ADB CLI wholesale with a custom ADB protocol
  implementation.
- Do not claim a generic scoped-storage bypass.

### P1 — recommended next engineering work

1. **Backend Logcat event batching**
   - Medium complexity.
   - Main remaining audit/performance issue.

2. **ADB push/pull compression**
   - Low complexity.
   - Prefer existing CLI support such as `adb push -z any` and
     `adb pull -z any`; do not reimplement Sync v2 merely for compression.

3. **Transfer SHA-256 verification**
   - Low/Medium complexity.
   - Optional post-transfer verification for large files.
   - Use remote `sha256sum` / `toybox sha256sum` when available.
   - Report verification unavailable instead of pretending success.

### P2

- TV text-entry panel using scrcpy clipboard first, Android clipboard where
  available, `input text` only as simple fallback.
- signed/versioned Safe Tuning metadata feed with explicit licensing and
  rollback.
- scrcpy audio-source/audio-only UX.

### P3

- focused ADB smart-socket client for selected read-only/high-frequency
  operations, only after profiling/benchmarking;
- guarded TV custom-launcher wizard;
- macro engine later.

README also lists App backup/restore and Permission/AppOps inspector as future
product ideas. The user has **not selected a new implementation target yet**.

---

## 7. Distribution state

Ready-to-run **DroidSphere 0.1.1** release-candidate files are committed under
`distribution/`.

### Windows

```text
distribution/windows/DroidSphere-0.1.1-windows-amd64-installer.exe
distribution/windows/DroidSphere-0.1.1-windows-amd64.exe
```

### Linux

```text
distribution/linux/DroidSphere-0.1.1-linux-amd64.AppImage
distribution/linux/DroidSphere-0.1.1-linux-amd64.deb
```

### macOS

```text
distribution/macos/DroidSphere-0.1.1-macos-universal.zip
```

macOS bundle is universal but not Apple-notarized yet.

### SHA-256

```text
7e2bb8a626e69da346bba0d15d47b3c5b1b5c40c119ec3ce62e643464bb987ee  distribution/linux/DroidSphere-0.1.1-linux-amd64.AppImage
a466003cec76df7b771419662ddde1e03310b8547affc52d6ddda825c3a0424a  distribution/linux/DroidSphere-0.1.1-linux-amd64.deb
9e7e90a61bdcd3fdbb30415ead8d39ea5eb3a5972a8679d4271e30430b2957c2  distribution/macos/DroidSphere-0.1.1-macos-universal.zip
d8c3bb82602a092569267c832beaf94a4d55b99eda74239051287e2037a7c7b9  distribution/windows/DroidSphere-0.1.1-windows-amd64-installer.exe
e69a33f0d9c688f2fae6a86f3f6dafeaa46a444fc6543dc0cbebaa58d948f3c8  distribution/windows/DroidSphere-0.1.1-windows-amd64.exe
```

The repository also includes `distribution/SHA256SUMS.txt`.

---

## 8. CI/release verification at handoff

PR #29:

```text
Hotfix Base UI crash and harden DroidSphere 0.1.1
```

was merged successfully.

PR #29 head:

```text
982f892c19248bb394fef38c76c4b10d5cbadf09
```

PR CI run #131 for that head:

```text
success
```

Post-merge `main` CI run #132 for merge commit
`854b65075d07e3c2bb73388419db509466d3c91b`:

```text
success
```

DroidSphere 0.1.1 multi-platform distribution run #49:

```text
success
```

Thus the handoff begins from a **green main**.

---

## 9. PR history relevant to current architecture

Important completed PRs:

- **#23** — Current+ visual completion, Tool Locations, settings backup and
  remembered-device selector.
- **#24** — Safe Tuning profiles and reversible restore.
- **#25** — PC ↔ Android dual-pane File Manager.
- **#26** — Linux/macOS build gates and portability hardening.
- **#27** — TVADB Hub → DroidSphere rebrand and generic Android-device scope.
- **#28** — Connection Doctor + How to install + committed Windows/Linux/macOS
  distribution packages.
- **#29** — Base UI #31 runtime hotfix + security hardening + DroidSphere 0.1.1.

Do not reopen/reimplement those features unless a new bug or requirement
specifically requires it.

---

## 10. User/product decisions to preserve

- User prefers Russian for this project.
- Work autonomously; do not repeatedly ask whether to continue.
- Use the connected GitHub repository and commit/push/merge changes yourself.
- For substantial work, keep the user informed with concise progress updates.
- Current+ is the chosen UI architecture.
- Do not replace Current+ with a permanent generic sidebar.
- One remembered-device selector is sufficient; no full Fleet Mode is needed.
- Large standalone Health Center is not a priority.
- TV Macros / Support Bundle are not current priorities.
- Settings backup/export is already required and implemented.
- Product scope is all Android devices, not only TVs.
- Preserve compatibility with old TVADB settings/data where practical.
- Safety and reversibility are preferred over aggressive device mutations.

---

## 11. What to do first in the next session

The user explicitly said this work phase could end after the current fixes.

Therefore, in a new session:

1. Synchronize `main` and verify the checkpoint.
2. Read this handoff and the referenced security/architecture documents.
3. Do not invent a new feature automatically unless the user asks to continue.
4. If the user reports a DroidSphere 0.1.1 runtime problem, reproduce/analyze
   that first and hotfix it.
5. If the user asks to continue feature development without specifying a
   feature, use the accepted architecture priorities:
   - P1 backend Logcat batching;
   - P1 ADB transfer compression;
   - P1 transfer SHA-256 verification.
6. Create a fresh feature branch from current `main`.
7. Preserve all newer work if `main` has advanced.
8. Require relevant tests and the full multi-platform CI before merging.

### Recommended verification by the user

The most useful next manual validation is to install/run DroidSphere 0.1.1,
open the Current+ device dropdown and confirm that Base UI error #31 no longer
occurs. Then smoke-test:

- device selection/reconnect;
- Connection Doctor;
- Apps;
- Safe Tuning analysis only;
- dual-pane file browsing/transfer;
- scrcpy;
- Terminal/Logcat.

---

## 12. New-session prompt template

Use the following prompt in a new ChatGPT session:

```text
Continue the DroidSphere project in:

mrAibo/tvadb-hub

Do not ask me to re-explain the project.

A canonical handoff exists in:

DROIDSPHERE_HANDOFF_2026-09-30.md

Read it completely first, then read the documents it references.

Synchronize the repository yourself:

git fetch origin
git checkout main
git pull --ff-only origin main
git rev-parse HEAD

The expected main checkpoint at handoff creation is:

854b65075d07e3c2bb73388419db509466d3c91b

Commit:
Merge pull request #29 from mrAibo/fix/base-ui-menu-group-error
Hotfix Base UI crash and harden DroidSphere 0.1.1

If main has advanced beyond that SHA, inspect every intervening commit before
modifying anything and preserve valid newer work.

Important current state:
- DroidSphere version 0.1.1.
- PR #29 is merged.
- Base UI production error #31 is already fixed.
- Router error recovery UI is already implemented.
- External security-audit hardening is already implemented and documented in
  docs/SECURITY_AUDIT.md.
- Architecture recommendations are documented in
  docs/ARCHITECTURE_REVIEW.md.
- Windows/Linux/macOS 0.1.1 packages and SHA-256 checksums are committed under
  distribution/.
- main CI is green.
- Do not continue from fix/base-ui-menu-group-error; it is already merged.

Use the connected GitHub tools, create fresh branches from current main, commit
and push changes yourself, open/review/merge PRs yourself after CI is green.
Keep me informed during longer work, but do not stop unnecessarily.

Continue from the actual repository state and my next instruction.
```

---

End of handoff.
