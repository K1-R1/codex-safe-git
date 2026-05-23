# Repository Instructions

## Project Profile

`codex-safe-git` is a local-first Go MCP server that exposes a narrow, auditable Git tool surface
for Codex. Treat it as safety-sensitive infrastructure: small changes, explicit refusal paths,
bounded outputs, and strong tests are more important than feature breadth.

## Working Rules

- Use Go 1.26 or newer. The local reference toolchain is Go 1.26.3.
- Keep the module dependency-free unless there is a clear, reviewed reason to add a dependency.
- Prefer `codex_safe_git` tools for local Git status, diff summaries, branch preparation, worktrees,
  and exact-file commits. Use shell Git only for read-only detail that the MCP does not expose.
- Do not run remote Git operations, package publishing, deploys, release tagging, destructive Git
  commands, or cloud/external mutations unless the user explicitly asks for that exact action.
- Do not read or commit secrets, credentials, private keys, shell profiles, keychains, wallets,
  `.env` files, audit logs, or generated local install artefacts.
- Preserve user work. Never revert unrelated changes.

## Implementation Standards

- Preserve the closed MCP tool surface. New tools require an explicit product decision, schema
  updates, docs, golden snapshots, and safety regression tests.
- Keep read-only tools free of intentional local writes, including audit-log writes.
- Keep mutating tools fail-closed, audited before Git state changes where applicable, and scoped to
  exact local branches, worktrees, or file lists.
- Do not expose patch text, blob contents, arbitrary object data, unbounded Git output, hidden
  allowlist paths, or secret-bearing paths through tool results.
- Keep Git subprocess calls fixed-argument and non-interactive. Hooks, pagers, credential helpers,
  editors, prompts, and signing must remain disabled.
- Update operator docs whenever behaviour, configuration, safety guarantees, or MCP contract fields
  change.

## Verification

Use the smallest useful loop while developing:

```sh
go test ./...
```

Before committing a completed change, run:

```sh
scripts/verify.sh
```

For installer or MCP protocol changes, also run:

```sh
scripts/install-local.sh --dry-run
scripts/smoke-stdio.sh
```

If a command fails because of sandbox or network restrictions, ask for approval rather than working
around the configured safety rails.
