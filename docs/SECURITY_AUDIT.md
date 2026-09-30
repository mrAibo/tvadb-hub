# Security audit follow-up

Date: 2026-09-30  
Scope: DroidSphere desktop application, Go backend, Wails IPC and React frontend.

This document records the repository-specific review performed after an external
security/code audit. Generic recommendations were checked against the actual
implementation before changes were made.

## 1. Command injection and subprocess handling

**Status: hardened.**

DroidSphere does not launch ADB/Fastboot/scrcpy through `sh -c`,
`cmd.exe /c` or an equivalent shell wrapper. The central executor passes the
program and arguments separately to Go's process API.

Additional hardening in 0.1.1:

- Android package names are restricted to `[A-Za-z0-9._]+` before package
  operations reach ADB.
- Wireless ADB endpoints are parsed as real `host:port` values and ports are
  restricted to 1..65535.
- Wireless pairing codes must contain exactly six digits.
- CR/LF/control-style endpoint input is rejected.

The interactive Terminal is intentionally different: it is an explicit
developer shell. Commands typed there are expected to execute on the selected
ADB/Fastboot target. That capability is user-requested functionality rather
than an implicit shell interpolation path.

## 2. Process lifecycle, zombies and pipe handling

**Status: hardened.**

Before this follow-up, child process groups were created but several cancellation
paths killed only the immediate process.

0.1.1 adds a common process-tree termination path:

- Unix children run in their own process group and cancellation kills that
  group.
- Windows uses `taskkill /T /F` for the launched PID, with direct process
  termination as a fallback.
- normal executor commands, Logcat, scrcpy sessions, scrcpy recording and
  PTY cancellation use the hardened lifecycle.
- Terminal sessions now own a cancellable context. Closing a terminal session
  therefore cancels a command that is currently running.

The central streaming executor already drained stdout and stderr concurrently,
which protects it from the classic full-pipe deadlock.

**Remaining improvement:** native Windows Job Objects would be stronger than
`taskkill /T` for adversarial child-process trees. The current implementation
is appropriate for the trusted ADB/Fastboot/scrcpy toolchain but this can be
revisited if DroidSphere starts arbitrary third-party executables.

## 3. File manager path traversal and filesystem safety

**Status: existing protections verified.**

Managed ZIP/TAR extraction already:

- rejects absolute and parent-traversal archive paths;
- verifies the final extraction target stays inside the destination;
- rejects non-regular ZIP entries;
- accepts only regular files/directories from TAR archives, thereby rejecting
  symlink entries;
- creates managed directories with restrictive permissions and executable files
  with `0755`, not `0777`.

Android file mutations normalize remote paths, reject null/newline characters
and block broad/system locations such as `/`, `/system`, `/vendor`,
`/data` and related protected trees.

Host filesystem symlinks are surfaced as symlinks in the UI. A user-selected
local symlink can still be followed by normal host filesystem APIs during a
transfer. This is considered a local-user action rather than an archive
extraction boundary; a future strict-mode option may refuse symlink transfers.

## 4. Managed toolchain integrity

**Status: hardened.**

Managed downloads are staged inside DroidSphere's per-user application data
directory rather than executed from a shared temporary directory.

0.1.1 makes archive validation fail closed:

- Android SDK Platform Tools 37.0.1 are pinned to the digest and exact byte size
  published in Google's SDK repository metadata for Linux, macOS and Windows.
  Google's repository metadata currently publishes SHA-1 for these archives,
  so DroidSphere verifies that upstream digest plus the exact upstream size.
- scrcpy 4.1 assets are pinned to the SHA-256 values published by the scrcpy
  project.
- managed scrcpy no longer constructs a non-existent Linux ARM64 v4.1 download;
  unsupported architectures return an explicit error instead.

Reference:
- Android SDK repository metadata:
  https://dl.google.com/android/repository/repository2-3.xml
- scrcpy release verification:
  https://github.com/Genymobile/scrcpy/blob/master/doc/verify-release.md

## 5. Logcat memory and IPC pressure

**Status: bounded and backend-batched.**

Protections that already existed:

- frontend log entries are queued and flushed every 100 ms;
- the React log view is virtualized;
- stored log entries are kept in a bounded rolling buffer.

0.1.1 aligns the defaults and enforces a hard ceiling:

- minimum: 1,000 entries;
- default: 5,000 entries;
- maximum: 50,000 entries;
- oversized values from legacy settings are clamped;
- IPC preference updates outside the supported range are rejected by Go.

The Go backend now batches parsed Logcat entries before Wails IPC. Each stream
flushes approximately every 75 ms or when 128 entries are queued, whichever
comes first. Ordering is preserved, memory remains bounded, and the final
partial batch is flushed before the stopped/error status when a stream exits or
is cancelled. The frontend keeps its existing 100 ms queue, rolling buffer and
virtualized rendering.

## 6. Safe Tuning and reversibility

**Status: existing backend protections verified and snapshot durability
hardened.**

Safe Tuning does not rely on frontend disabling alone. The Go service:

- resolves the selected package against the active profile;
- rejects keep-list/protected packages;
- rejects `dangerous` and `blocked` packages;
- requires explicit acknowledgement for `caution` packages;
- restricts reversible user-0 uninstall to eligible system apps.

Snapshots are written before changes. 0.1.1 strengthens atomic persistence by
syncing the temporary snapshot/config file before rename and performing a
best-effort directory sync after rename. Snapshot files use `0600`.

## 7. IPC and WebView safety

**Status: existing protections verified, input validation expanded.**

The frontend does not use `dangerouslySetInnerHTML` for shell, package or
Logcat output; React text rendering escapes untrusted strings.

Mutation-sensitive backend methods validate their inputs. This follow-up adds
strict package-name and Wireless ADB endpoint validation at the Go boundary.

A router-level error page is also provided so an unexpected UI exception no
longer exposes React Router's developer-oriented stack page to end users.

## Continuous checks

PR CI now runs:

- frontend lint, typecheck and tests;
- normal Go tests;
- `go vet ./...`;
- `govulncheck ./...`;
- Go race detection on concurrency-heavy backend packages;
- Windows, Linux and macOS desktop builds.

Fuzz seed tests are present for mDNS and Logcat parsers. Longer fuzz campaigns
can be run locally or in a future scheduled workflow, for example:

```bash
go test ./internal/device -fuzz=FuzzParseMDNSServices -fuzztime=60s
go test ./internal/shell -fuzz=FuzzParseLogcatEntry -fuzztime=60s
```

## Base UI runtime hotfix

The runtime error reported as Base UI error #31 was traced to
`Menu.GroupLabel` elements rendered directly inside the Current+ dropdown.
Base UI requires those labels to be descendants of `Menu.Group` or
`Menu.RadioGroup`.

Both dropdown sections are now grouped correctly and a regression test opens
the menu during frontend tests. DroidSphere also supplies a router
`errorElement` so future route-level UI exceptions show an application-owned
recovery screen instead of React Router's developer error page.
