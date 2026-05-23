# Release Process

This project ships source-first Go command releases. The public install contract is:

```sh
go install github.com/K1-R1/codex-safe-git/cmd/codex-safe-git-mcp@vX.Y.Z
```

Release tags must be semantic versions such as `v0.4.2`, and the tag should match
`internal/mcp/server.go` unless the release contains only documentation or repository setup changes.

## Release Policy

- Publish source releases only by default.
- Do not attach binary release assets until signing, checksums, provenance, and support policy are
  explicitly decided.
- Do not add `curl | sh`, Homebrew, package registries, or release automation without a separate
  maintainer decision.
- Prefer signed annotated tags when the maintainer already has signing configured. Otherwise use an
  annotated tag and document that the release tag is unsigned.
- Codex must not create, read, import, or operate private signing keys.

## Prepare A Release

1. Merge all release-candidate pull requests into `main`.
2. Fast-forward local `main` from `origin/main`.
3. Confirm the version in `internal/mcp/server.go`.
4. Confirm `CHANGELOG.md` has the release notes under the target version.
5. Run the local gates:

```sh
scripts/verify.sh
```

For a release-candidate pull request, also run `sh scripts/check-dco.sh main..HEAD` before pushing or
confirm that the required `signed-off` PR check has passed.

For a public install smoke test before tagging, test the current remote commit:

```sh
scripts/smoke-public-install.sh "$(git rev-parse origin/main)"
```

## Tag And Publish

Create the release tag on the exact commit that passed verification:

```sh
git tag -a v0.4.2 -m "codex-safe-git v0.4.2"
git push origin v0.4.2
```

If maintainer signing is already configured, use `git tag -s` instead of `git tag -a`.

Create the GitHub release from the pushed tag. Use GitHub's generated release notes, then review and
edit them before publishing. Label pull requests before release generation so changes land in the
intended release-note categories. If binary assets are ever added and release immutability is
enabled, create a draft first, attach assets, and publish only after the draft is complete.

After publishing, verify the user-facing install path:

```sh
scripts/smoke-public-install.sh v0.4.2
```

If the Go module proxy has not observed the new tag yet, retry after a short delay. For maintainer
validation only, `GOPROXY=direct scripts/smoke-public-install.sh v0.4.2` can confirm the tag before
the public proxy catches up.

## Security Releases

If a release fixes a vulnerability, use GitHub repository security advisories and private
vulnerability reporting. Publish the advisory when the fix is released so affected users can receive
the appropriate security signal.
