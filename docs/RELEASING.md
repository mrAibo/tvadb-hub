# Releasing DroidSphere for Windows

DroidSphere uses GitHub Actions to build the Windows portable executable and the per-user NSIS installer.

## Release checklist

1. Make sure `main` is green in PR CI.
2. Choose a stable three-part version such as `0.1.0`.
3. Set the same version in `Makefile` (`VERSION := ...`), `build/config.yml` and `internal/core/version.go`.
4. If build assets or product metadata changed, refresh the Wails build assets and review the resulting diff before merging.
5. Run **Windows Release** manually from `main` with **Publish = false** to build a release candidate.
6. Complete the physical-device validation in `docs/PHYSICAL_DEVICE_VALIDATION.md` and retain the JSON report/release notes outside the public repository if they contain device-specific information.
7. After the validation succeeds, set the repository Actions variable `PHYSICAL_DEVICE_VALIDATED_VERSION` to the exact project version, for example `0.1.0`. This is the durable publication gate for both manual publishing and `v*` tag-triggered publishing.
8. For a manual publication, run **Windows Release** again with **Publish = true** and **Physical device validated = true**. A tag-triggered publication is also rejected unless `PHYSICAL_DEVICE_VALIDATED_VERSION` exactly matches the tagged project version.
9. Verify that the workflow produced the versioned portable EXE, the versioned NSIS installer, `SHA256SUMS.txt` and the matching `BUILD_PROVENANCE-windows-release.json`.
10. Install the NSIS build on a clean Windows account and verify first-run setup, Wireless ADB discovery/pairing, reconnect, APK install, screenshot, scrcpy, file transfer, shell and Logcat.
11. Verify uninstall removes the application and Start/Desktop shortcuts.
12. Confirm the GitHub Release contains the exact files produced by the workflow.

## Physical-validation publication gate

The repository variable is version-bound deliberately. A validation for `0.1.0` must not authorize a later `0.1.1` release automatically.

- Building a non-published release candidate does **not** require the variable.
- Manual publication requires both the matching repository variable and the explicit workflow confirmation.
- Tag-triggered publication requires the matching repository variable.
- If physical validation discovers a blocking defect, do not set/update the variable until the repaired candidate has been validated.

## Code signing

The workflow supports Authenticode signing when repository secrets `WINDOWS_SIGNING_CERTIFICATE_BASE64` and `WINDOWS_SIGNING_CERTIFICATE_PASSWORD` are configured. `WINDOWS_TIMESTAMP_URL` is an optional repository variable.

When signing secrets are absent, the workflow deliberately produces unsigned assets instead of silently using an unknown certificate. When they are present, the executable is signed first, the installer is rebuilt with that signed payload, then the installer is signed and both signatures are verified.

## Version safety

The release job fails when the Makefile version, Wails build configuration and in-app version do not match. Tag-triggered runs also fail when the tag does not equal the project version prefixed with `v`.

Publication additionally fails unless `PHYSICAL_DEVICE_VALIDATED_VERSION` equals the exact project version. This prevents a tag push from bypassing the physical-device release gate.
