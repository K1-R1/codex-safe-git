# Security Policy

`codex-safe-git` is a local MCP server for safe Git operations. Security reports are especially
important when they affect repository allowlisting, path validation, secret redaction, audit logging,
or mutation boundaries.

## Supported Versions

Until the first public release, security fixes target the current `main` branch.

## Reporting

Before public release, report suspected vulnerabilities directly to the repository owner through the
same private channel used for project access. Do not include real secrets, tokens, private keys, or
credential material in reports. Use synthetic examples and disposable repositories whenever possible.

After public release, this file should be updated with the public reporting channel and any enabled
GitHub private vulnerability reporting process. The required public-release steps are tracked in
[docs/open-source-release-checklist.md](docs/open-source-release-checklist.md).

## Scope

In scope:

- bypasses of repo allowlisting or worktree-root validation
- unsafe path traversal, symlink, pathspec, or nested-repo handling
- protected branch mutation bypasses
- unbounded or secret-bearing tool output
- shell injection or unsafe Git subprocess composition
- audit-log failure cases that allow unaudited mutation

Out of scope:

- social engineering
- denial of service requiring local account compromise
- reports that require real credential disclosure
- issues in Git, Codex, or the operating system outside this repository's control
