# DroidSphere status

Date: 2026-10-02.

Stable historical implementation detail remains in
[DROIDSPHERE_HANDOFF_2026-09-30_P3.md](DROIDSPHERE_HANDOFF_2026-09-30_P3.md).
The live execution record is [issue #43](https://github.com/mrAibo/tvadb-hub/issues/43).

Fact labels used below: **implemented** (source merged), **unit/CI verified**
(automated evidence exists for that exact commit), **hardware validated** (a real
device passed the checklist) and **published** (release assets are public). The
current batch is none of the last three.

## Latest verified application baseline

- `main` checkpoint `68c6cc62667494bcce1a4e747f0cbf05783287c8`: merge of PR #63
  (`feat/logcat-phase2`), which sits on PR #61
  (`chore/release-readiness-status`) and PR #60 (`phase0/stability-candidate`).
- Post-merge CI run
  [36934452447](https://github.com/mrAibo/tvadb-hub/actions/runs/36934452447)
  passed all six repository jobs: `frontend-check`, `go-test`,
  `windows-internal-tests`, `windows-build`, `linux-build` and `macos-build`.
  This is the verified baseline for `68c6cc6`, **before** the current batch diff.
- At the PR #60 checkpoint (run
  [36874540492](https://github.com/mrAibo/tvadb-hub/actions/runs/36874540492))
  the Windows raw-byte witness checked 34 tracked generated/toolchain-sensitive
  paths and reported `mismatched=0`.
- PR #56 (guarded TV custom launcher wizard), PR #60, PR #61 and PR #63 are
  merged and are **implemented + unit/CI verified**. Do not re-implement the
  completed Logcat lifecycle/batching, Logcat 2.0 phase 2 (crash/ANR filters,
  saved filters, app/PID filtering, pinned events), transfers, Safe Tuning,
  device-target safety, Launcher Wizard, scrcpy lifecycle/presets or destructive
  Fastboot consent packages.

## Current batch — IN PROGRESS, not verified

The wireless discovery/pairing, release-evidence and Logcat-polish batch from
[TVADB_BATCH_2026-10-02.md](TVADB_BATCH_2026-10-02.md) is in progress. Work in
this tree is **not** part of the `68c6cc6` baseline, has **no green CI run** and
must not be reported as complete or CI-verified until a post-merge run exists.

- **B1 wireless discovery/pairing UX (other workers, in progress).** The backend
  already recognises all three official ADB service types — `_adb._tcp`,
  `_adb-tls-pairing._tcp` and `_adb-tls-connect._tcp` — and PairAndConnect already
  performs a bounded rediscovery of the connect service. The manual pairing UI
  and the target-bound `tcpip` consent flow are still being finished, so the
  end-to-end workflow is not done.
- Do not conflate two different actions: the Connection Bar **Refresh** calls
  `refreshDevices` (`adb devices -l`), which is *not* mDNS discovery of new
  unpaired candidates; wireless discovery is a separate path. On the reported
  Sony Bravia 7 / XR70 case (Google TV, Android 12) the TV's pairing window must
  stay open while the code is entered, and the pairing port differs from the
  connect port. The cause of that specific hardware failure is **not** declared
  established without runtime evidence.
- **B2 release evidence + dependency monitoring (this package).** Implemented and
  locally tested; the workflow/diff has not run in remote CI yet.
- **B3 Logcat polish (other workers, in progress)** in the same tree.
- No hardware validation has been performed for this batch: it is source-level
  work only.
- Independent verification (B4) and physical validation of a frozen candidate
  (B5) are still ahead.

## Release-readiness hardening (PR #61 retained)

- The multi-platform distribution workflow is manual-only, uses read-only
  repository contents permission and no longer pushes generated binaries back
  into a branch. No generated artefact is committed by CI.
- Publishing a Windows GitHub Release requires repository variables
  `PHYSICAL_DEVICE_VALIDATED_VERSION` **and** `PHYSICAL_DEVICE_VALIDATED_SHA` (a
  full Git commit SHA: currently 40 hex on GitHub, or 64 for SHA-256 Git IDs) to match the project version and the
  `github.sha` actually checked out, for **both** `v*` tag and manual publishing.
  A missing, malformed or mismatched value fails closed. A pushed `v*` tag can no
  longer bypass the gate.
- Manual publishing additionally requires the explicit
  `physical_device_validated=true` workflow input. An unpublished candidate build
  requires none of these variables.
- Published assets are never replaced: a run that finds the release already
  present fails instead of uploading with `--clobber`.
- The gate logic lives in a pure PowerShell helper with unit-like tests under
  `scripts/new-release-authorization-helper/`. The tests run locally, cover the
  tag/manual/no-publish/missing-version/missing-or-wrong-SHA/consent matrix and
  never publish anything.
- Dependency monitoring: Dependabot version updates for `bun` (text
  `frontend/bun.lock`), `gomod` and `github-actions`, with a 7-day minimum release
  age (`cooldown.default-days`). `bun audit --audit-level=high` gates the existing
  `frontend-check` job (still six jobs) and the existing `govulncheck` step is
  preserved.
- Source-SHA authorization proves which commit was validated; it is **not** proof
  that a later rebuild produces bit-identical artefacts. Validated candidate
  artifact hashes must be retained with the validation notes, and a rebuild from
  the same source must not be described as physically tested.

### Known open release findings

- This batch updates only the compatible `brace-expansion` lock entry from
  **5.0.9 to 5.0.12**. With pinned Bun 1.4.2, `bun audit --audit-level=high`
  passes locally; full audit still reports **3 moderate** transitive findings
  in `fast-uri` and `ip-address` via `shadcn` tooling, below the high gate.
  The lock update is in scope; remote CI for this diff is still pending.
- The new workflow diff is **not executed**: `release.yml` publication,
  `distribution.yml`, the dependency-update bot and remote CI all require
  Actions runs and are pending, not verified. No tag, release, workflow dispatch
  or GitHub settings write has been performed.

## Remaining release gates

1. **Physical Android/TV validation — issue #19.** Real firmware/OEM behaviour,
   Wireless ADB pairing/reconnect and the intended Windows package workflows are
   not proven until the checklist passes on hardware.
2. **Repository administration — issue #62.** At the PR #60 checkpoint `main` was
   not protected and repository rulesets were empty, and the connected GitHub App
   does not expose branch-protection writes. Required-check enforcement remains an
   administrator action tracked explicitly in
   [issue #62](https://github.com/mrAibo/tvadb-hub/issues/62).
3. **Remote CI for the current batch diff.** Pending; no green run exists yet for
   the in-progress wireless/release/Logcat diff.
4. **Publication/signing.** No GitHub Release, tag deployment or notarized macOS
   distribution is claimed by this status file. Authenticode signing remains
   optional and secret-dependent.
5. **Evidence-only ADB smart-socket adoption.** Production discovery remains on
   the official ADB CLI until repeatable same-endpoint measurements across
   supported hosts/transports justify a change.

## Next product work after the current batch

The wider backlog remains in [docs/ROADMAP.md](docs/ROADMAP.md): file preview,
folder sync/conflict policy, transfer history/resume, APK backup/restore,
Permission/AppOps inspection, device report export, device-side recording and
later automation/distribution work. Logcat 2.0 phase 2 (PR #63) and the guarded
custom launcher wizard (PR #56) are already completed and are not upcoming work.

## Continuation rule

Before every new package, synchronize current `main`, inspect commits newer than
this checkpoint and preserve valid work. Use a fresh branch, run the full CI
suite, merge only green and verify the post-merge `main` run. Record each
material transition in issue #43.
