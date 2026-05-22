# Private Threat Model

## Assets

- Developer source worktrees under configured allowed roots.
- Local Git branch history in those worktrees.
- Audit log metadata.
- Codex App and CLI MCP configuration.
- Local linked worktree paths under configured allowed roots.

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
- Git pathspec magic or filesystem symlinks expanding a requested file list beyond the user's exact
  intent.
- Local races where files change between safety checks and commit.
- Worktree creation into an unintended path, overlapping repository, symlinked parent, or
  secret-bearing local directory.
- Checkout to a protected, remote-like, hash-like, or branch-already-checked-out target.
- Accidental secret commit through path or content scanner gaps.
- Audit log path misconfiguration causing mutations to happen without durable local metadata.
- Audit metadata revealing sensitive project structure.
- MCP config tampering that points Codex to a broader tool or looser environment.

## Mitigations

- Expose only the closed fixed MCP tool surface documented in the MCP contract.
- Require exact worktree roots and explicit allowlists/allowed roots.
- Refuse default-branch mutations.
- Refuse ambiguous states and pre-staged changes.
- Disable hooks, signing, pager, editor, prompts, and credential helpers for Git subprocesses.
- Validate the resolved `git` executable before use and prefer an explicit trusted
  `CODEX_SAFE_GIT_GIT_PATH` when the operator cannot rely on the default trusted path set.
- Stage exact requested files and verify the staged set.
- Force literal Git pathspec handling and reject symlinked requested paths.
- Require new worktree paths to be under configured allowed roots, non-existent, non-overlapping,
  and outside secret-bearing path components.
- Redact worktree paths that are outside configured allowlists.
- Bound read-only ref, history, path, submodule, integrity, reflog, and self-check outputs and omit
  patch text, blob contents, raw object dumps, and hidden allowlist paths.
- Keep MCP read-only tools free of intentional audit-log writes or other local state changes.
- Refuse checkout to protected branches, remote/ref/hash-like branch names, dirty worktrees, and
  branches already checked out elsewhere.
- Refuse likely secret paths and likely secret material before staging, then rescan the staged diff
  before committing.
- Write metadata-only audit entries.
- Check audit writability before mutating Git state.

## Accepted Residual Risk

- A local attacker with the same user account can race filesystem changes or tamper with MCP config.
- Secret detection is heuristic and not a complete DLP system.
- A bad but policy-compliant commit can still be created on a non-default local branch.
- Local checksum verification proves binary integrity, not authorship; private signing remains an
  operator-controlled process outside Codex.
