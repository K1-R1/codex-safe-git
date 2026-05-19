from __future__ import annotations

import os

from codex_safe_git.core import CodexSafeGitConfig, CodexSafeGitRefusal
from codex_safe_git.server import handle_request
from tests.helpers import GitRepoTestCase


class CodexSafeGitConfigAndSurfaceTests(GitRepoTestCase):
    def test_env_config_is_required(self) -> None:
        with self.assertRaisesRegex(CodexSafeGitRefusal, "ALLOWED_REPOS"):
            CodexSafeGitConfig.from_env({})

    def test_env_config_supports_allowed_repo_roots(self) -> None:
        config = CodexSafeGitConfig.from_env(
            {
                "CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS": str(self.root),
                "CODEX_SAFE_GIT_AUDIT_LOG": str(self.audit_log),
            }
        )

        self.assertEqual(config.allowed_repos, frozenset())
        self.assertEqual(config.allowed_repo_roots, frozenset({self.root.resolve()}))

    def test_env_config_rejects_filesystem_root_as_allowed_repo_root(self) -> None:
        with self.assertRaisesRegex(CodexSafeGitRefusal, "filesystem root"):
            CodexSafeGitConfig.from_env(
                {
                    "CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS": os.path.abspath(os.sep),
                    "CODEX_SAFE_GIT_AUDIT_LOG": str(self.audit_log),
                }
            )

    def test_mcp_surface_is_narrow(self) -> None:
        listed = handle_request({"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
        names = [tool["name"] for tool in listed["result"]["tools"]]
        self.assertEqual(
            names,
            [
                "git_status",
                "git_diff_summary",
                "commit_files",
                "ensure_commit_branch",
                "create_commit_branch",
                "merge_branch",
            ],
        )
