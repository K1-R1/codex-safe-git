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
- `commit_files`
- `ensure_commit_branch`
- `create_commit_branch`
- `merge_branch`
- `list_worktrees`
- `create_worktree`
- `safe_checkout`

## Completed Private/Team Readiness

- CI coverage is defined for formatting, vet, tests, race tests, installer checks, direct stdio MCP
  smoke tests, and safety regression cases.
- `scripts/verify.sh` is the one-command local verification path.
- The deferred local worktree tools have been implemented with explicit safety constraints and test
  coverage.
- The audit retention, review, and manual rotation policy is documented in
  [Audit policy](audit-policy.md).
- Private/team onboarding is documented in [Team onboarding](team-onboarding.md).
- Private binary integrity uses installer-generated SHA-256 checksums and `--verify-install`.
- Private signing strategy is documented without requiring Codex to access signing keys.
- Protected-branch landing remains outside this MCP and is documented as a human-reviewed workflow.

Automated audit deletion or rotation is intentionally not implemented. That is now a documented
non-goal because audit movement should remain operator-owned and visible.

## Remaining Work

Only full open-source preparation remains deferred until explicitly approved. That future work would
include public release policy, public documentation polish, public support expectations, public
licensing/release review, public security disclosure process, and any public distribution signing
process.

## Permanent Non-Goals

These are not future features:

- arbitrary Git commands
- arbitrary shell
- push, pull, fetch, reset, clean, rebase, tag, force-push, branch deletion, remote mutation,
  deploys, package publishing, or PR creation
- secret, credential, token, wallet, keychain, shell-profile, or private signing key access
