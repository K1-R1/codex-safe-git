# MCP Contract

`codex-safe-git` is the production implementation behind the `codex_safe_git` MCP server id.

## Server Version

The MCP `initialize` response exposes `serverInfo.version`. Version changes should accompany
intentional contract changes.

## Tool Surface

The server exposes exactly nine tools:

- `git_status(repo_path)`
- `git_diff_summary(repo_path)`
- `commit_files(repo_path, files[], message, body?)`
- `ensure_commit_branch(repo_path, branch_name)`
- `create_commit_branch(repo_path, branch_name)`
- `merge_branch(repo_path, source_branch, target_branch?)`
- `list_worktrees(repo_path)`
- `create_worktree(repo_path, worktree_path, branch_name, base_branch?)`
- `safe_checkout(repo_path, branch_name)`

No tool accepts arbitrary Git arguments, shell commands, remotes, tags, force options, push, pull,
fetch, reset, clean, rebase, deploy, PR, publish, or credential operations.

`list_worktrees` returns only worktree paths that are themselves explicitly allowlisted or under an
allowed repo root. Unallowlisted worktrees are counted and redacted.

`create_worktree` creates a linked local worktree at a new path that is explicitly allowlisted or
under an allowed repo root. It creates a new safe local branch, refuses protected branch names,
requires a clean source worktree, refuses overlapping worktree paths, avoids remotes, and checks audit
writability before running `git worktree add`.

`safe_checkout` switches a clean worktree to an existing safe local branch. It refuses protected
target branches, missing branches, branches already checked out in another worktree, ambiguous Git
states, dirty worktrees, remotes, refs, hashes, and unsafe branch syntax.

## Result Shapes

Structured tool result shapes are snapshotted in
[`schemas/tool-results.schema.json`](schemas/tool-results.schema.json).

Tools also expose MCP `annotations` and concise per-tool `outputSchema` definitions in
`tools/list`, so clients can distinguish read-only inspection from local Git mutations and validate
`structuredContent` without reading this repository's docs.

Status, diff, untracked, and worktree arrays are bounded. Result payloads include the full visible
count, limit, and truncation flag next to each bounded array. Secret-bearing paths and unallowlisted
worktrees are counted separately and still redacted before truncation.

All refusals return:

```json
{ "result": "refused", "reason": "..." }
```

`tools/call` wraps the structured payload in MCP content:

```json
{
  "content": [{ "type": "text", "text": "{...}" }],
  "structuredContent": { "...": "..." }
}
```

Refusals also set `isError: true`.

## Compatibility Expectations

- Empty lists serialise as `[]`, not `null`.
- Response objects use deterministic JSON field names.
- Audit records are metadata-only and never include file contents, full diffs, secrets, credentials,
  keychain material, shell profiles, or environment dumps.
- Mutating operations require a writable audit log before Git state is changed.
- Requested commit file paths are literal and must not resolve through symlinks.
- Commit requests are bounded to 200 explicit files.
- `commit_files` scans likely secret material before staging and rescans the staged diff before
  committing.
- Worktree paths returned by `list_worktrees` must not disclose unallowlisted local paths.
- Git subprocesses ignore system and global Git config, while still reading repository-local config
  needed for the current worktree.
- New tools require an explicit product decision. The default stance remains to keep the surface
  closed.
