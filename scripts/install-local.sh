#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
CODEX_HOME="${CODEX_HOME:-"$HOME/.codex"}"
INSTALL_DIR="${CODEX_SAFE_GIT_INSTALL_DIR:-"$CODEX_HOME/tools/codex-safe-git"}"
ALLOWED_ROOTS="${CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS:-"$CODEX_HOME/worktrees:$HOME/personal/projects"}"
AUDIT_LOG="${CODEX_SAFE_GIT_AUDIT_LOG:-"$CODEX_HOME/log/codex-safe-git-audit.jsonl"}"

if [ ! -f "$SOURCE_DIR/src/codex_safe_git/server.py" ]; then
  echo "install-local.sh must be run from the codex-safe-git source tree" >&2
  exit 1
fi

case "$INSTALL_DIR" in
  "/" | "$HOME" | "$CODEX_HOME")
    echo "Refusing unsafe install directory: $INSTALL_DIR" >&2
    exit 1
    ;;
esac

mkdir -p "$INSTALL_DIR/bin" "$(dirname "$AUDIT_LOG")"
rsync -a \
  --exclude '.git' \
  --exclude '__pycache__' \
  --exclude '*.pyc' \
  --exclude '.pytest_cache' \
  --exclude '.ruff_cache' \
  "$SOURCE_DIR"/ "$INSTALL_DIR"/

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
Installed codex-safe-git to:
  $INSTALL_DIR

Use this MCP config:

[mcp_servers.codex_safe_git]
command = "$INSTALL_DIR/bin/codex-safe-git-mcp"
enabled_tools = ["git_status", "git_diff_summary", "commit_files", "ensure_commit_branch"]
default_tools_approval_mode = "approve"
enabled = true

[mcp_servers.codex_safe_git.env]
CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "$ALLOWED_ROOTS"
CODEX_SAFE_GIT_AUDIT_LOG = "$AUDIT_LOG"
EOF
