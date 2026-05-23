# Repository Maintenance Checklist

Use this checklist when preparing release candidates, reviewing repository settings, or changing the
public support model.

## Local Change Readiness

- Work from a non-protected branch with a clean worktree.
- Run `scripts/verify.sh`.
- Run `sh scripts/check-dco.sh main..HEAD` before opening or updating a pull request.
- Run `scripts/install-local.sh --dry-run`, `scripts/install-local.sh`, and
  `scripts/install-local.sh --verify-install` when installer, MCP config, or release behaviour
  changes.
- Run an installed MCP `self_check` when changing install, checksum, allowlist, or audit-log
  behaviour. Confirm the server version, 21-tool surface, checksum status, audit-log status, and
  redacted allowlist metadata.
- Review generated artefacts, local binaries, audit logs, and personal paths before committing.

## Repository Settings

- Keep `main` protected through branch rules or repository rulesets.
- Require pull request review before protected-branch updates.
- Require `verify (ubuntu-latest)`, `verify (macos-latest)`, and `signed-off` before merge. GitHub
  may display the DCO job as `dco / signed-off`.
- Keep `.github/CODEOWNERS` aligned with code-owner review requirements.
- Disable force pushes and branch deletion on protected branches.
- Require sign-off for web-based commits.
- Keep Dependabot alerts, Dependabot security updates, secret scanning, and push protection enabled.
- Use GitHub CodeQL default setup if available. Do not add a CodeQL Actions workflow while default
  setup is enabled.
- Keep private vulnerability reporting available before asking users to send private security
  reports.

## Release Readiness

- Confirm `go.mod`, GitHub Actions workflows, and action major versions still track supported stable
  releases from official sources.
- Confirm the MCP version in `internal/mcp/server.go` matches the intended semver tag for
  implementation releases.
- Confirm `CHANGELOG.md`, [Release process](release-process.md), and
  [Distribution](distribution.md) match the release shape.
- Smoke-test the remote commit before tagging:

```sh
scripts/smoke-public-install.sh "$(git rev-parse origin/main)"
```

- Smoke-test the published tag after release:

```sh
scripts/smoke-public-install.sh vX.Y.Z
```

## Deferred Decisions

- Binary release assets require checksum, signing, provenance, and support policy decisions first.
- Package-manager distribution, `curl | sh` installers, automated publishing, and external release
  services require an explicit maintainer decision first.
- Signing keys, package tokens, and release credentials must not be stored in the repository or
  pasted into Codex prompts.
