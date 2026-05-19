from __future__ import annotations

import json
import os
import subprocess
import tempfile
from pathlib import Path

import unittest


PROJECT_ROOT = Path(__file__).resolve().parents[2]
INSTALLER = PROJECT_ROOT / "scripts" / "install-local.sh"


class CodexSafeGitInstallerTests(unittest.TestCase):
    def test_dry_run_prints_stable_config_without_installing(self) -> None:
        with tempfile.TemporaryDirectory(prefix="codex-safe-git-installer-") as raw:
            root = Path(raw)
            install_dir = root / "install"
            env = self._installer_env(root, install_dir)

            result = subprocess.run(
                ["bash", str(INSTALLER), "--dry-run"],
                cwd=PROJECT_ROOT,
                env=env,
                text=True,
                capture_output=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("Dry run: would install codex-safe-git", result.stdout)
            self.assertIn(f'command = "{install_dir}/bin/codex-safe-git-mcp"', result.stdout)
            self.assertIn("create_commit_branch", result.stdout)
            self.assertFalse(install_dir.exists())

    def test_install_is_idempotent_and_wrapper_runs_without_source_cwd(self) -> None:
        with tempfile.TemporaryDirectory(prefix="codex-safe-git-installer-") as raw:
            root = Path(raw)
            install_dir = root / "install"
            env = self._installer_env(root, install_dir)

            for _ in range(2):
                result = subprocess.run(
                    ["bash", str(INSTALLER)],
                    cwd=PROJECT_ROOT,
                    env=env,
                    text=True,
                    capture_output=True,
                )
                self.assertEqual(result.returncode, 0, result.stderr)

            wrapper = install_dir / "bin" / "codex-safe-git-mcp"
            self.assertTrue(wrapper.exists())
            self.assertTrue(os.access(wrapper, os.X_OK))
            proc = subprocess.run(
                [str(wrapper)],
                cwd=root,
                env=env,
                input=json.dumps({"jsonrpc": "2.0", "id": 1, "method": "tools/list"}) + "\n",
                text=True,
                capture_output=True,
            )

            self.assertEqual(proc.returncode, 0, proc.stderr)
            response = json.loads(proc.stdout)
            names = [tool["name"] for tool in response["result"]["tools"]]
            self.assertIn("merge_branch", names)

    def test_refuses_unsafe_install_directory(self) -> None:
        with tempfile.TemporaryDirectory(prefix="codex-safe-git-installer-") as raw:
            root = Path(raw)
            env = self._installer_env(root, root)

            result = subprocess.run(
                ["bash", str(INSTALLER)],
                cwd=PROJECT_ROOT,
                env=env,
                text=True,
                capture_output=True,
            )

            self.assertNotEqual(result.returncode, 0)
            self.assertIn("Refusing unsafe install directory", result.stderr)

    def _installer_env(self, root: Path, install_dir: Path) -> dict[str, str]:
        env = os.environ.copy()
        env["CODEX_HOME"] = str(root)
        env["CODEX_SAFE_GIT_INSTALL_DIR"] = str(install_dir)
        env["CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS"] = str(root / "worktrees")
        env["CODEX_SAFE_GIT_AUDIT_LOG"] = str(root / "log" / "audit.jsonl")
        return env
