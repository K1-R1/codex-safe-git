# Operator Guide

This guide is for private or team-local use of the Go `codex-safe-git` MCP server with Codex App and
Codex CLI.

## Install Or Update

From the `codex-safe-git` source directory:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
```

Defaults:

- install path: `~/.codex/tools/codex-safe-git-go`
- binary: `~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp`
- allowed roots: `~/.codex/worktrees:$HOME/personal/projects`
- audit log: `~/.codex/log/codex-safe-git-audit.jsonl`
- protected branches: built-in `main`/`master` plus the repo's configured `init.defaultBranch`

Supported overrides:

```sh
CODEX_HOME="$HOME/.codex" \
CODEX_SAFE_GIT_INSTALL_DIR="$HOME/.codex/tools/codex-safe-git-go" \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS="$HOME/.codex/worktrees:$HOME/personal/projects" \
CODEX_SAFE_GIT_AUDIT_LOG="$HOME/.codex/log/codex-safe-git-audit.jsonl" \
CODEX_SAFE_GIT_PROTECTED_BRANCHES="trunk,develop" \
scripts/install-local.sh
```

Set `GO=/absolute/path/to/go` if Go is not on `PATH`. After install, normal Codex use does not need
the Go source tree or a Go toolchain.

Use `scripts/install-local.sh --print-config` to print only the MCP config block.

By default, custom install directories must remain under `$CODEX_HOME/tools`. If you need a different
private tool directory, set `CODEX_SAFE_GIT_ALLOW_EXTERNAL_INSTALL_DIR=1` and keep the path
user-owned, narrow, and outside any project worktree.

## MCP Config

Use the config printed by the installer. The active production server keeps the id
`codex_safe_git`, and its command points at the stable Go binary:

```toml
[mcp_servers.codex_safe_git]
command = "/Users/you/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp"
enabled_tools = ["git_status", "git_diff_summary", "commit_files", "ensure_commit_branch", "create_commit_branch", "merge_branch"]
default_tools_approval_mode = "approve"
enabled = true

[mcp_servers.codex_safe_git.env]
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "/Users/you/.codex/worktrees:/Users/you/projects"
CODEX_SAFE_GIT_AUDIT_LOG = "/Users/you/.codex/log/codex-safe-git-audit.jsonl"
CODEX_SAFE_GIT_PROTECTED_BRANCHES = "trunk,develop"
```

`default_tools_approval_mode = "approve"` applies only to this narrow MCP server. It does not change
Codex sandbox mode, Full Access, global approval policy, app-wide settings, or the permissions of
any other tool.

## Allowed Roots

Set `CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS` to explicit local containers for repositories Codex may work
in. Each requested `repo_path` must still be an exact Git worktree root.

Good examples:

- `~/.codex/worktrees`
- `~/personal/projects`
- a team-local projects directory

Avoid:

- `/`
- `$HOME`
- system directories
- cloud-sync roots with unrelated material
- credential, wallet, keychain, shell-profile, or secrets directories

For highly sensitive work, use `CODEX_SAFE_GIT_ALLOWED_REPOS` with exact repo paths instead of broad
roots.

## Protected Branches

`main`, `master`, and the repository's configured `init.defaultBranch` are always protected. Add
team-specific production branch names with `CODEX_SAFE_GIT_PROTECTED_BRANCHES`, using a comma-separated
list such as `trunk,develop,release/stable`. Protected branches cannot be commit targets, branch
creation targets, or merge targets.

## Audit Log

Audit records are JSON Lines. They contain metadata only: action, result, repo path, branch names,
file names, counts, refusal reasons, and commit hashes. They must not contain file contents, full
diffs, secrets, credentials, keychain material, shell profiles, or environment dumps.

Keep the audit log under a user-owned path such as `~/.codex/log/codex-safe-git-audit.jsonl`.
Mutating operations check audit writability before touching Git state and refuse if the audit log is
unavailable.

## Troubleshooting

- `repo_path is not explicitly allowlisted`: add the exact repo or a narrow parent root, then reload
  the Codex session.
- `repo_path must be the Git worktree root`: call the tool with the repository root, not a
  subdirectory.
- `repository already has staged changes`: unstage manually or start from a clean index.
- `repository has ambiguous state`: finish or abort the merge, rebase, cherry-pick, revert, bisect,
  or conflict manually.
- `refusing commit on protected branch`: create or switch to a safe non-default branch.
- `audit log is not writable`: fix the configured audit path or permissions before retrying.
- `requested path must not be a symlink`: commit the real file explicitly, or remove the symlink from
  the requested file list.
- `merge is not fast-forward`: merge manually or create a branch shape that can fast-forward.

## Removal

To remove the local Go install, first remove or disable the `codex_safe_git` MCP entry from Codex
configuration, then delete `~/.codex/tools/codex-safe-git-go`. Keep or archive the audit log
according to your local retention needs.
