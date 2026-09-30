# Validation and build provenance

The frontend typecheck force-checks referenced application/tooling projects and
then explicitly checks test roots under a strict config. The old root
`tsc --noEmit` with `files: []` did not check referenced source. Development
builds now use project mode too. No blanket compiler suppressions were added.

build/toolchain.json records Go 1.26.7, Bun 1.4.2, Wails v3.0.0-beta.20 and
NSIS 3.12.0. CI/release/distribution workflows pin these versions. Wails CLI,
Go module and frontend runtime agree; frozen dependencies are retained.
check-toolchain.ts rejects drift or missing pins. Windows uses shared NSIS
setup with installed-version verification and upgrade/downgrade as needed.

After building/signing, BUILD_PROVENANCE-<platform>.json records checkout SHA,
dirty status, workflow run/attempt, pins, actual Go/Bun/Wails versions and
SHA-256 artifact digests. Workflow SHA must match checkout SHA. The manifest is
uploaded with binaries and included in release/distribution assets. This code
change itself does not publish a release.

This is a provenance record, not a signed attestation or a claim of byte-for-byte
reproducibility. Runner images, OS libraries, signing timestamps and major-tag
Actions are not hermetic. Existing signing/physical validation gates stay intact.

Historical committed distribution packages lack reliable source manifests.
Do not call them current, guess/backfill their source SHA, or overwrite them
as part of documentation updates.

Core compiler/workflow checks passed locally before environment disconnection;
the recovered final tree and new provenance regression tests require fresh CI.
Full Windows NSIS and three-host desktop validation use CI. No physical-device
validation is implied. Live continuation and limitations: issue #43.
