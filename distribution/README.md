# DroidSphere distribution

DroidSphere release artifacts are produced by GitHub Actions from the repository source tree.
Compiled binaries are intentionally **not committed** to the source branch.

## Windows

Canonical release names:

- `DroidSphere-<version>-windows-amd64.exe` — portable executable.
- `DroidSphere-<version>-windows-amd64-installer.exe` — per-user NSIS installer.
- `SHA256SUMS.txt` — SHA-256 checksums.

Use the PR/CI artifacts for review builds and GitHub Releases for published builds.

## Linux and macOS

PR CI performs native Linux and macOS desktop builds. Public Linux packages and signed/notarized
macOS distribution remain separate release-engineering milestones; see `docs/PORTABILITY.md`.
