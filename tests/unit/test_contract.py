from __future__ import annotations

import json
from pathlib import Path

from codex_safe_git.server import SERVER_INFO, handle_request
from tests.helpers import GitRepoTestCase


PROJECT_ROOT = Path(__file__).resolve().parents[2]


class CodexSafeGitContractTests(GitRepoTestCase):
    def test_schema_snapshot_is_valid_json(self) -> None:
        schema = json.loads(
            (PROJECT_ROOT / "docs" / "schemas" / "tool-results.schema.json").read_text(
                encoding="utf-8"
            )
        )

        self.assertEqual(schema["title"], "Codex Safe Git MCP Tool Results")
        self.assertIn("oneOf", schema)

    def test_initialize_exposes_server_version(self) -> None:
        response = handle_request({"jsonrpc": "2.0", "id": 1, "method": "initialize"})
        pyproject = (PROJECT_ROOT / "pyproject.toml").read_text(encoding="utf-8")
        expected_version = next(
            line.split('"')[1] for line in pyproject.splitlines() if line.startswith("version = ")
        )

        self.assertEqual(
            response["result"]["serverInfo"]["version"],
            expected_version,
        )
        self.assertEqual(SERVER_INFO["version"], expected_version)

    def test_success_and_refusal_payload_keys_are_stable(self) -> None:
        status_response = handle_request(
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": "git_status",
                    "arguments": {"repo_path": str(self.repo)},
                },
            },
            self.codex_safe_git,
        )
        refusal_response = handle_request(
            {
                "jsonrpc": "2.0",
                "id": 3,
                "method": "tools/call",
                "params": {
                    "name": "git_status",
                    "arguments": {"repo_path": str(self.root / "outside")},
                },
            },
            self.codex_safe_git,
        )

        self.assertEqual(
            sorted(status_response["result"]["structuredContent"]),
            [
                "ambiguous_reasons",
                "branch",
                "clean",
                "entries",
                "has_staged_changes",
                "is_detached",
                "redacted_secret_path_count",
                "repo",
                "result",
            ],
        )
        self.assertEqual(
            sorted(refusal_response["result"]["structuredContent"]),
            ["reason", "result"],
        )
        self.assertTrue(refusal_response["result"]["isError"])
