# Maintaining

This file records maintainer-only workflow notes for preparing `codex-safe-git` changes and public
source releases.

## Local Release Candidate Checks

Before promoting a build:

```sh
scripts/verify.sh
scripts/smoke-public-install.sh "$(git rev-parse origin/main)"
scripts/install-local.sh --dry-run
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

Then run a direct installed MCP `self_check` from Codex and confirm:

- server version is expected
- tool count is 21
- checksum status is `matched`
- audit log is configured
- allowed roots are redacted or hashed

## Versioning

Update `internal/mcp/server.go` when the MCP implementation version changes. Update the changelog
and docs when the public contract, safety guarantees, or operator workflow changes.

Release tags should be semantic versions such as `v0.4.2`. See
[docs/release-process.md](docs/release-process.md) for the tag, release-note, and public install
smoke-test workflow.

## Deferred Distribution Work

Homebrew, package registries, `curl | sh` installers, binary release assets, and external release
services are intentionally out of scope until there is a separate maintainer decision covering
signing, checksums, provenance, and support expectations.
