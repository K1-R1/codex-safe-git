# Codex Safe Git

`codex-safe-git` is the canonical Go implementation of the local safe Git MCP server for Codex App
and Codex CLI. The earlier Python prototype has been removed after parity, App, and CLI validation.

## Tool Surface

The Go server must expose exactly:

- `git_status(repo_path)`
- `git_diff_summary(repo_path)`
- `commit_files(repo_path, files[], message, body?)`
- `ensure_commit_branch(repo_path, branch_name)`
- `create_commit_branch(repo_path, branch_name)`
- `merge_branch(repo_path, source_branch, target_branch?)`

No tool accepts arbitrary Git arguments or shell commands.

Safety-critical guarantees include literal exact-file staging, symlink refusal for requested files,
fail-closed audit checks for mutations, metadata-only audit records, and protected branch refusal for
`main`, `master`, repository defaults, and operator-configured production branch names.

## Local Verification

Use the local Go toolchain:

```sh
scripts/verify.sh
```

## First-Time Local Install

After local tests pass:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
```

The installer prints the minimal MCP config for Codex App and Codex CLI. The production server
should point to the stable installed binary, not this source worktree.

See:

- [Operator guide](docs/operator-guide.md)
- [MCP contract](docs/mcp-contract.md)
- [Validation runbook](docs/validation-runbook.md)
- [Future TODO](docs/future-todo.md)
- [Security invariants](docs/invariants.md)
- [Private threat model](docs/threat-model.md)
