#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
CODEX_HOME="${CODEX_HOME:-"$HOME/.codex"}"
INSTALL_DIR="${CODEX_SAFE_GIT_INSTALL_DIR:-"$CODEX_HOME/tools/codex-safe-git"}"
ALLOWED_ROOTS="${CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS:-"$CODEX_HOME/worktrees:$HOME/personal/projects"}"
AUDIT_LOG="${CODEX_SAFE_GIT_AUDIT_LOG:-"$CODEX_HOME/log/codex-safe-git-audit.jsonl"}"
VERSION="$(awk -F '"' '/^version = / { print $2; exit }' "$SOURCE_DIR/pyproject.toml" 2>/dev/null || true)"
DRY_RUN=0
PRINT_CONFIG=0

usage() {
  cat <<EOF
Usage: scripts/install-local.sh [--dry-run] [--print-config] [--version] [--help]

Installs codex-safe-git to a stable local Codex tool directory.

Environment overrides:
  CODEX_HOME                         Default: $HOME/.codex
  CODEX_SAFE_GIT_INSTALL_DIR          Default: \$CODEX_HOME/tools/codex-safe-git
  CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS   Default: \$CODEX_HOME/worktrees:\$HOME/personal/projects
  CODEX_SAFE_GIT_AUDIT_LOG            Default: \$CODEX_HOME/log/codex-safe-git-audit.jsonl
EOF
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

if [ ! -f "$SOURCE_DIR/src/codex_safe_git/server.py" ]; then
  echo "install-local.sh must be run from the codex-safe-git source tree" >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  echo "Could not determine codex-safe-git version from pyproject.toml" >&2
  exit 1
fi

if [ -z "$ALLOWED_ROOTS" ]; then
  echo "CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS must not be empty" >&2
  exit 1
fi

if [ -z "$AUDIT_LOG" ]; then
  echo "CODEX_SAFE_GIT_AUDIT_LOG must not be empty" >&2
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
  wrapper:       $INSTALL_DIR/bin/codex-safe-git-mcp
  allowed roots: $ALLOWED_ROOTS
  audit log:     $AUDIT_LOG

MCP config that would be used:

EOF
  print_config
  exit 0
fi

mkdir -p "$INSTALL_DIR" "$(dirname "$AUDIT_LOG")"
rsync -a --delete \
  --exclude '.git' \
  --exclude 'bin' \
  --exclude '__pycache__' \
  --exclude '*.pyc' \
  --exclude '.pytest_cache' \
  --exclude '.ruff_cache' \
  "$SOURCE_DIR"/ "$INSTALL_DIR"/

mkdir -p "$INSTALL_DIR/bin"
cat > "$INSTALL_DIR/bin/codex-safe-git-mcp" <<'PY'
#!/usr/bin/env python3
from __future__ import annotations

import runpy
import sys
from pathlib import Path

root = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(root / "src"))
runpy.run_module("codex_safe_git", run_name="__main__")
PY
chmod 0755 "$INSTALL_DIR/bin/codex-safe-git-mcp"

cat <<EOF
Installed codex-safe-git $VERSION to:
  $INSTALL_DIR

Use this MCP config:

EOF
print_config
