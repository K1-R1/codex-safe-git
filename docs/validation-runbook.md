# Validation Runbook

This runbook records the proof expected for the canonical Go implementation in Codex App and Codex
CLI.

## Preconditions

- Do not enable Full Access or bypass the sandbox.
- Keep Codex at `workspace-write`.
- Keep `enabled_tools` to exactly:
  - `git_status`
  - `git_diff_summary`
  - `commit_files`
  - `ensure_commit_branch`
  - `create_commit_branch`
  - `merge_branch`
- Use explicit allowed repos or allowed repo roots.
- Use an explicit audit log path.
- Do not point active MCP config at a disposable worktree path.

## Local Go Verification

From `codex-safe-git/`:

```sh
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
```

If Go is not installed globally, set `GO=/absolute/path/to/go` for installer scripts and call the Go
binary directly for local checks.

Expected evidence:

- Formatting exits cleanly.
- Vet exits cleanly.
- Unit, integration-style policy, and MCP tests pass.
- Race tests pass where practical.
- Temporary repos and audit logs are cleaned.

## Direct Stdio Validation

Build a disposable binary and run `initialize`, `tools/list`, and at least one `tools/call` request
over stdin/stdout before changing Codex config.

Expected evidence:

- `serverInfo.name` is `codex-safe-git`.
- `tools/list` exposes only the six intended tools.
- `git_status` or `git_diff_summary` works for an allowed repo root.
- An outside-root call returns `{ "result": "refused", "reason": "..." }`.

## Installer Verification

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh --print-config
scripts/install-local.sh
```

Expected evidence:

- The install path is stable, user-owned, and not a source worktree dependency.
- The printed config uses the Go binary.
- The config has explicit allowed roots and audit log path.
- The installed binary runs without a source worktree dependency.

## App Validation

Reload Codex App as needed after installer or config changes.

Through the App MCP tools, prove:

- status/diff for a normal project repo under an allowed root
- status/diff for a Codex worktree under an allowed root
- outside-root refusal
- protected `main`/`master`/default branch refusal
- safe branch creation or preparation
- fast-forward merge into a non-default local target
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
