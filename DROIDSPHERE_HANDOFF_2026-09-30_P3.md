# DroidSphere — audit remediation and P3 handoff

Date: 2026-09-30. Repository: `mrAibo/tvadb-hub`. Product: DroidSphere.

This file supersedes the earlier PR #29 handoff. Read it completely before
changing code. Do not ask the user to re-explain the project.

## Continuation protocol and live state

The durable, frequently updated execution record is
[issue #43](https://github.com/mrAibo/tvadb-hub/issues/43).
Read its current body before choosing the next action. This file records a
stable milestone; the issue records later commits, CI results, merges,
blockers and the next action without a documentation commit for every poll.

After each material transition, update issue #43: commit/PR publication,
CI failure/success, merge, post-merge verification, or change of work package.
Do not call a feature complete merely because its PR is open or CI is running.
Refresh this handoff after completed engineering packages.

Before modifying code:

```bash
git fetch origin
git checkout main
git pull --ff-only origin main
git rev-parse HEAD
```

Verified source checkpoint before this documentation package:
`e194c441d9a9967009d463ead02c017e3cdaea51`, merge of PR #38.
Post-merge run [36710126650](https://github.com/mrAibo/tvadb-hub/actions/runs/36710126650)
passed frontend-check, go-test, Windows, Linux and macOS builds.
Inspect every intervening commit if main is newer and preserve valid newer work.
Never overwrite a parallel session's valid changes or duplicate merged features.

## Completed foundation and engineering work

The application already supports USB/Wireless ADB, pairing/mDNS, remembered
devices/reconnect, dashboard/device details/performance, TV remote, app/APK/split
APK management, dual-pane files, Safe Tuning, scrcpy, screenshots/recording,
clipboard, shell/Logcat, Fastboot, managed tools, settings backup and Connection
Doctor. Windows/Linux/macOS build gates and older 0.1.1 release candidates exist.
Committed distribution packages must not be assumed to include later source PRs.

Accepted P1/P2 packages are merged:

- #30: backend Logcat batching (~75 ms, 128 entries, final partial flush).
- #31: capability-aware official ADB CLI transfer compression.
- #32: optional SHA-256 transfer verification.
- #33: TV text entry (clipboard when supported; restricted ASCII fallback).
- #34: scrcpy audio sources and audio-only sessions.
- #36: Logcat crash/ANR filters and saved views.
- #35: signed/versioned Safe Tuning metadata feed with cache rollback.
- #37: read-only `internal/adbproto` `host:devices-l` prototype and benchmark.

Production discovery still uses the official `adb devices -l` path. Smart sockets
have NOT been adopted. Measurements across Windows/Linux/macOS and no-device,
USB, Wireless and multiple-device conditions remain an evidence gate.

New audit fixes merged:

- #38: confirmed-target binding across analysis/apply/restore, explicit user 0,
  stale frontend response rejection, strict model/codename profile eligibility.
- #39: applicable built-in safety floors independent of external profile IDs;
  serialized feed refresh/rollback and trust recheck before cache/state commit.

## Published but not yet merged at this milestone

| PR | Scope | Head SHA | Verified full PR CI |
| --- | --- | --- | --- |
| #40 | Durable intent journal before mutation; exact installation/enabled-state restore; pending/unknown reconciliation; unique atomic-write temporary files | `e6088a474a36452806c814c8a7f5e32458e1a1b6` | [36710393028](https://github.com/mrAibo/tvadb-hub/actions/runs/36710393028) |
| #41 | Detect advertised clipboard set/get support, exact readback before TV PASTE, no false scrcpy transport inference | `88d4bb77e254d3107edea5d7a0bedd6c3a97a823` | [36710693943](https://github.com/mrAibo/tvadb-hub/actions/runs/36710693943) |
| #42 | Detached configuration snapshots, synchronized reads/writes, selection deadlock prevention, app facade race CI | `771e400fc82b3906b865924a1898659c6db04976` | [36710807103](https://github.com/mrAibo/tvadb-hub/actions/runs/36710807103) |

All five jobs in each listed run passed. An interrupted tool call did not merge
#40. Read issue #43 and current PR metadata before acting; these may be merged
after this document is written. Integrate current main into pending PRs and
validate their combined tree, then merge one at a time and check main CI.

## Accepted remaining sequence

1. Complete #40 -> #41 -> #42 with integration and post-merge CI.
2. Streams and Logcat: finish stdout/stderr reads before Wait closes pipes,
   surface scanner failures, preserve bounded batching/final flush, and make
   cancellation/reader failure cleanup deterministic.
3. Transfers: prevent overlapping operations from replacing each other's cancel
   function; pin target/tool/preferences for the whole batch; return truthful
   per-file results; distinguish unsupported host hashing types from real I/O
   failures; classify retries from actual stderr/stdout diagnostics.
4. Extend Safe Tuning protection to discovered current HOME/IME packages for
   unknown OEMs, not only a static list of known recovery packages.
5. Benchmark: CLI and smart socket must use the same endpoint; alternate paired
   samples; check snapshot consistency; export raw/summary JSON evidence.
6. Validation/distribution: make frontend typecheck actually check application,
   tooling and tests; pin Bun/Wails/NSIS across all workflows; associate build
   artifacts with exact source SHA. Do not claim old binaries are current.
7. Guarded TV Custom Launcher Wizard. This is the next accepted P3 product
   feature after the safety prerequisites, not macros.
8. Final review, documentation, full CI, updated handoff.

For each package use a fresh branch from current main, tests and documentation,
commit/push/open PR autonomously, fix full CI, merge only green and verify main.
Do not stop after merely opening a PR. Keep short progress updates in Russian.

## Launcher wizard requirements

Integrate into the existing TV experience. Preflight must identify the pinned
target, SDK/device family/current HOME/candidates and supported operations.
Do not expose firmware-dependent operations universally across Fire TV,
Google TV and OEM Android TV. Unknown capability means no automatic apply.

Before changing HOME: validate installed/enabled candidate and resolvable HOME
activity, test launch where possible, capture original configuration durably,
show target/current/requested launcher, exact actions and recovery path, and
require explicit confirmation. Apply only capability-appropriate, reversible
operations. Verify HOME resolution/launch afterwards and automatically restore
the captured state if verification fails where safe. Expose a manual ADB
recovery path. Never silently uninstall/disable the stock launcher or use broad
destructive shell tricks. Backend tests must cover decisions and rollback;
frontend tests must cover stale target/confirmation/error handling.

## Architecture and compatibility boundaries

- Preserve Current+: bottom dock, persistent compact device/status bar,
  connected/remembered selector and host settings. No generic permanent sidebar.
- Keep repository slug, historical `ADBKit/...` imports, legacy app-data/settings
  compatibility and updater references unless explicitly authorized otherwise.
- Official Platform Tools remain the compatibility baseline. No custom Sync
  replacement and no wholesale custom ADB client adoption.
- Safe Tuning protection is enforced in Go, not only UI. External/community
  metadata must never weaken built-in protected/dangerous rules. No silent
  GPL/LGPL dataset vendoring into the MIT project.
- Keep structured subprocess execution and process-tree cancellation. No host
  `sh -c` / `cmd.exe /c` wrappers for ordinary device operations. Validate inputs
  at the backend boundary and preserve portable native path handling.
- Preserve Logcat backend batches/frontend bounds, existing scrcpy service and
  conservative restricted text fallback. A scrcpy process is not a usable
  protocol control channel by itself.
- No Fleet Mode, standalone Health Center or early macro priority. Later macros
  must use inspectable, cancellable safe service operations.

## Limitations and recovery facts

Automatic workspace maintenance removed the previous transient checkout and
toolchain. Published PRs are intact. Early stream/PTY edits were uncommitted and
not pushed; rebuild them from the requirements above, not from assumed code.

Unit/fake-device tests do not establish real firmware compatibility. Physical TV
validation, user experience smoke tests and cross-platform ADB measurements are
still required. Unknown/ambiguous package-state output must fail closed.

Repository administration is not available through the connected GitHub app;
administrator-enforced branch protections remain an explicit external task.
Do not claim they were configured. Notarized macOS/public package publishing
and unrelated roadmap features are later work.

## New-session startup

Read this file and issue #43 completely; synchronize main; inspect newer commits
and open PRs; read `docs/ARCHITECTURE_REVIEW.md`, `docs/ROADMAP.md`,
`docs/SECURITY_AUDIT.md`, `docs/ADB_SMART_SOCKET_BENCHMARK.md` and relevant
operation documentation. Continue the next incomplete accepted item without
asking for repeated authorization to branch, push, open PR, monitor or merge.
Record verified outcomes and next action in issue #43 throughout execution.
