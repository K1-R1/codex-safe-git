# Codex Safe Git

`codex-safe-git` is the canonical Go implementation of the local safe Git MCP server for Codex App
and Codex CLI. The earlier Python prototype has been removed after parity, App, and CLI validation.

## Tool Surface

The Go server must expose exactly:

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

No tool accepts arbitrary Git arguments or shell commands.

Safety-critical guarantees include literal exact-file staging, strict exact-path syntax validation,
symlink refusal for requested files, bounded streaming secret scans before staging, post-stage secret
rescans, commit-message secret scanning, fail-closed audit checks for mutations, metadata-only audit
records, trusted Git executable resolution, hard-bounded Git stdout/stderr capture, bounded stdio
request handling, and protected branch refusal for `main`, `master`, repository defaults, and
operator-configured production branch names. Worktree and checkout tools are local-only, refuse
protected target branches, avoid remotes, and require clean state before mutating filesystem or
branch checkout state. Read-only history, ref, path, submodule, integrity, reflog, and self-check
tools return bounded structured summaries without patch text, blob contents, raw object dumps, audit
writes, repair, fetch, clone, expiry, deletion, or remote mutation.

## Local Verification

Use the local Go 1.26+ toolchain:

```sh
scripts/verify.sh
```

This runs formatting, vet, normal tests, coverage-reporting tests, race tests, installer checks,
checksum verification, and a direct stdio MCP smoke test.

## First-Time Local Install

After local tests pass:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

The installer prints the minimal MCP config for Codex App and Codex CLI, writes a SHA-256 checksum
beside the installed binary, and verifies that checksum. The production server should point to the
stable installed binary, not this source worktree.

See:

- [Operator guide](docs/operator-guide.md)
- [MCP contract](docs/mcp-contract.md)
- [Validation runbook](docs/validation-runbook.md)
- [Team onboarding](docs/team-onboarding.md)
- [Audit policy](docs/audit-policy.md)
- [Private binary distribution](docs/private-binary-distribution.md)
- [Future TODO](docs/future-todo.md)
- [Security invariants](docs/invariants.md)
- [Private threat model](docs/threat-model.md)
