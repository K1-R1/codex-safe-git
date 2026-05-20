# Security Invariants

The Go implementation is the canonical `codex-safe-git` safety model.

- The MCP surface is exactly nine tools: `git_status`, `git_diff_summary`, `commit_files`,
  `ensure_commit_branch`, `create_commit_branch`, `merge_branch`, `list_worktrees`,
  `create_worktree`, and `safe_checkout`.
- Repos must be explicitly allowlisted or exact Git worktree roots under explicit allowed roots.
- `repo_path` must be the exact Git worktree root.
- `main`, `master`, the configured default branch, and operator-configured protected branches are
  protected.
- Commits on protected branches are refused.
- Branch creation/preparation for protected branches is refused.
- Merge targets on protected branches are refused.
- Detached commits are refused unless a safe branch is prepared first.
- Ambiguous Git states are refused: merge, rebase, cherry-pick, revert, bisect, and conflicts.
- Pre-existing staged changes are refused before commit/branch/merge mutations.
- Commits stage exactly the listed file set and verify the staged set before committing.
- Commit file paths are literal paths only; Git pathspec magic and symlink indirection are not allowed
  to expand or redirect the requested file set.
- Secret-bearing paths and likely secret material are refused before commit, including a second scan
  of the staged diff after exact-file staging.
- Audit records are metadata-only and never include file contents, full diffs, environment dumps, or
  credentials.
- Mutating operations fail closed before touching Git state when the audit log is unavailable.
- Git is invoked with fixed arguments only.
- Hooks, prompts, pagers, credential helpers, editors, and GPG signing are disabled.
- Branch names must be safe local branches, not refs, remotes, commit hashes, paths, or lock names.
- `merge_branch` is fast-forward only and targets only non-default local branches.
- `list_worktrees` redacts worktree paths outside the configured allowlist or allowed roots.
- `create_worktree` creates only new non-protected local branches under allowed local roots and
  refuses dirty, ambiguous, overlapping, existing, or secret-bearing paths.
- `safe_checkout` switches only clean worktrees to existing non-protected local branches and refuses
  branches already checked out in another worktree.
