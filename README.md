# codex-safe-git

[![CI](https://github.com/K1-R1/codex-safe-git/actions/workflows/verify.yml/badge.svg)](https://github.com/K1-R1/codex-safe-git/actions/workflows/verify.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

`codex-safe-git` is a local MCP server that gives Codex a narrow, auditable Git tool
surface. It is designed for safe local branch, worktree, status, review, and exact-file
commit workflows without exposing arbitrary Git commands or shell execution.

The server is local-first: no telemetry, no remotes, no network Git operations, no binary release
assets, and no package-manager distribution. Public installation uses Go's command installer.

## Tool Surface

The Go server exposes exactly these MCP tools:

- `git_status(repo_path)`
- `git_diff_summary(repo_path)`
- `list_local_branches(repo_path)`
- `compare_refs(repo_path, base_ref, target_ref)`
- `commit_log_summary(repo_path, ref?, limit?)`
- `show_commit_summary(repo_path, commit_ref)`
- `list_local_refs(repo_path)`
- `merge_base(repo_path, left_ref, right_ref)`
- `changed_files_between_refs(repo_path, base_ref, target_ref)`
- `path_status(repo_path, paths[], include_ignore_source?)`
- `submodule_summary(repo_path)`
- `repository_integrity_check(repo_path)`
- `reflog_summary(repo_path, ref?, limit?)`
- `self_check(repo_path)`
- `commit_files(repo_path, files[], message, body?)`
- `ensure_commit_branch(repo_path, branch_name)`
- `create_commit_branch(repo_path, branch_name)`
- `merge_branch(repo_path, source_branch, target_branch?)`
- `list_worktrees(repo_path)`
- `create_worktree(repo_path, worktree_path, branch_name, base_branch?)`
- `safe_checkout(repo_path, branch_name)`

No tool accepts arbitrary Git arguments, shell commands, remotes, force options, push,
pull, fetch, reset, clean, rebase, tag mutation, package publishing, deployment, or PR
creation.

## Safety Model

Safety-critical guarantees include:

- explicit repo allowlisting and exact Git worktree-root validation
- literal exact-file staging with strict path syntax checks
- symlink, traversal, secret-bearing path, and ambiguous state refusal
- bounded streaming secret scans before staging and staged-diff rescans before commit
- protected branch refusal for `main`, `master`, repository defaults, and configured names
- metadata-only audit records for mutating tools
- bounded, structured, deterministic outputs for read-only tools
- no patch text, blob contents, raw object dumps, or hidden local paths by default

See [MCP contract](docs/mcp-contract.md), [security invariants](docs/invariants.md), and
[threat model](docs/threat-model.md) for the durable guarantees.

## Install Locally

Install the command directly:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@latest
mkdir -p "$HOME/.codex/worktrees" "$HOME/.codex/log"
codex-safe-git-mcp --print-config
```

For repeatable installs, use a release tag such as `@v0.4.2` instead of `@latest`.

If `$(go env GOPATH)/bin` is not on `PATH`, run the installed binary by absolute path. See
[Distribution](docs/distribution.md) for the full public and source-tree install paths.

For source-tree development, run verification first:

```sh
scripts/verify.sh
```

Then install the stable local binary and print the Codex MCP config:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

The installer builds the Go binary, installs it under `~/.codex/tools/codex-safe-git-go`,
writes a SHA-256 sidecar checksum, and prints a TOML config block for Codex App and Codex
CLI.

## Development

Requirements:

- Go 1.26.3, or a compatible Go installation with automatic toolchain downloads enabled
- Git
- `shasum` or `sha256sum`

Useful commands:

```sh
go test ./...
go test -race ./...
scripts/smoke-stdio.sh
scripts/verify.sh
```

## Docs

- [Getting started](docs/getting-started.md)
- [Operator guide](docs/operator-guide.md)
- [Codex config example](docs/codex-config-example.toml)
- [MCP contract](docs/mcp-contract.md)
- [Validation runbook](docs/validation-runbook.md)
- [Audit policy](docs/audit-policy.md)
- [Distribution](docs/distribution.md)
- [Release process](docs/release-process.md)
- [Local install integrity](docs/local-install-integrity.md)
- [Developer Certificate of Origin](DCO.md)
- [Security invariants](docs/invariants.md)
- [Threat model](docs/threat-model.md)
- [Repository maintenance checklist](docs/repository-maintenance.md)

## Release Status

This repository ships public source releases through Go module tags. The supported distribution path
is `go install`; Homebrew/package-manager distribution, binary release assets, automated publishing,
and external release services remain deferred until there is an explicit maintainer decision.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and contribution guidelines.

## Security

See [SECURITY.md](SECURITY.md) for supported security reporting expectations.

## License

MIT - see [LICENSE](LICENSE).
