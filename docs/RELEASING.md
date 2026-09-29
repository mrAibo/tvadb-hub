# Releasing TVADB Hub for Windows

TVADB Hub uses a reproducible GitHub Actions release workflow for the Windows portable executable and the per-user NSIS installer.

## Release checklist

1. Make sure `main` is green in PR CI.
2. Choose a stable three-part version such as `0.1.0`.
3. Set the same version in `Makefile` (`VERSION := ...`) and `build/config.yml` (`info.version`).
4. If build assets or product metadata changed, refresh the Wails build assets and review the resulting diff before merging.
5. Run the **Windows Release** workflow manually from `main` first with **Publish = false** to build a release candidate. After physical-TV validation succeeds, run it again with **Publish = true** and **Physical TV validated = true**, or create and push the matching tag (for example `v0.1.0`).
6. Verify the workflow produced the versioned portable EXE, the versioned NSIS installer, and `SHA256SUMS.txt`.
7. Install the NSIS build on a clean Windows account and verify first-run setup, Wireless ADB discovery/pairing, reconnect, APK install, screenshot, Scrcpy, file transfer, shell and Logcat.
8. Verify uninstall removes the application and Start/Desktop shortcuts.
9. Confirm the GitHub Release contains the exact files from the workflow.

## Code signing

The workflow supports Authenticode signing when repository secrets `WINDOWS_SIGNING_CERTIFICATE_BASE64` and `WINDOWS_SIGNING_CERTIFICATE_PASSWORD` are configured. `WINDOWS_TIMESTAMP_URL` is an optional repository variable.

When signing secrets are absent, the workflow deliberately publishes unsigned assets instead of silently using an unknown certificate. When they are present, the executable is signed first, the installer is rebuilt with that signed payload, then the installer is signed and both signatures are verified.

## Version safety

The release job fails when the Makefile version, Wails build configuration, and in-app update-check version do not match. Manual publication also requires an explicit physical-TV validation confirmation. Tag-triggered releases also fail when the tag does not equal the project version prefixed with `v`. This prevents accidental publication of a mis-versioned installer.
