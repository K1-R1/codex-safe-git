# Future TODO

## Current State

Codex Safe Git is the canonical Go implementation. The earlier Python prototype has been removed
after local, direct stdio, Codex App, and Codex CLI validation.

The active local MCP config should point to:

```text
~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp
```

The MCP surface is exactly:

- `git_status`
- `git_diff_summary`
- `list_local_branches`
- `compare_refs`
- `commit_log_summary`
- `show_commit_summary`
- `list_local_refs`
- `merge_base`
- `changed_files_between_refs`
- `path_status`
- `submodule_summary`
- `repository_integrity_check`
- `reflog_summary`
- `self_check`
- `commit_files`
- `ensure_commit_branch`
- `create_commit_branch`
- `merge_branch`
- `list_worktrees`
- `create_worktree`
- `safe_checkout`

## Completed Project Readiness

- CI coverage is defined for formatting, vet, tests, race tests, installer checks, direct stdio MCP
  smoke tests, and safety regression cases.
- `scripts/verify.sh` is the one-command local verification path.
- The deferred local worktree tools have been implemented with explicit safety constraints and test
  coverage.
- The audit retention, review, and manual rotation policy is documented in
  [Audit policy](audit-policy.md).
- Local onboarding is documented in [Getting started](getting-started.md).
- Local install integrity uses installer-generated SHA-256 checksums and `--verify-install`.
- Public command distribution uses `go install ...@version`; source-tree install remains available
  for maintainers.
- Public release signing remains deferred without requiring Codex to access signing keys.
- Protected-branch landing remains outside this MCP and is documented as a human-reviewed workflow.
- The approved local branch, ref comparison, commit summary, path status, submodule summary,
  integrity check, reflog summary, and self-check tools are implemented as bounded read-only
  summaries.
- Completion hardening is implemented: streaming bounded secret scans for new files, stricter
  commit path syntax, `rev-parse --end-of-options` verification, path-aware submodule parsing,
  stronger MCP schema parity tests, bounded stdio request handling, installer TOML escaping, and
  explicit non-mutating `self_check` audit-log writability semantics.

Automated audit deletion or rotation is intentionally not implemented. That is now a documented
non-goal because audit movement should remain operator-owned and visible.

## Capability Record

### Implemented Enhancements

These features were selected because they are local, bounded, mostly read-only, and compatible with
the existing fail-closed safety model:

- `list_local_branches`: list local branches with current/protected/checked-out status and HEAD
  hashes.
- `compare_refs`: summarise merge base, ahead/behind counts, and changed-file counts for two local
  refs without returning patch text.
- `commit_log_summary`: return recent bounded commit metadata for a local branch, such as hash,
  subject, author date, and changed-file count.
- `show_commit_summary`: return bounded metadata and changed-file names for one local commit without
  returning patch text.
- `list_local_refs`: list bounded local refs with type, target hash, and protected/visible status.
- `merge_base`: report the merge base for two local refs.
- `changed_files_between_refs`: return bounded, redacted changed-file names and counts between two
  local refs.
- `path_status`: report tracked, ignored, untracked, deleted, modified, unmerged, sparse, and
  skip-worktree state for an explicit bounded path list, with optional redacted ignore-source
  metadata.
- `submodule_summary`: report configured and working-tree submodules, expected and current commits,
  dirty or uninitialised state, and mutation-refusal reasons without cloning, fetching, or updating.
- `repository_integrity_check`: run a bounded read-only integrity diagnostic and return severity
  counts without repair, `lost-found` writes, or noisy object dumps.
- `reflog_summary`: report recent bounded HEAD or branch movements for local recovery/audit, with
  redacted summaries and no expire, delete, or drop behaviour.
- `self_check`: report the MCP version, tool surface, binary/checksum status, audit-log writability,
  and redacted/hashed allowlist configuration.

### Capability Verdicts

This is the current decision record for the safe Git MCP surface. `Implemented` rows were previously
approved `Add` decisions and are now part of the active tool contract.

