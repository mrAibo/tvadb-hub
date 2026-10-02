# Releasing DroidSphere for Windows

DroidSphere uses GitHub Actions to build the Windows portable executable and the per-user NSIS installer.

## Release checklist

1. Make sure `main` is green in PR CI.
2. Choose a stable three-part version such as `0.1.0`.
3. Set the same version in `Makefile` (`VERSION := ...`), `build/config.yml` and `internal/core/version.go`.
4. If build assets or product metadata changed, refresh the Wails build assets and review the resulting diff before merging.
5. Run **Windows Release** manually from `main` with **Publish = false** to build a release candidate. This path requires no gate variables.
6. Complete the physical-device validation in `docs/PHYSICAL_DEVICE_VALIDATION.md`, retain the JSON report/test notes plus the artifact hashes (`SHA256SUMS.txt` and `BUILD_PROVENANCE-windows-release.json`) outside the public repository if they contain device-specific information, and record the exact candidate commit SHA.
7. After the validation succeeds, set the repository Actions variable `PHYSICAL_DEVICE_VALIDATED_VERSION` to the exact project version and `PHYSICAL_DEVICE_VALIDATED_SHA` to the full Git commit SHA of the validated candidate (currently 40 hex characters on GitHub). Together they are the durable publication gate for both manual publishing and `v*` tag-triggered publishing.
8. For a manual publication, run **Windows Release** again with **Publish = true** and **Physical device validated = true**. A tag-triggered publication is also rejected unless both variables match the tagged version and the checked-out commit.
9. Verify that the workflow produced the versioned portable EXE, the versioned NSIS installer, `SHA256SUMS.txt` and the matching `BUILD_PROVENANCE-windows-release.json`.
10. Install the NSIS build on a clean Windows account and verify first-run setup, Wireless ADB discovery/pairing, reconnect, APK install, screenshot, scrcpy, file transfer, shell and Logcat.
11. Verify uninstall removes the application and Start/Desktop shortcuts.
12. Confirm the GitHub Release contains the exact files produced by the workflow.

## Publication gate: version + validated source SHA

| Repository Actions variable | Required value |
| --- | --- |
| `PHYSICAL_DEVICE_VALIDATED_VERSION` | the exact project version, for example `0.1.0` |
| `PHYSICAL_DEVICE_VALIDATED_SHA` | the full Git commit SHA of the validated candidate (currently 40 hex characters on GitHub; 64 for SHA-256 Git object IDs) |

- The gate applies to **both** publication paths: a manual run with `publish=true`, and a push of a `v*` tag.
- Manual publication additionally requires the `physical_device_validated=true` workflow input; the tag path takes no input and relies on the variables.
- Missing, malformed (not a full 40- or 64-character Git object ID) or mismatched values fail the run before anything is built or published.
- Building an unpublished release candidate does **not** require either variable.
- The version part stops a validation for `0.1.0` from authorizing `0.1.1` automatically. The SHA part stops a validation from authorizing a **different source commit of the same version** (for example a re-pushed or re-tagged branch).
- The SHA gate authorizes the **source commit**. It is *not* proof that a rebuild produces bit-identical artifacts. Publication uses the artifacts built in that same run, the validated candidate's retained hashes are the provenance record, and a rebuild is never described as the physically tested artifact.
- The logic lives in `scripts/new-release-authorization-helper/Resolve-ReleaseAuthorization.ps1` and is covered by unit-like tests in `scripts/new-release-authorization-helper/tests/ReleaseAuthorization.Tests.ps1`. Run them with `pwsh -NoProfile -File scripts/new-release-authorization-helper/tests/ReleaseAuthorization.Tests.ps1`; they never publish, tag, dispatch a workflow or contact the network.

## Asset replacement policy

The workflow never replaces published assets. If the target release already exists, publication fails closed instead of uploading with `--clobber`. Replacing a published release requires a deliberate decision either to delete that release or to publish a new version; assets are created once with `gh release create`.

## Dependency monitoring

- **Dependabot version updates** (`.github/dependabot.yml`) cover `bun` (frontend `bun.lock`), `gomod` and `github-actions`, each with a 7-day `cooldown.default-days` minimum release age. Dependabot supports the current text `bun.lock` from Bun 1.1.39; the repository does not use the legacy binary `bun.lockb`, and the package manager is not changed for the bot's convenience.
- Dependabot has no security-update support for the `bun` ecosystem, so frontend vulnerabilities are gated in CI instead by `bun audit --audit-level=high` in the `frontend-check` job. That step fails the job on high or critical advisories, and also fails if the advisory request itself fails (fail-closed); `--audit-level` was verified against the pinned Bun CLI with `bun audit --help`.
- Reachable Go vulnerabilities remain covered by the existing `govulncheck ./...` step.
- Enabling Dependabot for the repository is a repository setting; the committed configuration only takes effect once Dependabot is enabled.

## Code signing

The workflow supports Authenticode signing when repository secrets `WINDOWS_SIGNING_CERTIFICATE_BASE64` and `WINDOWS_SIGNING_CERTIFICATE_PASSWORD` are configured. `WINDOWS_TIMESTAMP_URL` is an optional repository variable.

When signing secrets are absent, the workflow deliberately produces unsigned assets instead of silently using an unknown certificate. When they are present, the executable is signed first, the installer is rebuilt with that signed payload, then the installer is signed and both signatures are verified.

## Version safety

The release job fails when the Makefile version, Wails build configuration and in-app version do not match. Tag-triggered runs also fail when the tag does not equal the project version prefixed with `v`.

Publication additionally fails unless `PHYSICAL_DEVICE_VALIDATED_VERSION` equals the exact project version and `PHYSICAL_DEVICE_VALIDATED_SHA` equals the exact checked-out `github.sha`. This prevents a tag push from bypassing the physical-device release gate and prevents one validated commit from authorizing another commit of the same version.

## Verification status

The workflow changes described here have not been executed in remote CI. Publication, distribution and dependency-bot behaviour still require an Actions run and are pending, not verified.
