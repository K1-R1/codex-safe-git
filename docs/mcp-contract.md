# MCP Contract

`codex-safe-git` is the production implementation behind the `codex_safe_git` MCP server id.

## Server Version

The MCP `initialize` response exposes `serverInfo.version`. Version changes should accompany
intentional contract changes.

## Tool Surface

The server exposes exactly six tools:

- `git_status(repo_path)`
- `git_diff_summary(repo_path)`
- `commit_files(repo_path, files[], message, body?)`
- `ensure_commit_branch(repo_path, branch_name)`
- `create_commit_branch(repo_path, branch_name)`
- `merge_branch(repo_path, source_branch, target_branch?)`

No tool accepts arbitrary Git arguments, shell commands, remotes, tags, force options, push, pull,
fetch, reset, clean, rebase, deploy, PR, publish, or credential operations.

## Result Shapes

Structured tool result shapes are snapshotted in
[`schemas/tool-results.schema.json`](schemas/tool-results.schema.json).

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
- New tools require an explicit product decision. The default stance is to keep the surface closed.
