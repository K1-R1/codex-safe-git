# Security Policy

`codex-safe-git` is a local MCP server for safe Git operations. Security reports are especially
important when they affect repository allowlisting, path validation, secret redaction, audit logging,
or mutation boundaries.

## Supported Versions

Security fixes target the latest public release and the current `main` branch until a broader
support policy is published.

## Reporting

Use GitHub private vulnerability reporting for suspected vulnerabilities when available:

<https://github.com/K1-R1/codex-safe-git/security/advisories/new>

Do not include real secrets, tokens, private keys, or credential material in reports. Use synthetic
examples and disposable repositories whenever possible.

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
