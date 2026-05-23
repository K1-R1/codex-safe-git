# Getting Started

This guide is the shortest path to a local `codex-safe-git` install for Codex App or Codex CLI.

## Public Install

Install the command with Go:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@latest
mkdir -p "$HOME/.codex/worktrees" "$HOME/.codex/log"
codex-safe-git-mcp --print-config
```

If the Go install directory is not on `PATH`, locate it with:

```sh
bin_dir="$(go env GOBIN)"
if [ -z "$bin_dir" ]; then
  bin_dir="$(go env GOPATH)/bin"
fi
"$bin_dir/codex-safe-git-mcp" --print-config
```

Use the printed config block in Codex App or Codex CLI, then reload Codex.

See [Distribution](distribution.md) for the full public, pinned-version, and source-tree install
policy.

## Verify

From the repository root:

```sh
scripts/verify.sh
```

The script runs formatting, vet, tests, coverage-reporting tests, race tests, installer checks,
checksum verification, and direct stdio MCP smoke validation.

## Source-Tree Install

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

Use the config printed by the installer. The active MCP command should point to the stable installed
binary under `~/.codex/tools/codex-safe-git-go`, not to a source worktree.

## Configure Allowed Roots

Set narrow local roots:

```toml
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "/Users/you/.codex/worktrees:/Users/you/projects"
```

Prefer exact `CODEX_SAFE_GIT_ALLOWED_REPOS` entries for highly sensitive repositories. Do not allow
`/`, `$HOME`, credential directories, wallet directories, shell profile directories, or broad
cloud-sync roots.

Every `repo_path` passed to the MCP must be an exact Git worktree root, even when it is under an
allowed parent root.

## Configure Protected Branches

`main`, `master`, and each repository's configured `init.defaultBranch` are protected automatically.
Add extra production branch names when needed:

```toml
CODEX_SAFE_GIT_PROTECTED_BRANCHES = "trunk,develop,release/stable"
```

Protected branches cannot be commit targets, branch creation targets, merge targets, worktree target
branches, or checkout targets through this MCP.

## Daily Use

Use `git_status` and `git_diff_summary` to inspect state. Prepare work with
`ensure_commit_branch`, `create_commit_branch`, `create_worktree`, or `safe_checkout`. Commit only
exact listed files with `commit_files`. Fast-forward only between non-protected local branches with
`merge_branch`.

The MCP does not push, fetch, pull, rebase, reset, clean, tag, delete branches, create PRs, publish
packages, deploy, or mutate remotes.

## Protected-Branch Landing

Final integration into protected branches belongs outside `codex-safe-git`: a human-reviewed pull
request, Codex's normal review/merge controls, or a manual operator merge. Before landing, run the
project's tests, inspect the branch diff, confirm audit records look sane, and verify the target
branch policy.

## Updating

1. Review the source diff and docs.
2. Run `scripts/verify.sh`.
3. Run `scripts/install-local.sh`.
4. Run `scripts/install-local.sh --verify-install`.
5. Reload Codex App or restart Codex CLI sessions that use the MCP.
6. Run a disposable validation from `docs/validation-runbook.md`.

## Audit Logs

Keep logs under a user-owned path such as `~/.codex/log/codex-safe-git-audit.jsonl`. Follow
`docs/audit-policy.md` for retention, review, and manual rotation.

## Removal

Disable or remove the MCP config entry first. Then remove the installed binary directory. Keep,
archive, or delete audit logs according to your local retention policy.
