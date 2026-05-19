# MCP Contract

`codex-safe-git` exposes a deliberately small stdio MCP surface. Tool names, required arguments, and
top-level response keys are treated as compatibility boundaries for private/team use.

## Server Version

The MCP `initialize` response includes `serverInfo.version`. Version changes should accompany
intentional contract changes.

## Tool Surface

- `git_status(repo_path)`
- `git_diff_summary(repo_path)`
- `commit_files(repo_path, files[], message, body?)`
- `ensure_commit_branch(repo_path, branch_name)`
- `create_commit_branch(repo_path, branch_name)`
- `merge_branch(repo_path, source_branch, target_branch?)`

No tool accepts arbitrary Git arguments or shell commands.

## Result Shapes

Structured result shapes are snapshotted in
[`schemas/tool-results.schema.json`](schemas/tool-results.schema.json).

All refusals return:

```json
{ "result": "refused", "reason": "..." }
```

Audit records are metadata-only. They may contain action names, result, repo path, branch names,
file names, counts, refusal reasons, and commit hashes. They must not contain file contents, full
diffs, secrets, credentials, keychain material, or environment dumps.
