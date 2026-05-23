# Operator Guide

This guide covers local operation of the Go `codex-safe-git` MCP server with Codex App and Codex
CLI.

## Public Install Or Update

Install or update the command with Go:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@latest
mkdir -p "$HOME/.codex/worktrees" "$HOME/.codex/log"
codex-safe-git-mcp --print-config
```

For a pinned install, use a release tag:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@v0.4.3
```

If the installed command is not on `PATH`, run it from `$(go env GOBIN)` when set, otherwise from
`$(go env GOPATH)/bin`.

## Source-Tree Install Or Update

From the `codex-safe-git` source directory:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

Defaults:

- install path: `~/.codex/tools/codex-safe-git-go`
- binary: `~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp`
- checksum: `~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp.sha256`
- allowed roots: `~/.codex/worktrees`
- audit log: `~/.codex/log/codex-safe-git-audit.jsonl`
- protected branches: built-in `main`/`master` plus the repo's configured `init.defaultBranch`
- git executable: trusted system/Homebrew/MacPorts/Nix Git from `PATH`, or explicit
  `CODEX_SAFE_GIT_GIT_PATH`

Supported overrides:

```sh
CODEX_HOME="$HOME/.codex" \
CODEX_SAFE_GIT_INSTALL_DIR="$HOME/.codex/tools/codex-safe-git-go" \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS="$HOME/.codex/worktrees:$HOME/projects" \
CODEX_SAFE_GIT_AUDIT_LOG="$HOME/.codex/log/codex-safe-git-audit.jsonl" \
CODEX_SAFE_GIT_PROTECTED_BRANCHES="trunk,develop" \
CODEX_SAFE_GIT_GIT_PATH="/usr/bin/git" \
scripts/install-local.sh
```

Set `GO=/absolute/path/to/go` if Go is not on `PATH`. After install, normal Codex use does not need
the Go source tree or a Go toolchain.

When using the default allowed root, the installer creates `~/.codex/worktrees` so the printed config
is immediately usable. If you override `CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS`, create those directories
first; the MCP refuses missing allowed roots at startup rather than silently widening scope.

Use `scripts/install-local.sh --print-config` to print only the MCP config block.
Use `scripts/install-local.sh --verify-install` to verify the installed binary against its SHA-256
sidecar checksum.

By default, custom install directories must remain under `$CODEX_HOME/tools`. If you need a different
tool directory, set `CODEX_SAFE_GIT_ALLOW_EXTERNAL_INSTALL_DIR=1` and keep the path user-owned,
narrow, and outside any project worktree.

## MCP Config

Use the config printed by the installer. The active production server keeps the id
`codex_safe_git`, and its command points at the stable Go binary:

```toml
[mcp_servers.codex_safe_git]
command = "/Users/you/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp"
enabled_tools = ["git_status", "git_diff_summary", "list_local_branches", "compare_refs", "commit_log_summary", "show_commit_summary", "list_local_refs", "merge_base", "changed_files_between_refs", "path_status", "submodule_summary", "repository_integrity_check", "reflog_summary", "self_check", "commit_files", "ensure_commit_branch", "create_commit_branch", "merge_branch", "list_worktrees", "create_worktree", "safe_checkout"]
default_tools_approval_mode = "approve"
enabled = true

[mcp_servers.codex_safe_git.env]
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "/Users/you/.codex/worktrees:/Users/you/projects"
CODEX_SAFE_GIT_AUDIT_LOG = "/Users/you/.codex/log/codex-safe-git-audit.jsonl"
CODEX_SAFE_GIT_PROTECTED_BRANCHES = "trunk,develop"
CODEX_SAFE_GIT_GIT_PATH = "/usr/bin/git"
```

`default_tools_approval_mode = "approve"` applies only to this narrow MCP server. It does not change
Codex sandbox mode, Full Access, global approval policy, app-wide settings, or the permissions of
any other tool.

## Allowed Roots

Set `CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS` to explicit local containers for repositories Codex may work
in. Each requested `repo_path` must still be an exact Git worktree root.

Good examples:

