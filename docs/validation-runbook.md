# Validation Runbook

This runbook records the proof expected for the canonical Go implementation in Codex App and Codex
CLI.

## Preconditions

- Do not enable Full Access or bypass the sandbox.
- Keep Codex at `workspace-write`.
- Keep `enabled_tools` to exactly:
  - `git_status`
  - `git_diff_summary`
  - `list_local_branches`
  - `compare_refs`
  - `commit_log_summary`
  - `show_commit_summary`
  - `list_local_refs`
  - `merge_base`
  - `changed_files_between_refs`
  - `path_status`
  - `submodule_summary`
  - `repository_integrity_check`
  - `reflog_summary`
  - `self_check`
  - `commit_files`
  - `ensure_commit_branch`
  - `create_commit_branch`
  - `merge_branch`
  - `list_worktrees`
  - `create_worktree`
  - `safe_checkout`
- Use explicit allowed repos or allowed repo roots.
- Use an explicit audit log path.
- Configure extra production branch names with `CODEX_SAFE_GIT_PROTECTED_BRANCHES` when a team uses
  names such as `trunk`, `develop`, or `release/stable`.
- Do not point active MCP config at a disposable worktree path.

## Local Go Verification

From `codex-safe-git/`:

```sh
scripts/verify.sh
```

If Go is not installed globally, set `GO=/absolute/path/to/go` when running the script.

Expected evidence:

- Formatting exits cleanly.
- Vet exits cleanly.
- Unit, integration-style policy, MCP tests, and coverage-reporting test runs pass.
- Race tests pass where practical.
- Installer dry-run and config printing succeed.
- A temporary install writes and verifies a SHA-256 checksum.
- Direct stdio MCP smoke validation succeeds.
- Temporary repos and audit logs are cleaned.

## Direct Stdio Validation

Build a disposable binary and run `initialize`, `tools/list`, and at least one `tools/call` request
over stdin/stdout before changing Codex config.

Expected evidence:

- `serverInfo.name` is `codex-safe-git`.
- `tools/list` exposes only the intended closed tool surface.
- `git_status` or `git_diff_summary` works for an allowed repo root.
- read-only ref, history, path, submodule, integrity, reflog, and self-check tools return bounded
  structured summaries.
- oversized or malformed stdio requests return structured errors and later valid requests still
  succeed.
- `list_worktrees` redacts worktrees outside the allowlist.
- `create_worktree` can create a disposable linked worktree under an allowed root.
- `safe_checkout` can switch that linked worktree to another existing non-protected local branch.
- An outside-root call returns `{ "result": "refused", "reason": "..." }`.

## Installer Verification

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh --print-config
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

Expected evidence:

- The install path is stable, user-owned, and not a source worktree dependency.
- The printed config uses the Go binary.
- The config has explicit allowed roots and audit log path.
- The printed TOML config escapes quotes, backslashes, tabs, and newlines in paths or branch lists.
- The default allowed root `~/.codex/worktrees` exists after install.
- The installed binary has a sibling `.sha256` file.
- `--verify-install` detects the binary checksum correctly.
- The installer refuses canonical unsafe install paths such as `$CODEX_HOME/..`.
- The installed binary runs without a source worktree dependency.

## App Validation

Reload Codex App as needed after installer or config changes.

Through the App MCP tools, prove:

- status/diff for a normal project repo under an allowed root
- status/diff for a Codex worktree under an allowed root
- outside-root refusal
- protected `main`/`master`/default branch refusal
- configured protected branch refusal when applicable
- symlink and literal-pathspec exact-file safeguards
- safe branch creation or preparation
- fast-forward merge into a non-default local target
- allowed worktree listing
- unallowlisted worktree path redaction
- safe linked worktree creation on a new non-protected local branch
- strict checkout to an existing non-protected local branch
- exact-file commit
- metadata-only audit logging

Use disposable local repos for branch, merge, and commit validation, then clean them up.

## CLI Validation

From a normal terminal with Codex CLI auth available:

```sh
codex mcp get codex_safe_git
codex mcp list
codex exec --json --ephemeral --skip-git-repo-check --sandbox workspace-write 'Use only the codex_safe_git MCP tools...'
```

Expected evidence:

- `codex mcp get codex_safe_git` points to the Go binary.
- `codex mcp list` shows exactly the intended enabled tool surface.
- The CLI agent uses MCP tools for Git state, branch, merge, and commit work.
- No shell Git is used inside `codex exec`.
- Protected/default targets refuse.
- Non-default fast-forward merge succeeds.
- Exact-file commit succeeds.

If CLI auth/keychain requires a normal terminal, ask the operator to run the exact command and paste
the output. Do not weaken Codex App permissions to compensate.
