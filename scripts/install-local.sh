#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
CODEX_HOME="${CODEX_HOME:-"$HOME/.codex"}"
INSTALL_DIR="${CODEX_SAFE_GIT_INSTALL_DIR:-"$CODEX_HOME/tools/codex-safe-git-go"}"
ALLOWED_ROOTS="${CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS:-"$CODEX_HOME/worktrees:$HOME/personal/projects"}"
AUDIT_LOG="${CODEX_SAFE_GIT_AUDIT_LOG:-"$CODEX_HOME/log/codex-safe-git-audit.jsonl"}"
PROTECTED_BRANCHES="${CODEX_SAFE_GIT_PROTECTED_BRANCHES:-}"
VERSION="$(awk '/const ServerVersion = / { gsub("\"", "", $4); print $4; exit }' "$SOURCE_DIR/internal/mcp/server.go" 2>/dev/null || true)"
GO_BIN="${GO:-go}"
ALLOW_EXTERNAL_INSTALL_DIR="${CODEX_SAFE_GIT_ALLOW_EXTERNAL_INSTALL_DIR:-0}"
DRY_RUN=0
PRINT_CONFIG=0

usage() {
  cat <<EOF
Usage: scripts/install-local.sh [--dry-run] [--print-config] [--version] [--help]

Builds and installs the Go codex-safe-git MCP binary to a stable local Codex tool directory.

Environment overrides:
  CODEX_HOME                         Default: $HOME/.codex
  CODEX_SAFE_GIT_INSTALL_DIR          Default: \$CODEX_HOME/tools/codex-safe-git-go
  CODEX_SAFE_GIT_ALLOW_EXTERNAL_INSTALL_DIR
                                     Set to 1 to allow install dirs outside \$CODEX_HOME/tools
  CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS   Default: \$CODEX_HOME/worktrees:\$HOME/personal/projects
  CODEX_SAFE_GIT_AUDIT_LOG            Default: \$CODEX_HOME/log/codex-safe-git-audit.jsonl
  CODEX_SAFE_GIT_PROTECTED_BRANCHES    Optional comma-separated extra protected branch names
  GO                                  Default: go
EOF
}

canonical_path() {
  local path="$1"
  local dir
  local suffix

  if [ -e "$path" ]; then
    if [ -d "$path" ]; then
      (cd "$path" && pwd -P)
    else
      dir="$(dirname "$path")"
      suffix="$(basename "$path")"
      printf '%s/%s\n' "$(cd "$dir" && pwd -P)" "$suffix"
    fi
    return
  fi

  dir="$(dirname "$path")"
  suffix="$(basename "$path")"
  while [ ! -d "$dir" ]; do
    suffix="$(basename "$dir")/$suffix"
    dir="$(dirname "$dir")"
  done
  printf '%s/%s\n' "$(cd "$dir" && pwd -P)" "$suffix"
}

path_within() {
  local path="$1"
  local root="$2"
  [ "$path" = "$root" ] || [[ "$path" == "$root"/* ]]
}

has_parent_segment() {
  local path="$1"
  [[ "$path" == ".." || "$path" == "../"* || "$path" == *"/.." || "$path" == *"/../"* ]]
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1
      ;;
    --print-config)
      PRINT_CONFIG=1
      ;;
    --version)
      echo "codex-safe-git $VERSION"
      exit 0
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [ ! -f "$SOURCE_DIR/cmd/codex-safe-git-mcp/main.go" ]; then
  echo "install-local.sh must be run from the codex-safe-git source tree" >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  echo "Could not determine codex-safe-git version" >&2
  exit 1
fi

if has_parent_segment "$INSTALL_DIR"; then
  echo "Refusing unsafe install directory: $INSTALL_DIR" >&2
  exit 1
fi

if has_parent_segment "$AUDIT_LOG"; then
  echo "Refusing unsafe audit log path: $AUDIT_LOG" >&2
  exit 1
fi

case "$INSTALL_DIR" in
  "/" | "$HOME" | "$CODEX_HOME")
    echo "Refusing unsafe install directory: $INSTALL_DIR" >&2
    exit 1
    ;;
esac

case "$AUDIT_LOG" in
  "/" | "$HOME" | "$CODEX_HOME")
    echo "Refusing unsafe audit log path: $AUDIT_LOG" >&2
    exit 1
    ;;
esac

CODEX_HOME="$(canonical_path "$CODEX_HOME")"
INSTALL_DIR="$(canonical_path "$INSTALL_DIR")"
AUDIT_LOG="$(canonical_path "$AUDIT_LOG")"
TOOLS_DIR="$CODEX_HOME/tools"

case "$INSTALL_DIR" in
  "/" | "$HOME" | "$CODEX_HOME" | "$TOOLS_DIR")
    echo "Refusing unsafe install directory: $INSTALL_DIR" >&2
    exit 1
    ;;
esac

if ! path_within "$INSTALL_DIR" "$TOOLS_DIR" && [ "$ALLOW_EXTERNAL_INSTALL_DIR" != "1" ]; then
  echo "Refusing install directory outside $TOOLS_DIR without CODEX_SAFE_GIT_ALLOW_EXTERNAL_INSTALL_DIR=1: $INSTALL_DIR" >&2
  exit 1
fi

case "$AUDIT_LOG" in
  "/" | "$HOME" | "$CODEX_HOME")
    echo "Refusing unsafe audit log path: $AUDIT_LOG" >&2
    exit 1
    ;;
esac

print_config() {
  cat <<EOF
[mcp_servers.codex_safe_git]
command = "$INSTALL_DIR/bin/codex-safe-git-mcp"
enabled_tools = ["git_status", "git_diff_summary", "commit_files", "ensure_commit_branch", "create_commit_branch", "merge_branch"]
default_tools_approval_mode = "approve"
enabled = true

[mcp_servers.codex_safe_git.env]
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "$ALLOWED_ROOTS"
CODEX_SAFE_GIT_AUDIT_LOG = "$AUDIT_LOG"
EOF
  if [ -n "$PROTECTED_BRANCHES" ]; then
    printf 'CODEX_SAFE_GIT_PROTECTED_BRANCHES = "%s"\n' "$PROTECTED_BRANCHES"
  fi
}

if [ "$PRINT_CONFIG" -eq 1 ]; then
  print_config
  exit 0
fi

if [ "$DRY_RUN" -eq 1 ]; then
  cat <<EOF
Dry run: would install codex-safe-git $VERSION
  source:        $SOURCE_DIR
  install dir:   $INSTALL_DIR
  binary:        $INSTALL_DIR/bin/codex-safe-git-mcp
  allowed roots: $ALLOWED_ROOTS
  audit log:     $AUDIT_LOG
  protected:     ${PROTECTED_BRANCHES:-main/master plus repo default only}

MCP config that would be used:

EOF
  print_config
  exit 0
fi

mkdir -p "$INSTALL_DIR/bin" "$(dirname "$AUDIT_LOG")"
"$GO_BIN" build -trimpath -o "$INSTALL_DIR/bin/codex-safe-git-mcp" ./cmd/codex-safe-git-mcp
chmod 0755 "$INSTALL_DIR/bin/codex-safe-git-mcp"

cat <<EOF
Installed codex-safe-git $VERSION to:
  $INSTALL_DIR

Use this MCP config:

EOF
print_config
