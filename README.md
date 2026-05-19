# Codex Safe Git

`codex-safe-git` is a narrow stdio MCP server for deterministic local Git commits.

It is intentionally not a general Git automation server. It exposes only:

- `git_status(repo_path)`
- `git_diff_summary(repo_path)`
- `ensure_commit_branch(repo_path, branch_name)`
- `create_commit_branch(repo_path, branch_name)`
- `commit_files(repo_path, files[], message, body?)`
- `merge_branch(repo_path, source_branch, target_branch?)`

## Safety Defaults

- Fails closed unless `CODEX_SAFE_GIT_ALLOWED_REPOS` and/or
  `CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS`, plus `CODEX_SAFE_GIT_AUDIT_LOG`, are set.
- Requires exact repo allowlist matches or an exact Git worktree root beneath an explicit allowed
  repo root after path resolution.
- Refuses ambiguous Git states, pre-existing staged changes, protected default-branch commits, unsafe paths, likely secrets, attribution-bearing messages, unexpected MCP arguments, and execution-capable Git configuration.
- Can attach a detached worktree to an explicit safe local non-default branch at current `HEAD` before committing.
- Can create/switch to explicit safe local non-default branches at current `HEAD`.
- Can fast-forward merge one local branch into the current non-default local branch.
- Uses fixed Git subprocess arguments only.
- Does not expose arbitrary shell commands or arbitrary Git commands.

## Project Direction

Codex Safe Git is intended to provide local-only, sandbox-friendly Git operations for Codex app
and Codex CLI across Codex projects. The long-term goal is safe, best-practice Git hygiene for
Codex while preserving normal `workspace-write` permissions.

It should remain narrow by design. Arbitrary shell, arbitrary Git, remotes, publishing, PRs, and
deployment workflows are permanent non-goals rather than deferred features.

## Run Locally

From this directory:

```sh
PYTHONPATH=src \
CODEX_SAFE_GIT_ALLOWED_REPOS=/absolute/path/to/repo \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS=/absolute/path/to/codex/worktrees:/absolute/path/to/projects \
CODEX_SAFE_GIT_AUDIT_LOG=/absolute/path/to/audit.jsonl \
python3 -m codex_safe_git
```

Installed entry point:

```sh
codex-safe-git-mcp
```

## Test

```sh
scripts/verify.sh
```

Equivalent commands:

```sh
PYTHONPYCACHEPREFIX=/private/tmp/codex-safe-git-pycache \
python3 -m compileall -q src tests

PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src \
python3 -m unittest discover -s tests -v
```

## Codex MCP Configuration

See [docs/codex-config-example.toml](docs/codex-config-example.toml). The example is intentionally not installed automatically.

For a stable local setup that works from any Codex app or CLI session, install a copy under
`~/.codex/tools/codex-safe-git`:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
```

The script copies this server to the stable tool path, creates a fixed
`codex-safe-git-mcp` wrapper, and prints the MCP config block to use.

For day-to-day Codex use, configure one local MCP registration and set
`CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS` to explicit project containers, such as the Codex worktree root
and your local projects root. This makes `codex_safe_git` available across projects while still
requiring each request to name an exact Git worktree root.

The recommended config uses server-local `default_tools_approval_mode = "approve"` for the six
safe-git tools only. This keeps non-interactive Codex CLI runs usable without changing global
approval policy, sandbox mode, or the narrow MCP surface.

For first-time setup and operations, see [docs/operator-guide.md](docs/operator-guide.md). For the
MCP response contract, see [docs/mcp-contract.md](docs/mcp-contract.md).

For end-to-end app and CLI proof steps, see [docs/validation-runbook.md](docs/validation-runbook.md).
