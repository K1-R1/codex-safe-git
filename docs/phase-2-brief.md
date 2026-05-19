# Codex Safe Git Phase 2 Brief

## Phase 1 Result

Phase 1 was fully achieved in a disposable scratch area under `/private/tmp/codex-safe-git-mvp.Qi01Xk`.

Validated behaviours:

- local commits can be created in an allowlisted scratch repo without changing Codex permissions
- only explicitly listed files are staged and committed
- non-allowlisted repos, non-Git repos, paths outside the repo, secret-bearing paths, likely secret material, attribution-bearing commit messages, empty commits, pre-existing staged changes, detached HEAD, merge, rebase, cherry-pick, bisect, and conflict states are refused
- a minimal JSONL audit trail was produced

Cleanup:

- `trash /private/tmp/codex-safe-git-mvp.Qi01Xk` was attempted first and failed due to permissions
- recursive `rm` was rejected by policy
- the exact scratch tree was then removed with `find /private/tmp/codex-safe-git-mvp.Qi01Xk -depth -delete`
- `test -e /private/tmp/codex-safe-git-mvp.Qi01Xk` returned exit `1`

## Phase 2 Scope

Build a repo-contained stdio MCP server with no third-party runtime dependency and no Codex configuration changes.

Tools:

```text
git_status(repo_path)
git_diff_summary(repo_path)
commit_files(repo_path, files[], message, body?)
```

Runtime configuration is explicit and fail-closed:

- `CODEX_SAFE_GIT_ALLOWED_REPOS`: path-list of allowed Git worktree roots
- `CODEX_SAFE_GIT_AUDIT_LOG`: JSONL audit log path

No project, user, MCP, plugin, hook, rule, AGENTS, app, permission, sandbox, or Full Access settings are changed by this implementation.

## Safety Model

- Exact allowlist match is required after resolving `repo_path`.
- Tool inputs are schema-narrowed and unexpected arguments are refused server-side.
- `repo_path` must be a Git worktree root.
- `commit_files` refuses detached HEAD, merge, rebase, cherry-pick, revert, bisect, unresolved conflict, and pre-existing staged-change states.
- All tools refuse repos with execution-capable Git configuration for clean/smudge/process filters, custom diff commands/textconv, or fsmonitor.
- Requested paths must stay inside the repo and must be regular files or tracked deletions.
- Secret-bearing paths are refused before content inspection.
- Likely secret material in requested additions is refused.
- The tool stages only listed files and verifies the staged set exactly before committing.
- Empty commits are refused.
- Commit messages with Codex/OpenAI/ChatGPT/generated-by/co-authored attribution are refused.
- Git subprocesses use fixed argument lists, disabled hooks, no credential helper, no terminal prompting, no external diff, no signing, and no network-oriented Git operations.
- Audit records only timestamp, action, result, repo, files, commit hash where applicable, counts, and refusal reason.

## MCP Notes

Current OpenAI Codex docs describe stdio MCP servers as configured with `command`, optional `args`, `env`, `env_vars`, and `cwd`, while `config.toml` supports `mcp_servers.<id>.enabled_tools` to allowlist exposed tools.

Current MCP specification docs describe stdio transport as newline-delimited JSON-RPC on stdin/stdout, with logging reserved for stderr. The server implements the minimal lifecycle and tool methods needed for deterministic local tests:

- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

## Non-Goals

The Phase 2 server does not expose:

- arbitrary shell commands
- arbitrary Git commands
- push, pull, fetch, merge, rebase, reset, clean, tag, branch deletion, remote mutation, deploy, package publish, or PR creation operations
- credential, token, or secret-reading tools

## Verification

Required checks:

```sh
PYTHONPYCACHEPREFIX=/private/tmp/codex-safe-git-pycache python3 -m compileall -q src tests
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src python3 -m unittest discover -s tests -v
git status --short
```

The tests must prove the commit path, refusal paths, exact listed-file staging, audit logging, and MCP tool surface.
