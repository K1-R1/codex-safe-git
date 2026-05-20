# Private Binary Distribution

`codex-safe-git` is ready for private or team-local binary distribution. Public release and
open-source release preparation remain deferred.

## Integrity

The installer writes and verifies:

```text
~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp
~/.codex/tools/codex-safe-git-go/bin/codex-safe-git-mcp.sha256
```

Verify an installed binary with:

```sh
scripts/install-local.sh --verify-install
```

When copying a private build to another machine, copy both the binary and `.sha256` file. The
receiving operator should verify the checksum before configuring Codex to use the binary.

## Signing Strategy

Checksum verification proves integrity, not authorship. For team distribution, use detached
signatures created by a human operator with a team-managed signing key outside Codex.

Recommended private signing flow:

1. Build and verify locally with `scripts/verify.sh`.
2. Install with `scripts/install-local.sh`.
3. Verify with `scripts/install-local.sh --verify-install`.
4. Produce a detached signature for the binary and checksum file using the team's existing signing
   process.
5. Distribute the binary, checksum, detached signature, version, commit hash, and validation evidence
   together.
6. Verify the signature and checksum on the receiving machine before updating MCP config.

Codex must not create, read, import, or operate private signing keys unless the operator explicitly
approves that exact action in a separate high-trust workflow.

## Rollback

Keep the previously verified private binary and checksum until the replacement has passed local
validation. To roll back, point the MCP config at the previous verified install directory or reinstall
from the previous validated build.
