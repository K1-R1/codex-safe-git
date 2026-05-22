# Security Invariants

The Go implementation is the canonical `codex-safe-git` safety model.

- The MCP surface is closed and explicitly listed in [MCP contract](mcp-contract.md). New tools
  require a product decision and tests before exposure.
- Tools expose MCP annotations and output schemas so clients can reason about read-only versus
  mutating operations and validate structured results.
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
- Commit requests are bounded to 200 explicit files.
- Commits stage exactly the listed file set and verify the staged set before committing.
- Commit file paths are literal paths only; Git pathspec magic and symlink indirection are not allowed
  to expand or redirect the requested file set.
- Secret-bearing paths and likely secret material are refused before commit. Untracked file content is
  scanned with byte and line bounds, and staged diffs are rescanned after exact-file staging.
- Audit records are metadata-only and never include file contents, full diffs, environment dumps, or
  credentials.
- Mutating operations fail closed before touching Git state when the audit log is unavailable.
- Status, diff, untracked, and worktree list payloads are bounded and include count, limit, and
  truncation metadata.
- Ref, history, path, submodule, integrity, reflog, and self-check payloads are bounded structured
  summaries and do not include patch text, blob contents, raw object dumps, remote state, or hidden
  allowlist paths.
- Git subprocess stdout and stderr capture is hard-bounded before higher-level parsing, and oversized
  command output fails closed instead of being partially interpreted.
- MCP stdio request frames are bounded and malformed or oversized frames return structured errors
  without preventing later valid requests on the stream.
- MCP read-only tools do not write audit records or other intentional local state.
- Git is invoked with fixed arguments only.
- Hooks, prompts, pagers, credential helpers, editors, and GPG signing are disabled.
- `git` is resolved from trusted install locations or an explicit absolute `CODEX_SAFE_GIT_GIT_PATH`.
- System and global Git config are ignored by Git subprocesses; repository-local config is still
  inspected for execution-capable settings.
- Branch names must be safe local branches, not refs, remotes, commit hashes, paths, or lock names.
- `merge_branch` is fast-forward only and targets only non-default local branches.
- `list_worktrees` redacts worktree paths outside the configured allowlist or allowed roots.
- `create_worktree` creates only new non-protected local branches under allowed local roots and
  refuses dirty, ambiguous, overlapping, existing, or secret-bearing paths.
- `safe_checkout` switches only clean worktrees to existing non-protected local branches and refuses
  branches already checked out in another worktree.
