#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT"

go_bin="${GO:-go}"
if [ -z "${GOCACHE:-}" ]; then
  export GOCACHE="${TMPDIR:-/tmp}/codex-safe-git-go-build-cache"
fi
if [ "$GOCACHE" != "off" ]; then
  mkdir -p "$GOCACHE"
fi
if [ -z "${GOMODCACHE:-}" ]; then
  export GOMODCACHE="${TMPDIR:-/tmp}/codex-safe-git-go-mod-cache"
fi
mkdir -p "$GOMODCACHE"

"$go_bin" fmt ./...
"$go_bin" vet ./...
"$go_bin" test ./...
"$go_bin" test -cover ./...
"$go_bin" test -race ./...

scripts/install-local.sh --dry-run >/dev/null
scripts/install-local.sh --print-config >/dev/null

tmp="$(mktemp -d "${TMPDIR:-/tmp}/codex-safe-git-verify.XXXXXX")"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

CODEX_HOME="$tmp/.codex" \
CODEX_SAFE_GIT_AUDIT_LOG="$tmp/.codex/log/codex-safe-git-audit.jsonl" \
  scripts/install-local.sh >/dev/null
test -d "$tmp/.codex/worktrees"

CODEX_HOME="$tmp/.codex" scripts/install-local.sh --verify-install >/dev/null
scripts/smoke-stdio.sh

dco_repo="$tmp/dco-repo"
empty_hooks="$tmp/empty-hooks"
mkdir -p "$empty_hooks"
git init -b main "$dco_repo" >/dev/null
git -C "$dco_repo" config core.hooksPath "$empty_hooks"
git -C "$dco_repo" config commit.gpgsign false
git -C "$dco_repo" config tag.gpgsign false
git -C "$dco_repo" config user.name "Codex Safe Git Verify"
git -C "$dco_repo" config user.email "codex-safe-git-verify@example.invalid"
printf 'initial\n' > "$dco_repo/README.md"
git -C "$dco_repo" add README.md
git -C "$dco_repo" commit -s -m "Initial commit" >/dev/null
git -C "$dco_repo" switch -c codex/dco-smoke >/dev/null 2>&1
printf 'change\n' > "$dco_repo/change.txt"
git -C "$dco_repo" add change.txt
git -C "$dco_repo" commit -s -m "Add DCO smoke change" >/dev/null
(cd "$dco_repo" && sh "$ROOT/scripts/check-dco.sh" >/dev/null)
