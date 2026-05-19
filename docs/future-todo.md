# Future TODO

## Completed Validation

The codex-safe-git MCP has been validated in both Codex app and normal Codex CLI flows while preserving
the `workspace-write` sandbox and the narrow MCP surface.

Proven behaviours:

- The local test suite passes: compile checks plus 28 unit, integration, and MCP tests.
- Codex app can call the live `codex_safe_git` MCP tools against this linked worktree.
- A detached linked worktree can be attached to a non-default branch with `ensure_commit_branch`.
- `commit_files` can commit exact listed files on a non-default Codex worktree branch.
- Commits on protected `main` and remote-like branch names are refused.
- Codex CLI can call `git_status` and `git_diff_summary` through `codex_safe_git` with
  `--sandbox workspace-write`.
- Codex CLI can commit an exact file through `codex_safe_git` with `--sandbox workspace-write`.
- The active local MCP config uses explicit allowed repo roots for Codex worktrees and local project
  containers, not one-off per-worktree allowlists.
- The active local MCP config points at a stable wrapper under `~/.codex/tools/codex-safe-git`
  instead of a disposable Codex worktree checkout.
- Audit logs contain metadata only: action, result, repo, branch, filenames, counts, refusal
  reasons, and commit hashes where applicable.
- The server supports explicit allowed repo roots, so one MCP registration can cover Codex worktrees
  and local project containers without allowing arbitrary Git or shell.

Validation commits:

- App/worktree commit: `611dc518bb61e8b2a9cfdbaf6b5368a7994f2f04`
- App follow-up docs commits:
  - `2961674af74ac21b9f035c6464f5d3acabaa8ec5`
  - `c1564c7cb3b17466ce1c55d1bb8f751868afa833`
  - `d4c3672033f48b171b85de744c9715fe6cf230ee`
- CLI commit proof: `8a59c45059f75f1db06a274902ca8cc3c38548d6`
- Root allowlist CLI commit proof: `045df3c3cc8058bc883d7e7d9aa4de9b2abb9be5`

## Remaining Work

These are intentionally deferred beyond the proven MVP/MCP integration.

### Product Direction

Codex Safe Git should become the standard local safe-Git path for Codex app and Codex CLI across
Codex projects. It should stay local-only, deterministic, allowlisted, audited, and compatible with
the normal `workspace-write` sandbox and current permission model.

Future work should optimise for:

- clear per-machine allowed-root and audit-log configuration
- team-member installation and update workflows
- hardening suitable for eventual open-source release
- safe, best-practice local Git hygiene for Codex during long-running implementation work

### Next Engineering Work

- Add packaging/release polish for reuse outside this workspace.
- Add an operator guide for updating allowlists, audit log paths, and per-project MCP config safely.
- Choose a long-lived audit log strategy instead of `/private/tmp` if retention is desired.
- Add structured JSON schema snapshots for MCP tool outputs if external clients will depend on the
  response shape.
- Add CI coverage in an environment with Git and Python available.
- Add a team onboarding guide once the installation/update path is chosen.
- Prepare open-source readiness only after hardening: licence, security policy, contribution guide,
  threat model, reproducible tests, and clear non-goals.

### Possible Narrow Local Tools

Only local, deterministic, safety-reviewed tools should be considered. Candidate tools from the
original brief remain deferred:

- `create_worktree`
- `list_worktrees`
- a stricter `safe_checkout`

Any new tool must preserve exact allowlists, refuse ambiguous states, avoid remotes, and never become
an arbitrary Git command runner.

## Permanent Non-Goals

These are not future features:

- arbitrary Git commands
- arbitrary shell
- push, pull, fetch, reset, clean, merge, rebase, tag, force-push, branch deletion, remote mutation,
  deploys, package publishing, or PR creation
- secret, credential, token, wallet, keychain, or shell-profile access
