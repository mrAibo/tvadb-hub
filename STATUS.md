# DroidSphere status

Date: 2026-10-01.

Stable historical implementation detail remains in
[DROIDSPHERE_HANDOFF_2026-09-30_P3.md](DROIDSPHERE_HANDOFF_2026-09-30_P3.md).
The live execution record is [issue #43](https://github.com/mrAibo/tvadb-hub/issues/43).

## Latest verified application baseline

- `main` checkpoint before this release-readiness documentation/workflow package:
  `f122983eb911051de192412db17d22cf01f18ee9` (merge of PR #60,
  **Phase-0 stability: clean-build provenance + Wails-runtime isolation in five unit tests**).
- Post-merge CI run
  [36874540492](https://github.com/mrAibo/tvadb-hub/actions/runs/36874540492)
  passed all six repository jobs: `frontend-check`, `go-test`,
  `windows-internal-tests`, `windows-build`, `linux-build` and
  `macos-build`.
- The Windows raw-byte witness checked 34 tracked generated/toolchain-sensitive
  paths and reported `mismatched=0`.
- PR #60 fixed the previously unproven dirty-build provenance problem by
  recording the raw tracked-status snapshot, canonicalising generated inputs and
  isolating five frontend unit tests from the Wails runtime import-time timer.
- P1/P2/P3 engineering work through PR #60 is merged. Do not re-implement the
  completed Logcat lifecycle/batching, transfers, Safe Tuning, device-target
  safety, Launcher Wizard, scrcpy lifecycle/presets or destructive Fastboot
  consent packages.

## Release-readiness hardening after PR #60

This follow-up intentionally changes release/distribution policy and
documentation, not application behaviour:

- the multi-platform distribution workflow is manual-only and no longer pushes
  generated binaries back into the historical `fix/base-ui-menu-group-error`
  branch;
- distribution jobs use read-only repository contents permission;
- publishing a Windows GitHub Release is version-bound to the repository Actions
  variable `PHYSICAL_DEVICE_VALIDATED_VERSION`;
- manual publication additionally requires the explicit
  `physical_device_validated=true` workflow input;
- a `v*` tag can no longer bypass the physical-device gate merely by being
  pushed.

## Remaining release gates

1. **Physical Android/TV validation — issue #19.** Real firmware/OEM behaviour,
   Wireless ADB pairing/reconnect and the intended Windows package workflows are
   not proven until the checklist passes on hardware.
2. **Repository administration — issue #62.** At the PR #60 checkpoint,
   `main` was not protected and repository rulesets were empty. The connected
   GitHub App does not expose branch-protection writes, so required-check
   enforcement remains an administrator action tracked explicitly in
   [issue #62](https://github.com/mrAibo/tvadb-hub/issues/62).
3. **Publication/signing.** No GitHub Release, tag deployment or notarized macOS
   distribution is claimed by this status file.
4. **Evidence-only ADB smart-socket adoption.** Production discovery remains on
   official ADB CLI until repeatable same-endpoint measurements across supported
   hosts/transports justify a change.

## Next product work after release readiness

The next contained product package is **Logcat 2.0 phase 2**: app/PID filtering
and pinned events. The old `feat/logcat-app-pid-pins` branch is only a partial
backend experiment and is far behind current `main`; reuse ideas selectively
from a fresh branch rather than merging it directly.

The wider backlog remains in [docs/ROADMAP.md](docs/ROADMAP.md): file preview,
folder sync/conflict policy, transfer history/resume, APK backup/restore,
Permission/AppOps inspection, device report export, device-side recording and
later automation/distribution work.

## Continuation rule

Before every new package, synchronize current `main`, inspect commits newer
than this checkpoint and preserve valid work. Use a fresh branch, run the full
CI suite, merge only green and verify the post-merge `main` run. Record each
material transition in issue #43.
