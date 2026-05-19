# Codex Safe Git Validation Runbook

This runbook records the proof needed before calling the Codex Safe Git MCP complete for both Codex app
and Codex CLI safe setups. Run these commands from a normal interactive terminal, not from a
sandboxed Codex app shell that cannot read `~/.codex`.

## Preconditions

- Do not enable Full Access or `--dangerously-bypass-approvals-and-sandbox`.
- Keep the Codex sandbox at `workspace-write`.
- Keep the `codex_safe_git` MCP entry limited to:
  - `git_status`
  - `git_diff_summary`
  - `ensure_commit_branch`
  - `commit_files`
- Set `mcp_servers.codex_safe_git.default_tools_approval_mode = "approve"` for this server only. This
  does not change the global approval policy or sandbox mode; it lets non-interactive `codex exec`
  use the deliberately narrow codex-safe-git surface.
- Allowlist exact repos for narrow tests, or explicit repo roots such as the Codex worktree root and
  local projects root for default day-to-day use. Do not use `/` or broad system directories.
- Use an explicit audit log path and remove temporary audit logs after review.

## Local Codex Safe Git Tests

From `codex-safe-git/`:

```sh
PYTHONPYCACHEPREFIX=/private/tmp/codex-safe-git-pycache \
python3 -m compileall -q src tests

PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src \
python3 -m unittest discover -s tests -v
```

Expected evidence:

- Compile command exits 0.
- Unit/integration/MCP test suite exits 0.
- Worktree tests cover ordinary repos, linked worktrees, detached branch preparation, protected
  branch refusal, exact file staging, audit metadata, and MCP tool surface.

## Codex CLI MCP Registration

From any repo:

```sh
codex mcp get codex_safe_git
codex mcp list
```

Expected evidence:

- `codex_safe_git` uses the local `codex-safe-git` server.
- `enabled_tools` is exactly `["git_status", "git_diff_summary", "commit_files", "ensure_commit_branch"]`.
- `default_tools_approval_mode` is `approve`, or each of the four enabled tools has
  `approval_mode = "approve"`.
- The exact repo allowlist and/or allowed repo roots are explicit, and the audit log path is explicit.
- No broader Git, shell, network, push, pull, reset, merge, rebase, remote, deploy, PR, or publish
  tools are exposed.

## Codex CLI Live Test

Run this from a normal terminal with Codex CLI auth available:

```sh
codex exec --json --ephemeral --skip-git-repo-check --sandbox workspace-write \
  'Use only the codex_safe_git MCP tools. For /absolute/path/to/allowlisted/repo, call git_status, then git_diff_summary. If the repo is detached, call ensure_commit_branch with branch_name "codex/codex-safe-git-cli-validation". If there is one intentional file change, commit only that exact file with commit_files. Report the MCP tool names used, final branch, clean status, commit hash if any, and committed file list.'
```

Expected evidence:

- The CLI agent calls only `codex_safe_git` MCP tools for Git state and commit work.
- MCP calls complete successfully. `user cancelled MCP tool call` means the approval mode or prompt
  handling is not yet configured for non-interactive validation.
- Detached worktrees are prepared with `ensure_commit_branch`.
- Commits occur only on a non-`main`/non-`master` branch.
- `commit_files` receives exact file paths, not directories or globs.
- Audit log contains metadata only: action, result, repo, branch, filenames, and commit hash.

## Refusal Checks

Use disposable repos or clean states for refusal checks:

- `ensure_commit_branch(..., "main")` is refused.
- `ensure_commit_branch(..., "master")` is refused.
- `ensure_commit_branch(..., "origin/unsafe")` is refused.
- Unallowlisted repo paths are refused.
- Paths under allowed repo roots that are not exact Git worktree roots are refused.
- Detached `commit_files` without branch preparation is refused.

## Cleanup

- Remove temporary repos.
- Remove temporary audit logs after reviewing metadata.
- Remove generated pycache or test scratch directories.
- Confirm the repo has only intentional commits and no staged changes.
