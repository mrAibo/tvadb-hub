# DroidSphere — audit remediation and P3 handoff

Date: 2026-09-30 (historical file name and date kept for continuity). Content last verified
**2026-10-01** against `main` `535295c9f63d7aab2cdd286fc86e1b5e0d4c938a` (tree
`c7f65351a8b067aeb3ba2917514a3c7e151aae35`). Repository: `mrAibo/tvadb-hub`. Product: DroidSphere.
This documentation refresh travels in the open docs-only PR **#59** (base `535295c9…`); when it is
reviewed and merged, the final docs/main commit must be reconciled against that base — no later
commit is claimed here.

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

Historical checkpoint at the previous milestone: `41967b679551dc190d02461b5d8850245babaa23`,
merge of PR #45 (post-merge run [36752405380](https://github.com/mrAibo/tvadb-hub/actions/runs/36752405380)).
That checkpoint is superseded.

Verified source checkpoint at **2026-10-01**: `535295c9f63d7aab2cdd286fc86e1b5e0d4c938a`
("Merge pull request #57 from mrAibo/fix/fastboot-ui"), tree
`c7f65351a8b067aeb3ba2917514a3c7e151aae35`, working tree clean, open PRs = 0.
Post-merge PR CI run [36817054417](https://github.com/mrAibo/tvadb-hub/actions/runs/36817054417)
attempt 1 passed **six** GitHub Actions checks (frontend-check, go-test, windows-internal-tests,
linux-build, macos-build, windows-build) and the platform artifacts carry `BUILD_PROVENANCE` with
`sourceRevision = 535295c9f63d7aab2cdd286fc86e1b5e0d4c938a`. Exactly six checks are green — see
"What is deliberately not claimed" below; the external `Kilo Code Review` check did not run.

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
- #40: durable pending intent before mutation, exact installed/enabled-state
  recovery, interrupted-operation reconciliation and unique atomic-write temps.
- #41: advertised clipboard set/get support and exact readback before TV PASTE;
  no false inference of scrcpy control access from an external process.
- #42: detached configuration snapshots, synchronized access, selection
  deadlock prevention and application facade race checks.

All three were integrated with current main before merging. Post-merge runs
[36743305430](https://github.com/mrAibo/tvadb-hub/actions/runs/36743305430) (#40),
[36746045096](https://github.com/mrAibo/tvadb-hub/actions/runs/36746045096) (#41),
and [36749845032](https://github.com/mrAibo/tvadb-hub/actions/runs/36749845032) (#42)
passed all five jobs. These packages are complete; do not implement them again.

- #45: joined streaming output, bounded diagnostic tails, reader errors,
  deterministic cancellation/reaping, ordered Logcat final batches/status and
  CRLF coalescing. Fresh integration CI 36750569010 and post-merge main CI
  36752405380 passed all five jobs. `docs/STREAM_LIFECYCLE.md` describes the
  regression coverage; no physical-device validation is implied.

## Published but not yet merged at the 2026-09-30 milestone (historical)

The table that stood here listed PR #46 and PR #47 as published and pending. Both were merged
later the same day (#46 `2026-09-30T17:56:07Z`, #47 `2026-09-30T18:40:48Z`), and every later
package is now merged too — the live merged set is in "P3 source closure" below. This section is
kept only as history; do not re-open, re-implement or re-review those packages from it.

Transfer changes passed the production frontend build and 114 frontend tests at that milestone;
full Go tests are verified in CI, never inferred from Windows cross-compilation.

## P3 source closure (2026-10-01)

Every confirmed P3 package is merged on `main` `535295c9`. Each row names the independent review
gate that judged the patch; identity was checked byte-wise against the **latest reviewed head of that
package** (never inferred from PR titles). Where a later reviewed commit legitimately changed files,
the row says which head the final content matches.

| Package | Integrated as | Independent gate (workspace-root document) |
| --- | --- | --- |
| Device-bound Apps/Files, selection queue, monitor/cache, confirmed-device API | #53 (Go twin API + frontend guards) | `DEVICE_TARGET_INTEGRATION.md`: t27 PASS (`3f7fa7e5…`) + t50 PASS (`306a1656…`) |
| UX: Ctrl/K ownership, real theme toggle, protected preferences draft | #51 | t39 PASS r2 (`2023089…`) |
| Recording lifecycle: single `Wait` owner, fail-closed stop, bounded prompt probe | #54 | t26 PASS (`2206b09e…`) |
| Recording test-helper readiness + repeated race gate `-count=20` | #58 | t69 PASS (`b7382ff…`) |
| Strict fastboot twins (blank refusal before any tool work, pinned target) — **original M1 backend half** | #57 Go base | t54 PASS (`abd66e16…`) |
| Flash consent for partition/batch through strict RPC twins — **original M1 UI half** | #57 UI, baseline reviewed at t59 (`0184c44f…`, tree `e7abc456…`) | final content matches the **t75** head below: M3 legitimately changed 7 files, and t75 preserves the t59 baseline (an "empty diff against `0184c44`" would be wrong) |
| **M3**: captured target/inputs confirmation for wipe and sideload | #57 commit `9725375` | t75 PASS (`f4fd252…`, tree `d6910324…`) |
| Guarded Launcher Wizard backend (capability preflight, unique-HOME fail-closed, durable journal, cancel, rollback, offline recovery) | #56 + `1e056e3f…` | backend gate **t45 PASS** (kept separate); docs t48 |
| Launcher Wizard UI (Current+, truthful copy, offline recovery errors) | #56 | **t46 (round 1, `needs_revision`) → t71 (repair) → t72 (round 2, PASS)**; the round-2 review `LAUNCHER_WIZARD_REVIEW_R2.md` covers head `91623eef…` |
| Scrcpy preset persistence + non-clobber preferences | #55 | t56 needs_revision → **t67 PASS on reviewed head `30c9ca5…`**; the integrated head `bb7b578e…` (= merge `6411a71e^2`) preserves the same six blobs |
| Dataset endpoints/benchmark evidence and toolchain provenance (earlier phase) | #48, #49, #50 | t13/t14/t32/t34 records |

M1 (original scope) = the two confirmed cards (`PartitionFlashCard`, `RomFlashCard`): strict
`…ForDevice` RPC twins with a captured serial, and a consent dialog that shows the captured target.
M3 is a **deliberate minimal safety extension** discovered while fixing M1 — the same wrong-target
risk existed on the wipe and sideload cards, which were outside the original two-card C-12 scope.
It is not a roadmap item: it adds captured consent (serial plus, for sideload, the ZIP and mode),
keeps wipe/partition/batch requiring a live fastboot target, and accepts a sideload target whose
fastboot list is legitimately empty (an ADB-recovery device). All four destructive flows share one
synchronous dispatch admission checked before the first await. It is **not** a claim that every
power-user fastboot action (custom command, active-slot change, WOF helpers) is consent-gated.

Recording readiness (#58/#70): the Linux `-race` failure was a **test-harness readiness gap** (the
fake recorder's `os.WriteFile` creates the file before writing it, while the old wait checked only
existence), not a product bug and not a data race; the product's fail-closed empty-output path was
correct. The fix is test-only, and the repeated `-count=20` race step is now a CI gate.

Historical clarifications kept truthful (metadata, not source defects):
`t58`'s extra sibling test file was treated as outside its authored scope and was formally adopted
by the `t59` review — the earlier "directory/parser bug" explanation is **not** claimed (unproven).
`t55`'s verification had one cached run in which the fourth verify appeared missing and had to be
retried — an environment/caching artifact, not a validator bug. In the t68 attempt the literal
`gh pr checks --watch --fail-fast` form did not resolve the PR, so verification used the explicit
per-PR proof instead — the same command with a PR number/target, as reported by t55 — and was
independently reviewed (`t69`); there is no claim that the literal form itself passed.

**Release versus build/packaging (two separate statements).** The release/distribution workflows
(`release.yml`, triggered by `v*` tags, and `distribution.yml`) **never ran**: nothing was published,
signed, notarized or deployed, there are no tags or GitHub releases, and repository permissions were
not changed. Separately, the CI build/packaging steps **did execute**: `windows-build` produced the
Windows binary and the NSIS installer, `linux-build`/`macos-build` produced their platform binaries,
and those artifacts carry `BUILD_PROVENANCE` naming the exact revision (digests recomputed during
verification). Building on CI is not publishing.

Three metadata errors from the earlier whole-RC verification report (root
`SOURCE_RC_VERIFICATION.md`, t61) are corrected here and in the root ledgers and must not be copied
forward: (1) the final flash-UI content maps to the **t75** head `f4fd252…`, **not** to t59's
`0184c44…` — M3 legitimately changed 7 files and t75 preserves the t59 baseline; (2) the
scrcpy-presets **reviewed** head is `30c9ca5…` (t67), while `bb7b578e…` is the integration head that
carries the same six blobs; (3) the Launcher-UI chain is **t46 (round 1, `needs_revision`) → t71
(repair) → t72 (round 2, PASS)** with the backend gate t45 kept separate, and the
`sourceDirty: true` cause recorded by all three provenance files is **unproven** (hypothesis, not a
measurement) — no clean-source-built binary claim is made.

Final source-RC evidence (`t61`, root `SOURCE_RC_VERIFICATION.md`): six GitHub Actions checks green
on `535295c9` **plus** the Linux `./internal/launcher` race execution, the separate
`go test -race ./internal/scrcpy -count=20` gate, the Windows internal/atomic-write steps, and
three platform artifacts whose `BUILD_PROVENANCE` names this revision (digests recomputed).
Still open and explicitly **not** executed: physical Fire TV/Google TV validation (issue **#19**,
firmware/OEM behaviour unproven), administrator-enforced branch protection/required checks,
distribution publishing, signing/notarization, tags/releases/deployments, and Git-history rewriting.
`C-13` and `C-14` remain unproven.

## Accepted remaining sequence

1. ~~Finish #46 current-main integration/full CI, merge and verify main.~~ **Done** (merged
   2026-09-30; see "P3 source closure").
2. ~~Finish #47 integration/full CI, merge and verify main.~~ **Done** (merged 2026-09-30). Unknown
   recovery capability still blocks disabling actions; enabling recovery remains possible.
3. Benchmark: CLI and smart socket must use the same endpoint; alternate paired
   samples; check snapshot consistency; export raw/summary JSON evidence.
4. Validation/distribution: make frontend typecheck actually check application,
   tooling and tests; pin Bun/Wails/NSIS across all workflows; associate build
   artifacts with exact source SHA. Do not claim old binaries are current.
5. Guarded TV Custom Launcher Wizard. This is the next accepted P3 product
   feature after the safety prerequisites, not macros.
6. Final review, documentation, full CI, updated handoff.

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
toolchain. Published PRs are intact. Early stream/PTY edits were rebuilt and
are now published in #45; do not rebuild or duplicate that package again.
Local GTK/WebKit dependency installation was denied by environment permissions;
do not bypass them. Full desktop-dependent testing/builds use GitHub CI.

Unit/fake-device tests do not establish real firmware compatibility. Physical TV
validation, user experience smoke tests and cross-platform ADB measurements are
still required. Unknown/ambiguous package-state output must fail closed.

Environment limits versus CI evidence (do not conflate them):

- Local `bun run test` (Vitest) cannot start on this Windows host — the Vite/spawn path fails with
  `EPERM`. UI runtime proof therefore comes from CI's `frontend-check` job, never from a local pass.
- Local `go test -race` is impossible here (no cgo/gcc). Race proof comes from the CI Ubuntu jobs,
  including `./internal/launcher` and the repeated `./internal/scrcpy -count=20` step.
- `go test ./...` (repo root) needs the built frontend for `//go:embed`; use
  `go test ./internal/...` locally and let CI run the root package.
- Build provenance for the RC records `"sourceDirty": true` on all three artifacts. Its **cause is
  unproven**: the "`wails3 generate bindings` rewrites the tracked `frontend/bindings/**`"
  explanation is a hypothesis (and the earlier PR49-F1 "different artifact byte-target" claim was
  refuted), so no dirty-path list or log is asserted. The recorded revision is certified; **no
  pristine-checkout and no clean-source-built binary claim** is made until dirty paths are captured
  from an actual build log.
- Exactly **six** CI checks are green. The external `Kilo Code Review` check is `action_required`
  ("Insufficient credits to run review") — no review signal, not a code finding, and never counted
  as a seventh green check. No payment/plan/admin change was attempted or is authorized here.
- No wall-clock or hardware behaviour is asserted: no device, emulator, adb or fastboot command was
  run for this RC, and the FULL Go settings export/import round-trip (presets surviving a real
  backup/restore) has no end-to-end test yet — it remains an optional hardening item.
- This handoff is documentation only. Commenting the docs refresh into issue #43 happens after the
  independent review of the docs PR and its merge, not before.

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
