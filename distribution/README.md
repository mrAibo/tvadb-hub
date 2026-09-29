# DroidSphere distribution

DroidSphere release artifacts are produced by GitHub Actions from the repository source tree.

## Windows

Canonical release names:

- `DroidSphere-<version>-windows-amd64.exe` — portable executable.
- `DroidSphere-<version>-windows-amd64-installer.exe` — per-user NSIS installer.
- `SHA256SUMS.txt` — SHA-256 checksums.

The binary files currently committed under the historical `TVADB-Hub-*` names are pre-rebrand snapshots.
New builds and releases use the DroidSphere naming scheme.

## Linux and macOS

PR CI performs native Linux and macOS desktop builds. Public Linux packages and signed/notarized
macOS distribution remain separate release-engineering milestones; see `docs/PORTABILITY.md`.
