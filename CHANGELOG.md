# Changelog

All notable changes to `codex-safe-git` are recorded here.

This project uses explicit version updates in the MCP server metadata. Public release tags are
deferred until the project is published.

## Unreleased - Repository Preparation

- Prepared repository metadata, CI, licence, security, contribution, and maintainer docs.
- Added durable Codex repo instructions, GitHub issue and pull request templates, Dependabot
  configuration, CodeQL scanning, and an open-source release checklist.
- Preserved the 21-tool MCP surface and existing safety contract.
- Kept local install as the only supported distribution path.

## 0.4.2 - Completion Hardening

- Added streaming bounded secret scans for new files.
- Tightened exact commit path validation.
- Hardened ref verification, submodule path parsing, stdio frame bounds, and installer TOML escaping.
- Added stronger MCP schema parity and regression coverage.

## 0.4.1 - Safe Git Capability Completion

- Added bounded read-only branch, ref, history, path, submodule, integrity, reflog, and self-check
  tools.
- Completed the approved local Git capability list without adding rejected remote or destructive
  features.
