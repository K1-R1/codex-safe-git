# Security Invariants

The Go implementation is the canonical `codex-safe-git` safety model.

- The MCP surface is exactly six tools: `git_status`, `git_diff_summary`, `commit_files`,
  `ensure_commit_branch`, `create_commit_branch`, and `merge_branch`.
- Repos must be explicitly allowlisted or exact Git worktree roots under explicit allowed roots.
- `repo_path` must be the exact Git worktree root.
- `main`, `master`, and the configured default branch are protected.
- Commits on protected branches are refused.
- Branch creation/preparation for protected branches is refused.
- Merge targets on protected branches are refused.
- Detached commits are refused unless a safe branch is prepared first.
- Ambiguous Git states are refused: merge, rebase, cherry-pick, revert, bisect, and conflicts.
- Pre-existing staged changes are refused before commit/branch/merge mutations.
- Commits stage exactly the listed file set and verify the staged set before committing.
- Secret-bearing paths and likely secret material are refused before commit.
- Audit records are metadata-only and never include file contents, full diffs, environment dumps, or
  credentials.
- Git is invoked with fixed arguments only.
- Hooks, prompts, pagers, credential helpers, editors, and GPG signing are disabled.
- Branch names must be safe local branches, not refs, remotes, commit hashes, paths, or lock names.
- `merge_branch` is fast-forward only and targets only non-default local branches.
