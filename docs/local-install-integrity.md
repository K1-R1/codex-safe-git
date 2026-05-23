# Local Install Integrity

`codex-safe-git` supports source-tree installation through `scripts/install-local.sh`. Public command
installation should use `go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@version`
after the GitHub repository is published. Package-manager distribution and release signing remain
deferred.

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

The checksum proves local file integrity against the sidecar created at install time. It is not a
public release signature and does not prove authorship. Public `go install` users rely on the Go
module download and checksum mechanisms plus the selected Git tag.

## Moving Builds

If you manually move a local build between machines, copy both the binary and `.sha256` file. Verify
the checksum before configuring Codex to use the binary.

Codex must not create, read, import, or operate private signing keys unless the operator explicitly
approves that exact action in a separate high-trust workflow.

## Rollback

Keep the previously verified local binary and checksum until the replacement has passed local
validation. To roll back, point the MCP config at the previous verified install directory or
reinstall from the previous validated build.