- `~/.codex/worktrees`
- `~/projects`
- a dedicated projects directory

Avoid:

- `/`
- `$HOME`
- system directories
- cloud-sync roots with unrelated material
- credential, wallet, keychain, shell-profile, or secrets directories

For highly sensitive work, use `CODEX_SAFE_GIT_ALLOWED_REPOS` with exact repo paths instead of broad
roots.

## Git Executable

By default the server accepts `git` only from narrow trusted install locations such as `/usr/bin`,
Homebrew, MacPorts, or Nix store paths. If your environment uses a different Git binary, set
`CODEX_SAFE_GIT_GIT_PATH` to an absolute operator-controlled path. The server refuses relative paths,
unexpected executable names, and untrusted `PATH` lookups.

## Protected Branches

`main`, `master`, and the repository's configured `init.defaultBranch` are always protected. Add
environment-specific production branch names with `CODEX_SAFE_GIT_PROTECTED_BRANCHES`, using a
comma-separated list such as `trunk,develop,release/stable`. Protected branches cannot be commit
targets, branch creation targets, or merge targets.

## Landing Protected Branches

`codex-safe-git` is not the tool that lands work onto `main`, `master`, or another protected branch.
That final integration step should remain a human-reviewed repository workflow, such as a pull
request, Codex's normal review/merge controls, or a manual terminal merge performed by the operator.

Before landing a Codex branch:

- run the project tests and any repository-specific validation
- use `git_status` and `git_diff_summary` to confirm the Codex branch is clean and scoped
- review the branch diff through the normal repository review path
- merge into the protected branch outside `codex-safe-git`

This boundary is intentional: Codex may prepare, commit, branch, and fast-forward between safe
non-default local branches, but it cannot directly mutate production/default branch targets.

## Worktree And Checkout Tools

`list_worktrees` shows only worktrees that are themselves explicitly allowlisted or under an allowed
repo root. Worktrees outside those roots are counted and redacted to avoid disclosing unrelated local
paths.

`create_worktree` creates a linked local worktree on a new safe branch. The requested path must not
exist, its parent must already exist, it must be under an allowed root or exact allowlist entry, and
it must not overlap another worktree or pass through secret-bearing path components. The source
worktree must be clean and unambiguous.

`safe_checkout` switches only clean worktrees to existing non-protected local branches. It refuses
remote/ref/hash-like branch names and branches already checked out in another worktree.

## Audit Log

Audit records are JSON Lines. They contain metadata only: action, result, repo path, branch names,
file names, counts, refusal reasons, and commit hashes. They must not contain file contents, full
diffs, secrets, credentials, keychain material, shell profiles, or environment dumps.

Keep the audit log under a user-owned path such as `~/.codex/log/codex-safe-git-audit.jsonl`.
Mutating operations check audit writability before touching Git state and refuse if the audit log is
unavailable.
`self_check.audit_log_writable` is a non-mutating path and permission check for operator visibility;
it does not append to the audit log.

See [Audit policy](audit-policy.md) for retention, review, and rotation guidance.

## Local Binary Integrity

The installer writes a SHA-256 checksum beside the installed binary and verifies it immediately after
build. Before copying a local binary between machines, copy both files and run:

```sh
scripts/install-local.sh --verify-install
```

Checksum verification proves file integrity against the sidecar created at install time, not public
release authorship. See [Local install integrity](local-install-integrity.md) for the current local
install boundary.

For public distribution policy, see [Distribution](distribution.md).
For maintainer release steps, see [Release process](release-process.md).

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
- `worktree_path is not explicitly allowlisted`: create the worktree under an allowed root or add an
  exact allowlist entry, then reload the Codex session.
- `branch is already checked out in another worktree`: use that worktree, choose another branch, or
  ask the operator to move the branch manually.
- `worktree_path overlaps an existing worktree`: choose a sibling path outside all existing worktree
  roots.

## Removal

To remove the local Go install, first remove or disable the `codex_safe_git` MCP entry from Codex
configuration, then delete `~/.codex/tools/codex-safe-git-go`. Keep or archive the audit log
according to your local retention needs.
