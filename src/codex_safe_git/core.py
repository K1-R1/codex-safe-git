from __future__ import annotations

import json
import os
import re
import subprocess
import tempfile
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Iterable


class CodexSafeGitRefusal(Exception):
    """A deterministic refusal for an unsafe or unsupported operation."""


AI_ATTRIBUTION = re.compile(
    r"(generated[- ]?by\s+(codex|chatgpt|openai)|"
    r"co-authored-by:.*\b(codex|chatgpt|openai)\b|"
    r"\b(via|with|using)\s+(codex|chatgpt|openai)\b|"
    r"\b(codex|chatgpt|openai)\s+(generated|assisted|authored)\b)",
    re.IGNORECASE,
)

LIKELY_SENSITIVE_MATERIAL = re.compile(
    r"(BEGIN [A-Z ]*PRIVATE KEY|"
    r"(?<![A-Za-z0-9_])(api[_-]?key|secret|token|password|credential)"
    r"(?![A-Za-z0-9_])\s*[:=]\s*['\"]?[A-Za-z0-9_./+=:-]{8,}|"
    r"aws_secret_access_key\s*=|"
    r"sk-[A-Za-z0-9]{20,}|"
    r"gh[pousr]_[A-Za-z0-9_]{20,}|"
    r"xox[baprs]-[A-Za-z0-9-]{20,})",
    re.IGNORECASE,
)

SECRET_FILE_NAMES = {
    ".npmrc",
    ".pypirc",
    ".netrc",
    "auth.json",
    "credentials",
    "credentials.json",
    "id_dsa",
    "id_ecdsa",
    "id_ed25519",
    "id_rsa",
    "secrets.json",
    "tokens.json",
}

SECRET_DIR_NAMES = {
    ".aws",
    ".azure",
    ".docker",
    ".gnupg",
    ".kube",
    ".ssh",
    "wallet",
    "wallets",
}

SECRET_SUFFIXES = {
    ".key",
    ".p12",
    ".pem",
    ".pfx",
}

AMBIGUOUS_STATE_FILES = (
    "MERGE_HEAD",
    "CHERRY_PICK_HEAD",
    "REVERT_HEAD",
    "BISECT_LOG",
)

AMBIGUOUS_STATE_DIRS = (
    "rebase-apply",
    "rebase-merge",
)

CONFLICT_CODES = {"DD", "AU", "UD", "UA", "DU", "AA", "UU"}

EXECUTION_CONFIG_PATTERN = (
    r"^(filter\..*\.(clean|process|smudge)|diff\..*\.(command|textconv)|core\.fsmonitor)$"
)

PROTECTED_BRANCHES = {"main", "master"}
REMOTE_LIKE_BRANCH_PREFIXES = {"origin", "upstream", "remotes"}


@dataclass(frozen=True)
class CodexSafeGitConfig:
    allowed_repos: frozenset[Path]
    audit_log: Path
    allowed_repo_roots: frozenset[Path] = frozenset()

    @classmethod
    def from_env(cls, env: dict[str, str] | None = None) -> "CodexSafeGitConfig":
        source = os.environ if env is None else env
        allowed_raw = source.get("CODEX_SAFE_GIT_ALLOWED_REPOS", "")
        allowed_roots_raw = source.get("CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS", "")
        audit_raw = source.get("CODEX_SAFE_GIT_AUDIT_LOG", "")
        if not allowed_raw.strip() and not allowed_roots_raw.strip():
            raise CodexSafeGitRefusal(
                "CODEX_SAFE_GIT_ALLOWED_REPOS or CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS is required"
            )
        if not audit_raw.strip():
            raise CodexSafeGitRefusal("CODEX_SAFE_GIT_AUDIT_LOG is required")
        allowed = _pathset_from_env(allowed_raw)
        allowed_roots = frozenset(
            _normalise_repo_root(path) for path in _pathset_from_env(allowed_roots_raw)
        )
        if not allowed and not allowed_roots:
            raise CodexSafeGitRefusal("allowed repo configuration has no usable entries")
        return cls(
            allowed_repos=allowed,
            allowed_repo_roots=allowed_roots,
            audit_log=Path(audit_raw).expanduser().resolve(),
        )


def _pathset_from_env(raw: str) -> frozenset[Path]:
    return frozenset(Path(item).expanduser().resolve() for item in raw.split(os.pathsep) if item.strip())


