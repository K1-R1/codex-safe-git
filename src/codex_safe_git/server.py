from __future__ import annotations

import json
import sys
from typing import Any

from .core import CodexSafeGit, CodexSafeGitConfig, CodexSafeGitRefusal


PROTOCOL_VERSION = "2025-11-25"
SERVER_INFO = {
    "name": "codex-safe-git-commit",
    "title": "Codex Safe Git Commit",
    "version": "0.1.0",
}


TOOLS: list[dict[str, Any]] = [
    {
        "name": "git_status",
        "title": "Git Status",
        "description": "Return a redacted, read-only status summary for an explicitly allowlisted local Git repo.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "repo_path": {"type": "string"},
            },
            "required": ["repo_path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "git_diff_summary",
        "title": "Git Diff Summary",
        "description": "Return redacted file-level diff counts for an explicitly allowlisted local Git repo.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "repo_path": {"type": "string"},
            },
            "required": ["repo_path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "commit_files",
        "title": "Commit Files",
        "description": "Create a local commit from exactly listed files after deterministic safety checks.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "repo_path": {"type": "string"},
                "files": {"type": "array", "items": {"type": "string"}, "minItems": 1},
                "message": {"type": "string"},
                "body": {"type": "string"},
            },
            "required": ["repo_path", "files", "message"],
            "additionalProperties": False,
        },
    },
    {
        "name": "ensure_commit_branch",
        "title": "Ensure Commit Branch",
        "description": "Attach a detached worktree to a safe local non-default branch at current HEAD.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "repo_path": {"type": "string"},
                "branch_name": {"type": "string"},
            },
            "required": ["repo_path", "branch_name"],
            "additionalProperties": False,
        },
    },
]


def handle_request(message: dict[str, Any], codex_safe_git: CodexSafeGit | None = None) -> dict[str, Any] | None:
    method = message.get("method")
    request_id = message.get("id")

    if method == "notifications/initialized":
        return None
    if method == "initialize":
        return _result(
            request_id,
            {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {"tools": {"listChanged": False}},
                "serverInfo": SERVER_INFO,
                "instructions": (
                    "Use only git_status, git_diff_summary, ensure_commit_branch, and commit_files. "
                    "Repos must be explicitly allowlisted by environment."
                ),
            },
        )
    if method == "tools/list":
        return _result(request_id, {"tools": TOOLS})
    if method == "tools/call":
        params = message.get("params") or {}
        return _result(request_id, call_tool(params, codex_safe_git))
    if request_id is None:
        return None
    return _error(request_id, -32601, f"Unsupported method: {method}")


def call_tool(params: dict[str, Any], codex_safe_git: CodexSafeGit | None = None) -> dict[str, Any]:
    try:
        name = params.get("name")
        arguments = params.get("arguments") or {}
        if not isinstance(arguments, dict):
            raise CodexSafeGitRefusal("tool arguments must be an object")
        client = codex_safe_git or CodexSafeGit(CodexSafeGitConfig.from_env())
        if name == "git_status":
            _reject_unexpected_args(arguments, {"repo_path"})
            payload = client.git_status(_string_arg(arguments, "repo_path"))
        elif name == "git_diff_summary":
            _reject_unexpected_args(arguments, {"repo_path"})
            payload = client.git_diff_summary(_string_arg(arguments, "repo_path"))
        elif name == "commit_files":
            _reject_unexpected_args(arguments, {"repo_path", "files", "message", "body"})
            payload = client.commit_files(
                _string_arg(arguments, "repo_path"),
                _string_list_arg(arguments, "files"),
                _string_arg(arguments, "message"),
                _optional_string_arg(arguments, "body"),
            )
        elif name == "ensure_commit_branch":
            _reject_unexpected_args(arguments, {"repo_path", "branch_name"})
            payload = client.ensure_commit_branch(
                _string_arg(arguments, "repo_path"),
                _string_arg(arguments, "branch_name"),
            )
        else:
            raise CodexSafeGitRefusal(f"unsupported tool: {name}")
        return _tool_payload(payload, is_error=False)
    except CodexSafeGitRefusal as exc:
        return _tool_payload({"result": "refused", "reason": str(exc)}, is_error=True)


def _reject_unexpected_args(arguments: dict[str, Any], allowed: set[str]) -> None:
    unexpected = sorted(set(arguments) - allowed)
    if unexpected:
        raise CodexSafeGitRefusal("unexpected tool arguments: " + ", ".join(unexpected))


def _string_arg(arguments: dict[str, Any], name: str) -> str:
    value = arguments.get(name)
    if not isinstance(value, str):
        raise CodexSafeGitRefusal(f"{name} must be a string")
    return value


def _optional_string_arg(arguments: dict[str, Any], name: str) -> str | None:
    value = arguments.get(name)
    if value is None:
        return None
    if not isinstance(value, str):
        raise CodexSafeGitRefusal(f"{name} must be a string")
    return value


def _string_list_arg(arguments: dict[str, Any], name: str) -> list[str]:
    value = arguments.get(name)
    if not isinstance(value, list) or not all(isinstance(item, str) for item in value):
        raise CodexSafeGitRefusal(f"{name} must be an array of strings")
    return value


def _tool_payload(payload: dict[str, Any], *, is_error: bool) -> dict[str, Any]:
    text = json.dumps(payload, sort_keys=True)
    result: dict[str, Any] = {
        "content": [{"type": "text", "text": text}],
        "structuredContent": payload,
    }
    if is_error:
        result["isError"] = True
    return result


def _result(request_id: Any, result: dict[str, Any]) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "result": result}


def _error(request_id: Any, code: int, message: str) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}


def main() -> int:
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            message = json.loads(line)
            response = handle_request(message)
        except Exception as exc:
            request_id = None
            if "message" in locals() and isinstance(message, dict):
                request_id = message.get("id")
            response = _error(request_id, -32603, str(exc))
        if response is not None:
            sys.stdout.write(json.dumps(response, separators=(",", ":")) + "\n")
            sys.stdout.flush()
    return 0
