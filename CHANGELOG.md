# Changelog

All notable changes to `codex-safe-git` are recorded here.

This project uses explicit version updates in the MCP server metadata. Public release tags should
match the MCP server version unless a release contains only documentation or repository setup
changes.

## Unreleased

- Added `CODEOWNERS` to make the active code-owner review rule concrete.
- Tightened public maintenance, DCO, distribution, and release-process documentation after the first
  public release.

## 0.4.2 - Initial Public Release

- Prepared repository metadata, CI, licence, security, contribution, and maintainer docs.
- Added durable Codex repo instructions, GitHub issue and pull request templates, Dependabot
  configuration, CodeQL scanning, and an open-source release checklist.
- Added DCO enforcement and GitHub generated release-note configuration.
- Pinned the Go toolchain directive to the current stable Go patch release.
- Added binary CLI helpers for `--version`, `--help`, and `--print-config` to support `go install`
  distribution.
- Added a public `go install` smoke test for release validation.
- Preserved the 21-tool MCP surface and existing safety contract.
- Planned public distribution through `go install ...@version` while keeping source-tree install for
  maintainers.

### Completion Hardening

- Added streaming bounded secret scans for new files.
- Tightened exact commit path validation.
- Hardened ref verification, submodule path parsing, stdio frame bounds, and installer TOML escaping.
- Added stronger MCP schema parity and regression coverage.

## 0.4.1 - Safe Git Capability Completion

- Added bounded read-only branch, ref, history, path, submodule, integrity, reflog, and self-check
  tools.
- Completed the approved local Git capability list without adding rejected remote or destructive
  features.
