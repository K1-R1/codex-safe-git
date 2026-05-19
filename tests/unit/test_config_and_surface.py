from __future__ import annotations

from codex_safe_git.core import CodexSafeGitConfig, CodexSafeGitRefusal
from codex_safe_git.server import handle_request
from tests.helpers import GitRepoTestCase


class CodexSafeGitConfigAndSurfaceTests(GitRepoTestCase):
    def test_env_config_is_required(self) -> None:
        with self.assertRaisesRegex(CodexSafeGitRefusal, "ALLOWED_REPOS"):
            CodexSafeGitConfig.from_env({})

    def test_mcp_surface_is_narrow(self) -> None:
        listed = handle_request({"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
        names = [tool["name"] for tool in listed["result"]["tools"]]
        self.assertEqual(
            names,
            ["git_status", "git_diff_summary", "commit_files", "ensure_commit_branch"],
        )
