# Future TODO

## Current State

Codex Safe Git is now the canonical Go implementation. The earlier Python prototype has been
removed after local, direct stdio, Codex App, and Codex CLI validation.

The active local MCP config should point to:

```text
~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp
```

The MCP surface remains exactly:

- `git_status`
- `git_diff_summary`
- `commit_files`
- `ensure_commit_branch`
- `create_commit_branch`
- `merge_branch`

## Remaining Private/Team Work

- Decide audit log retention, review, and rotation policy for team machines.
- Add CI coverage for Go formatting, vet, tests, race tests, installer dry-run, and direct stdio MCP
  smoke tests.
- Add a private team onboarding guide for installing, updating, and configuring allowed repo roots.
- Add checksum/signing strategy for private binary distribution before broader team rollout.
- Keep open-source readiness deferred until explicitly approved.

## Possible Narrow Local Tools

Only local, deterministic, safety-reviewed tools should be considered. Candidate tools remain
deferred:

- `create_worktree`
- `list_worktrees`
- a stricter `safe_checkout`

Any new tool must preserve exact allowlists, refuse ambiguous states, avoid remotes, and never
become an arbitrary Git command runner.

## Permanent Non-Goals

These are not future features:

- arbitrary Git commands
- arbitrary shell
- push, pull, fetch, reset, clean, rebase, tag, force-push, branch deletion, remote mutation,
  deploys, package publishing, or PR creation
- secret, credential, token, wallet, keychain, or shell-profile access
