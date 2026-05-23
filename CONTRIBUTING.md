# Contributing

Thanks for helping make `codex-safe-git` safer and easier to operate.

## Development Setup

Requirements:

- Go 1.26.3, or a compatible Go installation with automatic toolchain downloads enabled
- Git
- `shasum` or `sha256sum`

Run the full local gate before proposing changes:

```sh
scripts/verify.sh
sh scripts/check-dco.sh main..HEAD
```

For smaller loops:

```sh
go test ./...
go test -race ./...
scripts/smoke-stdio.sh
```

## Change Guidelines

- Preserve the closed MCP tool surface unless a new tool has an explicit design decision.
- Keep read-only tools free of audit-log writes and other local state changes.
- Keep mutating tools narrow, auditable, and fail-closed.
- Return compact structured results with counts, limits, truncation, and redaction metadata.
- Do not add arbitrary Git arguments, shell execution, remotes, package publishing, or protected
  branch mutation.
- Add focused regression tests for new refusal paths, output bounds, and safety-sensitive edge cases.

## Developer Certificate of Origin

Every commit must include a Developer Certificate of Origin sign-off. The sign-off certifies that
the contribution can be submitted under this repository's open source licence terms. See
[DCO.md](DCO.md) for the project policy and link to the canonical DCO text.

Create signed-off commits with:

```sh
git commit -s
```

To repair the most recent local commit:

```sh
git commit --amend --signoff --no-edit
```

To repair a local branch before opening a pull request:

```sh
git rebase --signoff main
```

GitHub also requires sign-off for web-based commits in this repository. The required PR status check
is named `signed-off`; GitHub may display it as `dco / signed-off`.

## Pull Request Checklist

- The public tool contract remains intentional and documented.
- `scripts/verify.sh` passes.
- Every commit includes a `Signed-off-by` trailer.
- Docs are updated when operator behaviour, security guarantees, or MCP contracts change.
- No secrets, credentials, local personal paths, or generated artefacts are committed.

Use the pull request template in `.github/PULL_REQUEST_TEMPLATE.md` for release-risk notes and
verification details.
