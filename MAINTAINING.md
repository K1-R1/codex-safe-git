# Maintaining

This file records maintainer-only workflow notes for preparing `codex-safe-git` changes before a
future public release.

## Local Release Candidate Checks

Before promoting a build:

```sh
scripts/verify.sh
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

Do not tag or publish a release during pre-release preparation work.

## Public Release Prep Still Deferred

Use [docs/open-source-release-checklist.md](docs/open-source-release-checklist.md) as the release
gate before changing repository visibility or accepting public support commitments.

Before public release, decide:

- repository visibility and support expectations
- public vulnerability reporting channel
- release tagging and signing policy
- whether any package-manager distribution is worth supporting
- public CI status badge targets and repository settings

Homebrew, package registries, and external release services are intentionally out of scope for the
current pre-release preparation.