| Capability | Verdict | Reasoning |
| --- | --- | --- |
| `git_status` | Keep | Core read-only orientation tool; bounded, redacted, deterministic status is essential before any mutation. |
| `git_diff_summary` | Keep | Gives file-level change counts without patch text, which is the right context-efficient default. |
| `commit_files` | Keep | Exact-file commits with clean-state checks, secret scans, and audit logging are the safest local mutation. |
| `ensure_commit_branch` | Keep | Safely attaches detached work to a non-protected local branch before committing. |
| `create_commit_branch` | Keep | Provides narrow branch creation or switching without touching protected/default branches. |
| `merge_branch` | Keep | Fast-forward-only local branch integration is predictable and reviewable. |
| `list_worktrees` | Keep | Worktree visibility prevents branch/path confusion and avoids mutating the wrong checkout. |
| `create_worktree` | Keep | Safe linked worktrees support long or parallel work while preserving the current checkout. |
| `safe_checkout` | Keep | Clean-worktree, non-protected local checkout is useful and bounded. |
| `list_local_branches` | Implemented | High-value branch orientation; read-only, bounded, and marked with protected/checked-out status. |
| `compare_refs` | Implemented | Ahead/behind, merge-base, and changed-file counts help agents reason without full diffs. |
| `commit_log_summary` | Implemented | Bounded commit metadata supports review and planning without dumping patches. |
| `show_commit_summary` | Implemented | One-commit metadata and changed-file names are useful for audit and review with low context cost. |
| `list_local_refs` | Implemented | A bounded local ref inventory helps avoid ambiguous refs and protected-branch mistakes. |
| `merge_base` | Implemented | A small, read-only primitive that supports safer compare and branch reasoning. |
| `changed_files_between_refs` | Implemented | File names and counts between refs are useful for review while avoiding patch text by default. |
| `path_status` | Implemented | Exact path state, ignored status, sparse state, and unmerged state are high-value for safe exact-file commits. |
| `submodule_summary` | Implemented | Read-only submodule inventory and dirty/uninitialised state are useful; cloning, fetching, and updating stay out of scope. |
| `repository_integrity_check` | Implemented | Read-only integrity diagnostics are severity-counted, bounded, and never repair or write `lost-found`. |
| `reflog_summary` | Implemented | Bounded, redacted local reflog summaries help with recovery and audit without exposing full local history. |
| `self_check` | Implemented | Version, checksum, tool-surface, allowlist, and audit-log checks make MCP installation health explicit. |
| `blame` | No | Line-level ownership output is context-heavy and can expose personal metadata; use shell Git only for explicit manual investigation. |
| `grep` | No | Codex already has `rg`; a Git grep MCP would mostly add content and secret exposure risk. |
| restore uncommitted files | No | Restoring can overwrite or remove working-tree changes, which conflicts with preserving user work. |
| move or rename files | No | Normal file tools plus `commit_files` are sufficient; `git mv` also has index and submodule side effects. |
| remove files | No | File removal is destructive and should remain explicit through normal file tools and exact-file commits. |
| `revert` | No | Although safer than reset, it creates commits and can enter sequencer or conflict states; reconsider only with a new explicit design. |
| `cherry-pick` | No | It mutates index and working tree, can conflict, and can duplicate history in branch-sensitive ways. |
| archive or bundle export | No | Export artefacts can package repository contents or objects and are a poor fit for a safety-first MCP. |
| raw plumbing object reads | Internal only | Commands such as raw `cat-file` can expose arbitrary object contents; use them only behind bounded summary tools. |

## Remaining Work

No implementation, verification, safety, token-efficiency, MCP-contract, or documentation completion
items are currently open. Deferred release work includes future release tags, binary release assets,
release signing, and any package-manager distribution. Use [Open Source Release Checklist](open-source-release-checklist.md)
and [Release Process](release-process.md) for maintainer-owned external steps.

## Permanent Non-Goals

These are not future features:

- arbitrary Git commands
- arbitrary shell
- push, pull, fetch, reset, clean, rebase, tag, force-push, branch deletion, remote mutation,
  deploys, package publishing, or PR creation
- secret, credential, token, wallet, keychain, shell-profile, or private signing key access
