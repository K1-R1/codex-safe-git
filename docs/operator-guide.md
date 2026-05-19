# Operator Guide

This guide is for private or team-local use of `codex-safe-git` with Codex App and Codex CLI.

## Install Or Update

From the `codex-safe-git` source directory:

```sh
scripts/install-local.sh --dry-run
scripts/install-local.sh
```

Defaults:

- install path: `~/.codex/tools/codex-safe-git`
- wrapper: `~/.codex/tools/codex-safe-git/bin/codex-safe-git-mcp`
- allowed roots: `~/.codex/worktrees:$HOME/personal/projects`
- audit log: `~/.codex/log/codex-safe-git-audit.jsonl`

Supported overrides:

```sh
CODEX_HOME="$HOME/.codex" \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS="$HOME/.codex/worktrees:$HOME/personal/projects" \
CODEX_SAFE_GIT_AUDIT_LOG="$HOME/.codex/log/codex-safe-git-audit.jsonl" \
scripts/install-local.sh
```

Use `scripts/install-local.sh --print-config` to print only the MCP config block.

## MCP Config

Use the config printed by the installer. The active server should use the stable wrapper and should
not depend on a repo-local `cwd` or `PYTHONPATH`.

`default_tools_approval_mode = "approve"` is recommended for this MCP server only, because the
surface is narrow and deterministic. It does not change the global approval policy, Codex sandbox
mode, or any app-wide permissions.

## Allowed Roots

Set `CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS` to explicit local containers for repos Codex may work in.
Do not use `/`, `$HOME`, system directories, cloud-sync roots, or credential directories.

To add a new project area, append it to the path-separated root list and reload Codex App or restart
the Codex CLI session.

## Audit Log

Audit records are JSON Lines. They contain metadata only: action, result, repo path, branch names,
file names, counts, refusal reasons, and commit hashes. They must not contain file contents, full
diffs, secrets, credentials, keychain material, or environment dumps.

Keep the audit log under a user-owned path such as `~/.codex/log/codex-safe-git-audit.jsonl`.

## Refusal Examples

Expected refusals include:

- repo path outside configured roots
- repo path that is not the exact Git worktree root
- `main`, `master`, or configured default branch as a commit, branch-creation, or merge target
- remote-like branch names such as `origin/feature`
- dirty worktree before merge
- detached commit without branch preparation
- ambiguous Git states such as merge, rebase, cherry-pick, revert, bisect, or conflicts
- likely secret paths or likely secret material in requested commit diffs

## Verification Checklist

```sh
codex mcp get codex_safe_git
codex mcp list
```

Expected:

- command is the stable wrapper under `~/.codex/tools/codex-safe-git/bin`
- no `cwd`
- no `PYTHONPATH`
- enabled tools are exactly the documented `codex-safe-git` tools
- env contains explicit allowed roots and an explicit audit log path

Then run a Codex CLI smoke test under `--sandbox workspace-write` and confirm the agent uses only
`codex_safe_git` MCP tools for Git state and commit work.

## Removal

To remove the local install, first remove or disable the `codex_safe_git` MCP entry from Codex
configuration, then delete `~/.codex/tools/codex-safe-git`. Keep or archive the audit log according
to your local retention needs.
