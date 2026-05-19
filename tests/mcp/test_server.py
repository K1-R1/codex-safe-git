from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

from codex_safe_git.server import handle_request
from tests.helpers import GitRepoTestCase, run


class CodexSafeGitMcpTests(GitRepoTestCase):
    def test_tool_call_returns_structured_content(self) -> None:
        self.write_file("mcp.txt", "mcp\n")
        response = handle_request(
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": "commit_files",
                    "arguments": {
                        "repo_path": str(self.repo),
                        "files": ["mcp.txt"],
                        "message": "Add MCP file",
                    },
                },
            },
            self.codex_safe_git,
        )

        result = response["result"]
        self.assertEqual(result["structuredContent"]["result"], "committed")
        self.assertEqual(result["structuredContent"]["files"], ["mcp.txt"])
        self.assertEqual(json.loads(result["content"][0]["text"])["result"], "committed")

    def test_tool_call_rejects_unexpected_arguments(self) -> None:
        response = handle_request(
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": "git_status",
                    "arguments": {"repo_path": str(self.repo), "command": "status"},
                },
            },
            self.codex_safe_git,
        )

        result = response["result"]
        self.assertTrue(result["isError"])
        self.assertIn("unexpected tool arguments", result["structuredContent"]["reason"])

    def test_ensure_commit_branch_tool_call(self) -> None:
        run(["git", "switch", "--detach", "HEAD"], self.repo)
        response = handle_request(
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": "ensure_commit_branch",
                    "arguments": {
                        "repo_path": str(self.repo),
                        "branch_name": "codex/mcp-prepared",
                    },
                },
            },
            self.codex_safe_git,
        )

        result = response["result"]
        self.assertEqual(result["structuredContent"]["result"], "ok")
        self.assertEqual(result["structuredContent"]["branch"], "codex/mcp-prepared")

    def test_stdio_server_round_trip(self) -> None:
        self.write_file("stdio.txt", "stdio\n")
        env = os.environ.copy()
        env["CODEX_SAFE_GIT_ALLOWED_REPOS"] = str(self.repo)
        env["CODEX_SAFE_GIT_AUDIT_LOG"] = str(self.audit_log)
        env["PYTHONPATH"] = "src"
        payloads = [
            {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {}},
            {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "tools/call",
                "params": {
                    "name": "commit_files",
                    "arguments": {
                        "repo_path": str(self.repo),
                        "files": ["stdio.txt"],
                        "message": "Add stdio file",
                    },
                },
            },
        ]
        proc = subprocess.run(
            ["python3", "-m", "codex_safe_git"],
            cwd=Path(__file__).resolve().parents[2],
            env=env,
            input="".join(json.dumps(payload) + "\n" for payload in payloads),
            text=True,
            capture_output=True,
        )

        self.assertEqual(proc.returncode, 0, proc.stderr)
        responses = [json.loads(line) for line in proc.stdout.splitlines()]
        self.assertEqual([response["id"] for response in responses], [1, 2, 3])
        self.assertEqual(responses[2]["result"]["structuredContent"]["files"], ["stdio.txt"])
