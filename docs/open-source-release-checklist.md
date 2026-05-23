# Open Source Release Checklist

This checklist separates local repository readiness from external actions that require maintainer
intent, repository settings, credentials, or public support commitments.

## Local Release Candidate

- Run `scripts/verify.sh` on a clean branch.
- Run `scripts/smoke-public-install.sh "$(git rev-parse origin/main)"` after the release candidate
  is merged to `main`.
- Run `scripts/install-local.sh --dry-run`.
- Run `scripts/install-local.sh`.
- Run `scripts/install-local.sh --verify-install`.
- Run an installed MCP `self_check` and confirm the server version, 21-tool surface, checksum
  status, audit-log status, and redacted allowlist metadata.
- Confirm `go.mod` and GitHub Actions workflows use the latest stable Go/toolchain and supported
  action major versions from official sources.
- Confirm `codex-safe-git-mcp --print-config` prints a usable config from an installed binary.
- Review `README.md`, `SECURITY.md`, `CONTRIBUTING.md`, `MAINTAINING.md`, and
  `docs/mcp-contract.md` for stale private-project wording.
- Confirm generated artefacts, local binaries, audit logs, and personal paths are not tracked.

## GitHub Repository Settings

- Set the repository description, website, topics, and licence metadata.
- Enable branch protection or repository rulesets for `main`.
- Require CI before merge. After checks have appeared at least once, require `verify
  (ubuntu-latest)`, `verify (macos-latest)`, and `dco / signed-off`.
- Require pull request review before protected-branch updates.
- Disable force pushes and branch deletion on protected branches.
- Enable the repository setting that requires contributors to sign off on web-based commits.
- Enable Dependabot alerts and security updates.
- Enable CodeQL/code scanning default setup if available for the repository. Do not add a CodeQL
  Actions workflow while default setup is enabled; GitHub rejects advanced-configuration uploads in
  that mode.
- Enable private vulnerability reporting before accepting public security reports.
- Decide whether blank issues, discussions, and wiki pages should be enabled.

## Public Support Commitments

- Replace the pre-release security reporting text with the public reporting channel.
- Decide which versions, branches, or tags receive security fixes.
- Decide whether external contributors should be accepted immediately or after a stabilisation
  period.
- Keep initial distribution to `go install ...@version` plus source-tree install for maintainers.
  Homebrew, registries, and install scripts that fetch from the network remain deferred until
  explicitly approved.

## Release And Provenance

- Decide whether releases are source-only or include binaries.
- Decide whether tags are signed, who signs them, and where signing policy is documented.
- Create a semver tag that matches `internal/mcp/server.go` when publishing an implementation
  release.
- Use [Release process](release-process.md) for release tags, generated release notes, and public
  install smoke tests.
- If binary releases are added, define checksums, provenance, and verification instructions before
  publishing.
- Do not put signing keys, package tokens, or release credentials in the repository or Codex
  prompts.

## Final Pre-Publish Review

- Re-run the local release candidate checks.
- Review the full diff from the last private baseline.
- Confirm issue templates and pull request templates match the public support model.
- Confirm no private paths, archived-project references, personal setup notes, or audit logs remain.
- Publish only after the maintainer accepts the support, security, and release-process commitments.
