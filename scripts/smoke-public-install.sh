#!/usr/bin/env bash
set -euo pipefail

module_path="github.com/K1-R1/codex-safe-git"
command_path="$module_path/cmd/codex-safe-git-mcp"
ref="${1:-${CODEX_SAFE_GIT_PUBLIC_INSTALL_REF:-latest}}"
ref="${ref#@}"
go_bin="${GO:-go}"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/codex-safe-git-public-install.XXXXXX")"

cleanup() {
  if [ "${CODEX_SAFE_GIT_KEEP_PUBLIC_INSTALL_SMOKE:-0}" = "1" ]; then
    echo "Kept public install smoke directory: $tmp"
    return
  fi
  chmod -R u+w "$tmp" 2>/dev/null || true
  rm -rf "$tmp"
}
trap cleanup EXIT

export HOME="$tmp/home"
export GOBIN="$tmp/bin"
export GOPATH="$tmp/gopath"
export GOCACHE="$tmp/gocache"
export GOMODCACHE="$tmp/gomodcache"
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_TERMINAL_PROMPT=0

mkdir -p "$HOME/.codex/worktrees" "$HOME/.codex/log" "$GOBIN" "$GOPATH" "$GOCACHE" "$GOMODCACHE"

"$go_bin" install "$command_path@$ref"

binary="$GOBIN/codex-safe-git-mcp"
version_output="$("$binary" --version)"
case "$version_output" in
  "codex-safe-git "*) ;;
  *)
    echo "Unexpected version output: $version_output" >&2
    exit 1
    ;;
esac

config_output="$("$binary" --print-config)"
printf '%s\n' "$config_output" | grep -q '\[mcp_servers.codex_safe_git\]'
printf '%s\n' "$config_output" | grep -q "command = "
printf '%s\n' "$config_output" | grep -q "CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS"
printf '%s\n' "$config_output" | grep -q "CODEX_SAFE_GIT_AUDIT_LOG"

repo="$HOME/.codex/worktrees/smoke-repo"
git init -b main "$repo" >/dev/null
git -C "$repo" config user.name "Codex Safe Git Public Install Smoke"
git -C "$repo" config user.email "codex-safe-git-public-smoke@example.invalid"
printf 'smoke\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" commit -m "Initial commit" >/dev/null

output="$tmp/mcp-smoke.jsonl"
CODEX_SAFE_GIT_ALLOWED_REPOS="" \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS="$HOME/.codex/worktrees" \
CODEX_SAFE_GIT_AUDIT_LOG="$HOME/.codex/log/codex-safe-git-audit.jsonl" \
  "$binary" > "$output" <<EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"self_check","arguments":{"repo_path":"$repo"}}}
EOF

grep -q '"name":"codex-safe-git"' "$output"
grep -q '"git_status"' "$output"
grep -q '"self_check"' "$output"
grep -q '"result":"ok"' "$output"
grep -q '"server_version":"' "$output"

echo "Public install smoke test passed for $command_path@$ref ($version_output)"
