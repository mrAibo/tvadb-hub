# DroidSphere status

Date: 2026-10-13. Source status reconciled against merged PR #80 and its full PR CI.

Stable historical implementation detail remains in
[DROIDSPHERE_HANDOFF_2026-09-30_P3.md](DROIDSPHERE_HANDOFF_2026-09-30_P3.md).
The live execution record is [issue #43](https://github.com/mrAibo/tvadb-hub/issues/43).

Fact labels used below: **implemented** (source merged), **unit/CI verified**
(automated evidence exists for that exact commit), **hardware validated** (a real
device passed the checklist) and **published** (release assets are public). The current source features are implemented and CI-verified; real hardware validation
and publication remain separate pending gates.

## Verified source checkpoints

- Latest merged source at this status edit: **`7bb2f66b7639c2f1fdb3fcf321f311219ef0e5a6`** (PR #80, bounded read-only host/Android text and image file previews). Feature-head CI [38392192797](https://github.com/mrAibo/tvadb-hub/actions/runs/38392192797) completed **success 6/6**, including generated binding provenance on Windows. A distinct post-merge `main` CI run [38393151131](https://github.com/mrAibo/tvadb-hub/actions/runs/38393151131) was started; consult the linked run and [issue #43](https://github.com/mrAibo/tvadb-hub/issues/43) for its final result. This is not physical device acceptance.
- Previous `main` baseline `5a96704a403da931c2628114af921abc0a526b3c` (status PR #78): separate post-merge CI [38390535831](https://github.com/mrAibo/tvadb-hub/actions/runs/38390535831) **success 6/6**.
- Security PR #79 merged as `c3da9348c96f36cc4b74f2cd828de3e9b8344245`: vulnerable frontend transitive packages removed/upgraded, Go pin upgraded to 1.26.9, full post-merge `main` CI [38271678252](https://github.com/mrAibo/tvadb-hub/actions/runs/38271678252) **success 6/6**.
- Earlier October 2 wireless/release/Logcat-polish baseline: `b3a2dc1a932b20afd5c3efba33c8f30d04a42e0a`, separate post-integration `main` CI [36990976089](https://github.com/mrAibo/tvadb-hub/actions/runs/36990976089) **success 6/6**.
- Earlier PR #63 (Logcat 2.0 phase 2), PR #61 (release readiness), PR #60 (stability) and PR #56 (guarded TV launcher) are all merged. Their features must not be re-implemented.
- The 2026-10-02 wireless discovery/pairing, release-evidence, and Logcat-polish batch (B1/B2/B3) is now **implemented + unit/CI verified** on the above `main`, rather than in progress. See the local historical batch record `TVADB_BATCH_2026-10-02.md` in the engineering workspace. Hardware gate B5 and administrator gate A1 are **not** complete.
- A passing CI run verifies automated checks for this source SHA; it is **not** evidence that a specific Windows installer, running application, Android device, or firmware has passed end-to-end testing.

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

### Dependency and publication findings

- PR #79 removed the unused vulnerable `shadcn` CLI dependency, retained its MIT-licensed CSS, upgraded `source-map-js` to 1.2.2 and pinned Go 1.26.9 to fix the reachable standard-library advisories. Both `bun audit --audit-level=high` and `govulncheck ./...` remain enabled. Future vulnerability advisories may require additional fixes; re-evaluate on later dependency changes.
- The release/distribution policy has automated checks, but **no production publishing workflow, signed release, notarization, or physical-device validation is claimed** by these tests. A real publish remains an explicit later decision.

## Remaining release gates

1. **Physical Android/TV validation — issue #19.** Real firmware/OEM behaviour,
   Wireless ADB pairing/reconnect and the intended Windows package workflows are
   not proven until the checklist passes on hardware.
2. **Repository administration — issue #62.** At the PR #60 checkpoint `main` was
   not protected and repository rulesets were empty, and the connected GitHub App
   does not expose branch-protection writes. Required-check enforcement remains an
   administrator action tracked explicitly in
   [issue #62](https://github.com/mrAibo/tvadb-hub/issues/62).
3. **Remote CI for the 2026-10-02 source batch: complete.** Exact-SHA main CI [36990976089](https://github.com/mrAibo/tvadb-hub/actions/runs/36990976089) passed 6/6. Hardware validation remains pending.
4. **Publication/signing.** No GitHub Release, tag deployment or notarized macOS
   distribution is claimed by this status file. Authenticode signing remains
   optional and secret-dependent.
5. **Evidence-only ADB smart-socket adoption.** Production discovery remains on
   the official ADB CLI until repeatable same-endpoint measurements across
   supported hosts/transports justify a change.

## Next product work after the verified batch

The wider backlog remains in [docs/ROADMAP.md](docs/ROADMAP.md): read-only file preview is source-complete via PR #80 (hardware unvalidated). Next is folder sync with a dry-run/conflict policy, transfer history/resume and APK backup/restore,
Permission/AppOps inspection, device report export, device-side recording and
later automation/distribution work. Logcat 2.0 phase 2 (PR #63) and the guarded
custom launcher wizard (PR #56) are already completed and are not upcoming work.

## Continuation rule

Before every new package, synchronize current `main`, inspect commits newer than
this checkpoint and preserve valid work. Use a fresh branch, run the full CI
suite, merge only green and verify the post-merge `main` run. Record each
material transition in issue #43.
