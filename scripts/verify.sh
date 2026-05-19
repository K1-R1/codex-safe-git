#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
PYCACHE_PREFIX="${PYTHONPYCACHEPREFIX:-/private/tmp/codex-safe-git-pycache}"

cd "$ROOT"

bash -n scripts/install-local.sh
PYTHONPYCACHEPREFIX="$PYCACHE_PREFIX" python3 -m compileall -q src tests
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src python3 -m unittest discover -s tests -v
