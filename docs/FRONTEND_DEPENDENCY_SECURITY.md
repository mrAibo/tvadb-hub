# Frontend dependency security (2026-10-12)

The `shadcn` code-generation CLI was previously installed as a runtime dependency solely to import `shadcn/tailwind.css`. This pulled unused CLI/server packages (`@modelcontextprotocol/sdk`, `express`, `braces`) into the dependency graph, associated with advisories GHSA-6qxp-vccf-f47h, GHSA-jqcg-44mw-7w3h and GHSA-vfj7-8cjw-p6xm.

Following [official shadcn eject guidance](https://ui.shadcn.com/docs/changelog/2026-05-shadcn-eject), the CLI dependency is removed, and its MIT-licensed stylesheet is preserved in `frontend/src/shadcn-ejected.css` (version 4.21.0; original SHA-256 `BC7D83425702955B4CB67CB14EDE9D603F9D912376D57A2D81D661094D2A782A`). `components.json` is developer scaffolding metadata; running the shadcn CLI interactively now requires an explicit install.

The remaining GHSA-68fv-2mgg-jv7q finding is resolved by bumping the transitive `source-map-js` lock entry from 1.2.1 to 1.2.2, compatible with existing `^1.2.1` dependencies. The integrity is from the npm registry version manifest: `sha512-KGj/8Y43x35aZVDtt+J4mK1hoLGHULMYfSkODJNQjNDC3oW1PqPoxMwo0pLUsWM/UEGzON/NxeHywEfNXNP3Vw==`.

The existing security gate, frozen-lockfile install, lint/typecheck/tests and cross-platform build jobs remain enforced. No backend, device mutation or release workflow is changed.

## Go standard-library security update

The October 12, 2026 CI vulnerability database also flags ten reachable standard-library vulnerabilities in pinned Go 1.26.7. The official Go 1.26.9 release (October 8, 2026) fixes them (`net/http`, `net/textproto` and `crypto/tls`). The Go version is upgraded from 1.26.7 to 1.26.9 consistently in `build/toolchain.json`, CI, distribution/release workflows and build documentation; `go.mod` retains its 1.26.0 compatibility minimum. `govulncheck ./...` remains a required CI step.
