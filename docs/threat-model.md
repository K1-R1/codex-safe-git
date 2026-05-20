# Private Threat Model

## Assets

- Developer source worktrees under configured allowed roots.
- Local Git branch history in those worktrees.
- Audit log metadata.
- Codex App and CLI MCP configuration.

## Trust Boundaries

- Codex prompts and repository content are untrusted inputs.
- MCP config and allowed roots are trusted local operator configuration.
- The local Git executable and operating system user account are trusted dependencies.
- The audit log is local metadata and should remain user-private.

## Primary Threats

- Prompt injection causing Codex to request unsafe Git operations.
- Over-broad allowed roots exposing repos that were not intended for Codex-managed commits.
- Malicious Git configuration attempting to execute filters, diff drivers, hooks, pagers, prompts, or
  credential helpers.
- PATH hijack of the `git` binary.
- Local races where files change between safety checks and commit.
- Accidental secret commit through path or content scanner gaps.
- Audit metadata revealing sensitive project structure.
- MCP config tampering that points Codex to a broader tool or looser environment.

## Mitigations

- Expose only six fixed MCP tools.
- Require exact worktree roots and explicit allowlists/allowed roots.
- Refuse default-branch mutations.
- Refuse ambiguous states and pre-staged changes.
- Disable hooks, signing, pager, editor, prompts, and credential helpers for Git subprocesses.
- Validate the resolved `git` executable before use.
- Stage exact requested files and verify the staged set.
- Refuse likely secret paths and likely secret material.
- Write metadata-only audit entries.

## Accepted Residual Risk

- A local attacker with the same user account can race filesystem changes or tamper with MCP config.
- Secret detection is heuristic and not a complete DLP system.
- A bad but policy-compliant commit can still be created on a non-default local branch.

