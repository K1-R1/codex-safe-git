# Distribution

This project should be distributed as a Go command first, with source-tree installation kept for
maintainers and contributors.

## Recommended Public Install

After the GitHub repository is public, the primary install command should be:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@latest
```

For a pinned install, use a release tag:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@v0.4.2
```

`go install ...@version` is the right default for a dependency-free Go command because it builds the
published module directly and does not require users to clone the repository or run an installer
script from the internet.

The installed command is written to `GOBIN` when set, otherwise to `$(go env GOPATH)/bin`. If that
directory is not on `PATH`, run the command by absolute path:

```sh
bin_dir="$(go env GOBIN)"
if [ -z "$bin_dir" ]; then
  bin_dir="$(go env GOPATH)/bin"
fi
"$bin_dir/codex-safe-git-mcp" --print-config
```

Create the default local directories before enabling the printed MCP config:

```sh
mkdir -p "$HOME/.codex/worktrees" "$HOME/.codex/log"
```

Then add the printed TOML block to the relevant Codex App or Codex CLI config and reload Codex.

## Source-Tree Install

Use source-tree installation when developing this repository, validating a release candidate, or
installing from a private checkout before publication:

```sh
git clone https://github.com/K1-R1/codex-safe-git
cd codex-safe-git
scripts/verify.sh
scripts/install-local.sh
scripts/install-local.sh --verify-install
```

The source installer builds a stable binary under `~/.codex/tools/codex-safe-git-go`, writes a
sidecar checksum, creates the default `~/.codex/worktrees` root, and prints the Codex MCP config.

## Not Recommended Initially

Do not use a `curl | sh` installer for the initial public release. This tool exists to narrow Codex's
local Git mutation surface, so asking users to pipe a remote shell script into `sh` would send the
wrong trust signal. A reviewed source checkout or `go install ...@version` is clearer and easier to
audit.

Do not publish package-manager distribution in the initial release. Homebrew can be added later via a
separate tap if there is real demand, but it adds another repository, formula review surface, and
release automation to maintain.

Do not publish binary GitHub Release assets until signing, checksums, provenance, and support policy
are explicitly decided.

## Release Tags

For public use, prefer semver tags such as `v0.4.2`. `@latest` works best when it resolves to a
deliberate release tag instead of an arbitrary default-branch commit.

The local MCP `serverInfo.version` should match the intended public release tag unless the release is
only a documentation/setup change.
