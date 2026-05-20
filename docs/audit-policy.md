# Audit Policy

This policy is for private or team-local deployments of `codex-safe-git`.

## Record Contents

Audit records are JSON Lines metadata. They may contain action names, result states, repository
paths, file paths, worktree paths, branch names, commit hashes, counts, timestamps, and refusal
reasons.

Audit records must not contain file contents, full diffs, secret values, credentials, token material,
keychain material, shell profiles, environment dumps, wallet data, or private signing keys.

## Retention

Default private/team retention is 90 days on the local machine unless a stricter team policy applies.
For highly sensitive repositories, prefer shorter retention or exact per-repo audit logs with tighter
filesystem permissions.

Keep audit logs in a user-owned directory such as:

```text
~/.codex/log/codex-safe-git-audit.jsonl
```

Recommended permissions:

- directory: `0700`
- file: `0600`

## Review

Review audit records after unusual refusals, before team rollout changes, and when investigating
unexpected local Git state. Focus on action type, repo path, branch path, refusal reason, and commit
hash. Do not use audit logs as a substitute for source diff review.

## Rotation

Automated deletion or in-place rotation is intentionally not implemented in `codex-safe-git`.
Unexpected log movement can weaken the fail-closed audit guarantee and can hide records from the
operator.

The safe rotation process is manual and operator-owned:

1. Stop or disable the MCP server.
2. Move the current audit file to a dated private archive path.
3. Preserve permissions at `0600`.
4. Start Codex again and confirm a new audit record can be written.
5. Delete archived logs only after the retention window expires and only through the operator's
   normal local deletion policy.

Mutating tools continue to check audit writability before touching Git state.
