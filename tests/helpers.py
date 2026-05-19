from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path

from codex_safe_git.core import CodexSafeGit, CodexSafeGitConfig


def run(command: list[str], cwd: Path | None = None) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(command, cwd=cwd, text=True, capture_output=True)
    if result.returncode != 0:
        raise AssertionError(result.stderr or result.stdout)
    return result


class GitRepoTestCase(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory(prefix="codex-safe-git-test-")
        self.root = Path(self.tmp.name)
        self.repo = self.root / "repo"
        self.audit_log = self.root / "audit.jsonl"
        run(["git", "init", "--initial-branch=main", str(self.repo)])
        run(["git", "config", "user.name", "Local Tester"], self.repo)
        run(["git", "config", "user.email", "local-tester@example.invalid"], self.repo)
        self.write_file(".baseline", "baseline\n")
        run(["git", "add", ".baseline"], self.repo)
        run(["git", "commit", "-m", "Initial commit"], self.repo)
        run(["git", "switch", "-c", "work"], self.repo)
        self.codex_safe_git = CodexSafeGit(
            CodexSafeGitConfig(allowed_repos=frozenset({self.repo.resolve()}), audit_log=self.audit_log)
        )

    def tearDown(self) -> None:
        self.tmp.cleanup()

    def write_file(self, rel: str, text: str) -> None:
        path = self.repo / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def write_file_in(self, repo: Path, rel: str, text: str) -> None:
        path = repo / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")

    def audit_entries(self) -> list[dict[str, object]]:
        return [
            json.loads(line)
            for line in self.audit_log.read_text(encoding="utf-8").splitlines()
            if line.strip()
        ]