def _normalise_repo_root(path: Path) -> Path:
    if path == path.parent:
        raise CodexSafeGitRefusal("allowed repo root must not be the filesystem root")
    if not path.exists() or not path.is_dir():
        raise CodexSafeGitRefusal(f"allowed repo root does not exist or is not a directory: {path}")
    return path


def _path_within(path: Path, root: Path) -> bool:
    try:
        path.relative_to(root)
    except ValueError:
        return False
    return True


class CodexSafeGit:
    def __init__(self, config: CodexSafeGitConfig):
        self.config = config

    def git_status(self, repo_path: str) -> dict[str, Any]:
        try:
            repo = self._resolve_allowed_repo(repo_path)
            self._require_git_repo(repo)
            self._require_no_execution_config(repo)
            result = self._status(repo)
            self._audit("git_status", "ok", repo=repo, file_count=len(result["entries"]))
            return result
        except CodexSafeGitRefusal as exc:
            self._audit("git_status", "refused", repo_text=repo_path, reason=str(exc))
            raise

    def git_diff_summary(self, repo_path: str) -> dict[str, Any]:
        try:
            repo = self._resolve_allowed_repo(repo_path)
            self._require_git_repo(repo)
            self._require_no_execution_config(repo)
            state = self._repo_state(repo)
            if state["ambiguous_reasons"]:
                raise CodexSafeGitRefusal(
                    "repository has ambiguous state: " + ", ".join(state["ambiguous_reasons"])
                )
            unstaged = self._numstat(repo, ["diff", "--no-ext-diff", "--numstat", "-z"])
            staged = self._numstat(repo, ["diff", "--cached", "--no-ext-diff", "--numstat", "-z"])
            untracked = self._safe_untracked(repo)
            result = {
                "result": "ok",
                "repo": str(repo),
                "unstaged": unstaged,
                "staged": staged,
                "untracked": untracked,
            }
            self._audit(
                "git_diff_summary",
                "ok",
                repo=repo,
                file_count=len(unstaged["files"]) + len(staged["files"]) + len(untracked["files"]),
                redacted_secret_path_count=(
                    unstaged["redacted_secret_path_count"]
                    + staged["redacted_secret_path_count"]
                    + untracked["redacted_secret_path_count"]
                ),
            )
            return result
        except CodexSafeGitRefusal as exc:
            self._audit("git_diff_summary", "refused", repo_text=repo_path, reason=str(exc))
            raise

    def ensure_commit_branch(self, repo_path: str, branch_name: str) -> dict[str, Any]:
        branch = ""
        try:
            branch = self._normalise_branch_name(branch_name)
            repo = self._resolve_allowed_repo(repo_path)
            self._require_git_repo(repo)
            self._require_no_execution_config(repo)
            state = self._repo_state(repo)
            self._require_branch_prep_state(state)

            head_commit = self._current_head(repo)
            current_branch = state["branch"]
            if current_branch == branch:
                result = {
                    "result": "ok",
                    "repo": str(repo),
                    "branch": branch,
                    "action": "already_on_branch",
                    "head_commit": head_commit,
                }
                self._audit(
                    "ensure_commit_branch",
                    "ok",
                    repo=repo,
                    branch=branch,
                    commit_hash=head_commit,
                )
                return result
            if current_branch is not None:
                raise CodexSafeGitRefusal(
                    f"repository is already on a different branch: {current_branch}"
                )

            existing_head = self._local_branch_head(repo, branch)
            if existing_head is None:
                self._git(repo, ["switch", "-c", branch])
                action = "created"
            elif existing_head == head_commit:
                self._git(repo, ["switch", branch])
                action = "attached"
            else:
                raise CodexSafeGitRefusal("requested branch already exists at a different commit")

            result = {
                "result": "ok",
                "repo": str(repo),
                "branch": branch,
                "action": action,
                "head_commit": head_commit,
            }
            self._audit(
                "ensure_commit_branch",
                action,
                repo=repo,
                branch=branch,
                commit_hash=head_commit,
            )
            return result
        except CodexSafeGitRefusal as exc:
            self._audit(
                "ensure_commit_branch",
                "refused",
                repo_text=repo_path,
                branch=branch or branch_name if isinstance(branch_name, str) else None,
                reason=str(exc),
            )
            raise

    def commit_files(
        self,
        repo_path: str,
        files: Iterable[str],
        message: str,
        body: str | None = None,
    ) -> dict[str, Any]:
        file_list: list[str] = []
        try:
            file_list = self._normalise_file_argument(files)
            self._reject_ai_attribution(message, body)
            repo = self._resolve_allowed_repo(repo_path)
            requested = self._normalise_files(repo, file_list)
            self._require_git_repo(repo)
            self._require_no_execution_config(repo)
            state = self._require_clear_commit_state(repo)
            self._require_commit_branch_allowed(state)
            self._reject_likely_secret_material(repo, requested)

            self._git(repo, ["add", "--", *requested])
            staged = self._staged_files(repo)
            try:
                if self._is_index_empty(repo):
                    raise CodexSafeGitRefusal("refusing empty commit")
                if sorted(staged) != sorted(requested):
                    raise CodexSafeGitRefusal("staged file set does not exactly match requested files")
                self._git(repo, ["commit", "--no-gpg-sign", "-m", message, *(["-m", body] if body else [])])
            except Exception:
                self._unstage_best_effort(repo, requested)
                raise

            commit_hash = self._git(repo, ["rev-parse", "HEAD"]).stdout.strip()
            result = {
                "result": "committed",
                "repo": str(repo),
                "commit_hash": commit_hash,
                "files": sorted(requested),
                "audit_summary": (
                    "allowlisted repo; clear state; exact staged file set; "
                    "no likely secrets; attribution-free message"
                ),
            }
            self._audit("commit_files", "committed", repo=repo, files=sorted(requested), commit_hash=commit_hash)
            return result
        except CodexSafeGitRefusal as exc:
            self._audit("commit_files", "refused", repo_text=repo_path, files=file_list, reason=str(exc))
            raise

    def _normalise_file_argument(self, files: Iterable[str]) -> list[str]:
        if isinstance(files, (str, bytes)):
            raise CodexSafeGitRefusal("files must be an iterable of path strings, not a single string")
        try:
            file_list = list(files)
        except TypeError as exc:
            raise CodexSafeGitRefusal("files must be an iterable of path strings") from exc
        if not all(isinstance(item, str) for item in file_list):
            raise CodexSafeGitRefusal("files must contain only path strings")
        return file_list

    def _resolve_allowed_repo(self, repo_path: str) -> Path:
        repo = Path(repo_path).expanduser().resolve()
        if repo in self.config.allowed_repos:
            return repo
        if any(_path_within(repo, root) for root in self.config.allowed_repo_roots):
            return repo
        raise CodexSafeGitRefusal("repo_path is not explicitly allowlisted or under an allowed repo root")

    def _require_git_repo(self, repo: Path) -> None:
        if not repo.exists() or not repo.is_dir():
            raise CodexSafeGitRefusal("repo_path does not exist or is not a directory")
        inside = self._git(repo, ["rev-parse", "--is-inside-work-tree"], check=False)
        if inside.returncode != 0 or inside.stdout.strip() != "true":
            raise CodexSafeGitRefusal("repo_path is not a Git worktree")
        top = Path(self._git(repo, ["rev-parse", "--show-toplevel"]).stdout.strip()).resolve()
        if top != repo:
            raise CodexSafeGitRefusal("repo_path must be the Git worktree root")

    def _require_clear_commit_state(self, repo: Path) -> dict[str, Any]:
        state = self._repo_state(repo)
        if state["ambiguous_reasons"]:
            raise CodexSafeGitRefusal("repository has ambiguous state: " + ", ".join(state["ambiguous_reasons"]))
        if state["has_staged_changes"]:
            raise CodexSafeGitRefusal("repository already has staged changes")
        return state

    def _require_branch_prep_state(self, state: dict[str, Any]) -> None:
        blocking = [reason for reason in state["ambiguous_reasons"] if reason != "detached HEAD"]
        if blocking:
            raise CodexSafeGitRefusal("repository has ambiguous state: " + ", ".join(blocking))
        if state["has_staged_changes"]:
            raise CodexSafeGitRefusal("repository already has staged changes")

    def _require_commit_branch_allowed(self, state: dict[str, Any]) -> None:
        branch = state["branch"]
        if branch in PROTECTED_BRANCHES:
            raise CodexSafeGitRefusal(f"refusing commit on protected branch: {branch}")

    def _require_no_execution_config(self, repo: Path) -> None:
        result = self._git(repo, ["config", "--get-regexp", EXECUTION_CONFIG_PATTERN], check=False)
        if result.returncode == 1:
            return
        if result.returncode != 0:
            detail = (result.stderr or result.stdout).strip()
            raise CodexSafeGitRefusal(f"git config inspection failed: {detail}")
        keys = sorted({line.split(None, 1)[0] for line in result.stdout.splitlines() if line.strip()})
        if keys:
            raise CodexSafeGitRefusal("repository has execution-capable Git config: " + ", ".join(keys))

    def _repo_state(self, repo: Path) -> dict[str, Any]:
        ambiguous: list[str] = []
        branch_result = self._git(repo, ["symbolic-ref", "--quiet", "--short", "HEAD"], check=False)
        branch = branch_result.stdout.strip() if branch_result.returncode == 0 else None
        if branch is None:
            ambiguous.append("detached HEAD")

        git_dir = self._git_dir(repo)
        for name in AMBIGUOUS_STATE_FILES:
            if (git_dir / name).exists():
                ambiguous.append(name)
        for name in AMBIGUOUS_STATE_DIRS:
            if (git_dir / name).exists():
                ambiguous.append(name)

        entries = self._porcelain_entries(repo)
        if any(entry["code"] in CONFLICT_CODES or "U" in entry["code"] for entry in entries):
            ambiguous.append("unresolved conflicts")

        return {
            "branch": branch,
            "is_detached": branch is None,
            "ambiguous_reasons": ambiguous,
            "has_staged_changes": bool(self._staged_files(repo)),
        }

    def _status(self, repo: Path) -> dict[str, Any]:
        state = self._repo_state(repo)
        entries: list[dict[str, str]] = []
        redacted = 0
        for entry in self._porcelain_entries(repo):
            if self._is_secret_path(entry["path"]):
                redacted += 1
                continue
            entries.append(entry)
        return {
            "result": "ok",
            "repo": str(repo),
            "branch": state["branch"],
            "is_detached": state["is_detached"],
            "ambiguous_reasons": state["ambiguous_reasons"],
            "has_staged_changes": state["has_staged_changes"],
            "clean": not entries and redacted == 0,
            "entries": entries,
            "redacted_secret_path_count": redacted,
        }

    def _normalise_files(self, repo: Path, files: list[str]) -> list[str]:
        if not files:
            raise CodexSafeGitRefusal("at least one file must be listed")
        normalised: list[str] = []
        seen: set[str] = set()
        for item in files:
            if not isinstance(item, str) or not item.strip():
                raise CodexSafeGitRefusal("requested files must be non-empty strings")
            raw = Path(item)
            path = raw.expanduser().resolve() if raw.is_absolute() else (repo / raw).resolve()
            try:
                rel = path.relative_to(repo).as_posix()
            except ValueError as exc:
                raise CodexSafeGitRefusal(f"requested file is outside repo: {item}") from exc
            if rel in {"", "."}:
                raise CodexSafeGitRefusal("requested path must be a file")
            if rel in seen:
                raise CodexSafeGitRefusal(f"duplicate requested file: {rel}")
            seen.add(rel)
            self._reject_secret_path(rel)
            if path.exists():
                if not path.is_file() or path.is_symlink():
                    raise CodexSafeGitRefusal(f"requested path must be a regular file: {rel}")
            elif not self._tracked(repo, rel):
                raise CodexSafeGitRefusal(f"requested file does not exist and is not tracked: {rel}")
            normalised.append(rel)
        return normalised

    def _reject_secret_path(self, rel: str) -> None:
        if self._is_secret_path(rel):
            raise CodexSafeGitRefusal(f"refusing secret-bearing path: {rel}")

    def _is_secret_path(self, rel: str) -> bool:
        path = Path(rel)
        parts = {part.lower() for part in path.parts}
        if parts & SECRET_DIR_NAMES:
            return True
        if any("wallet" in part for part in parts):
            return True
        name = path.name.lower()
        return (
            name == ".env"
            or name.startswith(".env.")
            or name in SECRET_FILE_NAMES
            or name.startswith("id_rsa")
            or name.startswith("id_dsa")
            or name.startswith("id_ecdsa")
            or name.startswith("id_ed25519")
            or any(name.endswith(suffix) for suffix in SECRET_SUFFIXES)
        )

    def _reject_likely_secret_material(self, repo: Path, rels: list[str]) -> None:
        for rel in rels:
            path = repo / rel
            if path.exists() and not self._tracked(repo, rel):
                additions = path.read_text(encoding="utf-8", errors="replace").splitlines()
            else:
                diff = self._git(repo, ["diff", "--no-ext-diff", "--unified=0", "--", rel]).stdout
                additions = [
                    line[1:]
                    for line in diff.splitlines()
                    if line.startswith("+") and not line.startswith("+++")
                ]
            if any(LIKELY_SENSITIVE_MATERIAL.search(line) for line in additions):
                raise CodexSafeGitRefusal(f"requested diff appears to contain secret material: {rel}")

    def _reject_ai_attribution(self, message: str, body: str | None) -> None:
        if not isinstance(message, str) or not message.strip():
            raise CodexSafeGitRefusal("commit message subject is empty")
        text = "\n".join(part for part in (message, body) if part)
        if "\x00" in text:
            raise CodexSafeGitRefusal("commit message contains NUL")
        if AI_ATTRIBUTION.search(text):
            raise CodexSafeGitRefusal("commit message contains AI/tool attribution")

    def _normalise_branch_name(self, branch_name: str) -> str:
        if not isinstance(branch_name, str) or not branch_name.strip():
            raise CodexSafeGitRefusal("branch_name must be a non-empty string")
        branch = branch_name.strip()
        if branch != branch_name or "\x00" in branch:
            raise CodexSafeGitRefusal("branch_name contains invalid characters")
        if branch in PROTECTED_BRANCHES:
            raise CodexSafeGitRefusal(f"refusing protected branch: {branch}")
        if branch.startswith("-") or branch.startswith("/") or branch.endswith("/"):
            raise CodexSafeGitRefusal("branch_name has unsafe syntax")
        if branch.startswith("refs/") or branch.split("/", 1)[0] in REMOTE_LIKE_BRANCH_PREFIXES:
            raise CodexSafeGitRefusal("branch_name must be a local branch, not a remote/ref path")
        unsafe_tokens = ("..", "//", "@{", "\\", ":", "?", "[", "*", "~", "^", " ")
        if any(token in branch for token in unsafe_tokens):
            raise CodexSafeGitRefusal("branch_name has unsafe syntax")
        if branch.endswith(".") or branch.endswith(".lock"):
            raise CodexSafeGitRefusal("branch_name has unsafe syntax")
        if any(part.startswith(".") or part.endswith(".lock") for part in branch.split("/")):
            raise CodexSafeGitRefusal("branch_name has unsafe syntax")
        check = subprocess.run(
            ["git", "check-ref-format", "--branch", branch],
            text=True,
            capture_output=True,
            env=self._git_env(),
        )
        if check.returncode != 0:
            raise CodexSafeGitRefusal("branch_name is not a valid local branch name")
        return branch

    def _safe_untracked(self, repo: Path) -> dict[str, Any]:
        files: list[str] = []
        redacted = 0
        for entry in self._porcelain_entries(repo):
            if entry["code"] != "??":
                continue
            if self._is_secret_path(entry["path"]):
                redacted += 1
            else:
                files.append(entry["path"])
        return {"files": sorted(files), "redacted_secret_path_count": redacted}

    def _numstat(self, repo: Path, args: list[str]) -> dict[str, Any]:
        raw = self._git(repo, args).stdout
        records = [record for record in raw.split("\0") if record]
        files: list[dict[str, Any]] = []
        redacted = 0
        for record in records:
            fields = record.split("\t")
            if len(fields) < 3:
                continue
            additions_text, deletions_text, rel = fields[0], fields[1], fields[-1]
            if self._is_secret_path(rel):
                redacted += 1
                continue
            files.append(
                {
                    "path": rel,
                    "additions": None if additions_text == "-" else int(additions_text),
                    "deletions": None if deletions_text == "-" else int(deletions_text),
                }
            )
        return {"files": files, "redacted_secret_path_count": redacted}

    def _porcelain_entries(self, repo: Path) -> list[dict[str, str]]:
        raw = self._git(repo, ["status", "--porcelain=v1", "-z"]).stdout
        parts = [part for part in raw.split("\0") if part]
        entries: list[dict[str, str]] = []
        index = 0
        while index < len(parts):
            item = parts[index]
            code = item[:2]
            path = item[3:] if len(item) > 3 else ""
            if code.startswith("R") or code.startswith("C"):
                index += 1
                if index < len(parts):
                    path = parts[index]
            entries.append({"code": code, "path": path})
            index += 1
        return entries

    def _staged_files(self, repo: Path) -> list[str]:
        raw = self._git(repo, ["diff", "--cached", "--no-renames", "--name-only", "-z"]).stdout
        return [item for item in raw.split("\0") if item]

    def _is_index_empty(self, repo: Path) -> bool:
        return self._git(repo, ["diff", "--cached", "--quiet", "--exit-code"], check=False).returncode == 0

    def _tracked(self, repo: Path, rel: str) -> bool:
        return self._git(repo, ["ls-files", "--error-unmatch", "--", rel], check=False).returncode == 0

    def _current_head(self, repo: Path) -> str:
        return self._git(repo, ["rev-parse", "HEAD"]).stdout.strip()

    def _local_branch_head(self, repo: Path, branch: str) -> str | None:
        result = self._git(
            repo,
            ["rev-parse", "--verify", "--quiet", f"refs/heads/{branch}^{{commit}}"],
            check=False,
        )
        if result.returncode == 0:
            return result.stdout.strip()
        if result.returncode == 1:
            return None
        detail = (result.stderr or result.stdout).strip()
        raise CodexSafeGitRefusal(f"git branch inspection failed: {detail}")

    def _git_dir(self, repo: Path) -> Path:
        raw = self._git(repo, ["rev-parse", "--git-dir"]).stdout.strip()
        git_dir = Path(raw)
        if not git_dir.is_absolute():
            git_dir = repo / git_dir
        return git_dir.resolve()

    def _unstage_best_effort(self, repo: Path, rels: list[str]) -> None:
        if rels:
            self._git(repo, ["reset", "-q", "HEAD", "--", *rels], check=False)

    def _git(
        self,
        repo: Path,
        args: list[str],
        *,
        check: bool = True,
    ) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory(prefix="codex-safe-git-hooks-") as hooks_dir:
            command = [
                "git",
                "-c",
                "commit.gpgsign=false",
                "-c",
                "tag.gpgsign=false",
                "-c",
                f"core.hooksPath={hooks_dir}",
                "-c",
                "credential.helper=",
                "-c",
                "core.pager=cat",
                "-C",
                str(repo),
                *args,
            ]
            env = self._git_env()
            result = subprocess.run(command, text=True, capture_output=True, env=env)
        if check and result.returncode != 0:
            detail = (result.stderr or result.stdout).strip()
            safe_args = " ".join(args[:2])
            raise CodexSafeGitRefusal(f"git {safe_args} failed: {detail}")
        return result

    def _git_env(self) -> dict[str, str]:
        env: dict[str, str] = {}
        for name in ("PATH", "HOME", "LANG", "LC_ALL", "TMPDIR"):
            value = os.environ.get(name)
            if value:
                env[name] = value
        false_path = "/usr/bin/false" if Path("/usr/bin/false").exists() else "false"
        env.update(
            {
                "GIT_ASKPASS": false_path,
                "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_EDITOR": ":",
                "GIT_TERMINAL_PROMPT": "0",
                "SSH_ASKPASS": false_path,
            }
        )
        return env

    def _audit(
        self,
        action: str,
        result: str,
        *,
        repo: Path | None = None,
        repo_text: str | None = None,
        files: list[str] | None = None,
        commit_hash: str | None = None,
        branch: str | None = None,
        reason: str | None = None,
        file_count: int | None = None,
        redacted_secret_path_count: int | None = None,
    ) -> None:
        payload: dict[str, Any] = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "action": action,
            "result": result,
        }
        if repo is not None:
            payload["repo"] = str(repo)
        elif repo_text is not None:
            payload["repo"] = repo_text
        if files is not None:
            payload["files"] = files
        if commit_hash is not None:
            payload["commit_hash"] = commit_hash
        if branch is not None:
            payload["branch"] = branch
        if reason is not None:
            payload["reason"] = reason
        if file_count is not None:
            payload["file_count"] = file_count
        if redacted_secret_path_count is not None:
            payload["redacted_secret_path_count"] = redacted_secret_path_count

        self.config.audit_log.parent.mkdir(parents=True, exist_ok=True)
        with self.config.audit_log.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(payload, sort_keys=True) + "\n")
