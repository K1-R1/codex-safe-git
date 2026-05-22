#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT"

go_bin="${GO:-go}"
tmp="$(mktemp -d "${TMPDIR:-/tmp}/codex-safe-git-smoke.XXXXXX")"
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_TERMINAL_PROMPT=0
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

allowed_root="$tmp/allowed"
repo="$allowed_root/repo"
audit_log="$tmp/audit/codex-safe-git-audit.jsonl"
binary="$tmp/codex-safe-git-mcp"
linked="$allowed_root/linked"

mkdir -p "$allowed_root"
git init -b main "$repo" >/dev/null
git -C "$repo" config user.name "Codex Safe Git Smoke"
git -C "$repo" config user.email "codex-safe-git-smoke@example.invalid"
printf 'initial\n' > "$repo/README.md"
git -C "$repo" add README.md
git -C "$repo" commit -m "Initial commit" >/dev/null
git -C "$repo" switch -c work >/dev/null 2>&1
git -C "$repo" branch codex/smoke-checkout

"$go_bin" build -trimpath -o "$binary" ./cmd/codex-safe-git-mcp

allowed_output="$tmp/allowed.jsonl"
CODEX_SAFE_GIT_ALLOWED_REPOS="" \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS="$allowed_root" \
CODEX_SAFE_GIT_AUDIT_LOG="$audit_log" \
  "$binary" > "$allowed_output" <<EOF
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"create_worktree","arguments":{"repo_path":"$repo","worktree_path":"$linked","branch_name":"codex/smoke-worktree"}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"list_worktrees","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"safe_checkout","arguments":{"repo_path":"$linked","branch_name":"codex/smoke-checkout"}}}
{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_local_branches","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"compare_refs","arguments":{"repo_path":"$repo","base_ref":"work","target_ref":"codex/smoke-checkout"}}}
{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"commit_log_summary","arguments":{"repo_path":"$repo","limit":1}}}
{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"show_commit_summary","arguments":{"repo_path":"$repo","commit_ref":"HEAD"}}}
{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"list_local_refs","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"merge_base","arguments":{"repo_path":"$repo","left_ref":"work","right_ref":"codex/smoke-checkout"}}}
{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"changed_files_between_refs","arguments":{"repo_path":"$repo","base_ref":"work","target_ref":"codex/smoke-checkout"}}}
{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"path_status","arguments":{"repo_path":"$repo","paths":["README.md"]}}}
{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"submodule_summary","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"repository_integrity_check","arguments":{"repo_path":"$repo"}}}
{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"reflog_summary","arguments":{"repo_path":"$repo","limit":1}}}
{"jsonrpc":"2.0","id":18,"method":"tools/call","params":{"name":"self_check","arguments":{"repo_path":"$repo"}}}
EOF

grep -q '"name":"codex-safe-git"' "$allowed_output"
grep -q '"git_status"' "$allowed_output"
grep -q '"list_worktrees"' "$allowed_output"
grep -q '"create_worktree"' "$allowed_output"
grep -q '"safe_checkout"' "$allowed_output"
grep -q '"list_local_branches"' "$allowed_output"
grep -q '"repository_integrity_check"' "$allowed_output"
grep -q '"self_check"' "$allowed_output"
grep -q '"result":"ok"' "$allowed_output"
grep -q '"action":"created"' "$allowed_output"
grep -q '"action":"switched"' "$allowed_output"

outside="$tmp/outside"
mkdir "$outside"
refusal_output="$tmp/refusal.jsonl"
CODEX_SAFE_GIT_ALLOWED_REPOS="" \
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS="$allowed_root" \
CODEX_SAFE_GIT_AUDIT_LOG="$audit_log" \
  "$binary" > "$refusal_output" <<EOF
{"jsonrpc":"2.0","id":19,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":"$outside"}}}
EOF

grep -q '"result":"refused"' "$refusal_output"
grep -q 'not explicitly allowlisted' "$refusal_output"

echo "Direct stdio smoke test passed"
