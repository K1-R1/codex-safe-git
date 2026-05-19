from __future__ import annotations

import tempfile
from pathlib import Path

from codex_safe_git.core import CodexSafeGit, CodexSafeGitConfig, CodexSafeGitRefusal
from tests.helpers import GitRepoTestCase, run


class CodexSafeGitCommitIntegrationTests(GitRepoTestCase):
    def test_commit_files_commits_only_explicit_files(self) -> None:
        self.assertEqual(run(["git", "branch", "--show-current"], self.repo).stdout.strip(), "work")
        self.write_file("allowed.txt", "allowed\n")
        self.write_file("unlisted.txt", "unlisted\n")

        result = self.codex_safe_git.commit_files(str(self.repo), ["allowed.txt"], "Add allowed file")

        self.assertEqual(result["files"], ["allowed.txt"])
        shown = run(["git", "show", "--name-only", "--format=%s", "HEAD"], self.repo).stdout
        self.assertIn("Add allowed file", shown)
        self.assertIn("allowed.txt", shown)
        self.assertNotIn("unlisted.txt", shown)
        status = run(["git", "status", "--short"], self.repo).stdout
        self.assertIn("?? unlisted.txt", status)
        self.assertEqual(self.audit_entries()[-1]["result"], "committed")

    def test_commit_files_refuses_protected_default_branches(self) -> None:
        run(["git", "switch", "main"], self.repo)
        self.write_file("main.txt", "main\n")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "protected branch: main"):
            self.codex_safe_git.commit_files(str(self.repo), ["main.txt"], "Try main commit")

        run(["git", "switch", "-c", "master"], self.repo)
        self.write_file("master.txt", "master\n")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "protected branch: master"):
            self.codex_safe_git.commit_files(str(self.repo), ["master.txt"], "Try master commit")

        run(["git", "switch", "-c", "trunk"], self.repo)
        run(["git", "config", "init.defaultBranch", "trunk"], self.repo)
        self.write_file("trunk.txt", "trunk\n")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "protected branch: trunk"):
            self.codex_safe_git.commit_files(str(self.repo), ["trunk.txt"], "Try trunk commit")

    def test_commit_files_supports_tracked_deletions(self) -> None:
        self.write_file("old.txt", "old\n")
        self.codex_safe_git.commit_files(str(self.repo), ["old.txt"], "Add old file")
        (self.repo / "old.txt").unlink()

        result = self.codex_safe_git.commit_files(str(self.repo), ["old.txt"], "Remove old file")

        self.assertEqual(result["files"], ["old.txt"])
        self.assertFalse((self.repo / "old.txt").exists())

    def test_commit_files_supports_explicit_rename_pairs(self) -> None:
        self.write_file("old/package.py", "same content\n")
        self.codex_safe_git.commit_files(str(self.repo), ["old/package.py"], "Add old package")
        (self.repo / "old/package.py").unlink()
        self.write_file("new/package.py", "same content\n")

        result = self.codex_safe_git.commit_files(
            str(self.repo),
            ["old/package.py", "new/package.py"],
            "Rename package path",
        )

        self.assertEqual(result["files"], ["new/package.py", "old/package.py"])
        self.assertEqual(run(["git", "status", "--short"], self.repo).stdout, "")
        shown = run(["git", "show", "--name-status", "--format=%s", "--no-renames", "HEAD"], self.repo).stdout
        self.assertIn("Rename package path", shown)
        self.assertIn("A\tnew/package.py", shown)
        self.assertIn("D\told/package.py", shown)

    def test_failed_exact_file_check_leaves_index_clean(self) -> None:
        self.write_file("old.txt", "old\n")
        self.codex_safe_git.commit_files(str(self.repo), ["old.txt"], "Add old file")
        (self.repo / "old.txt").unlink()
        self.write_file("new.txt", "new\n")
        original_staged_files = self.codex_safe_git._staged_files

        def mismatched_staged_files(repo: Path) -> list[str]:
            files = original_staged_files(repo)
            if files == ["new.txt", "old.txt"]:
                return ["new.txt"]
            return files

        self.codex_safe_git._staged_files = mismatched_staged_files  # type: ignore[method-assign]

        with self.assertRaisesRegex(CodexSafeGitRefusal, "staged file set"):
            self.codex_safe_git.commit_files(
                str(self.repo),
                ["old.txt", "new.txt"],
                "Try mismatched staging",
            )

        self.assertEqual(run(["git", "diff", "--cached", "--name-only"], self.repo).stdout, "")
        self.assertIn(" D old.txt", run(["git", "status", "--short"], self.repo).stdout)

    def test_refuses_unallowlisted_and_non_git_repos(self) -> None:
        outside = self.root / "outside"
        outside.mkdir()
        with self.assertRaisesRegex(CodexSafeGitRefusal, "not explicitly allowlisted"):
            self.codex_safe_git.git_status(str(outside))

        not_git = self.root / "not-git"
        not_git.mkdir()
        other = CodexSafeGit(
            CodexSafeGitConfig(
                allowed_repos=frozenset({not_git.resolve()}),
                audit_log=self.audit_log,
            )
        )
        with self.assertRaisesRegex(CodexSafeGitRefusal, "not a Git worktree"):
            other.git_status(str(not_git))

    def test_repo_under_allowed_root_is_allowed_but_must_be_worktree_root(self) -> None:
        codex_safe_git = CodexSafeGit(
            CodexSafeGitConfig(
                allowed_repos=frozenset(),
                allowed_repo_roots=frozenset({self.root.resolve()}),
                audit_log=self.audit_log,
            )
        )
        self.write_file("rooted.txt", "rooted\n")

        result = codex_safe_git.commit_files(str(self.repo), ["rooted.txt"], "Add rooted file")

        self.assertEqual(result["files"], ["rooted.txt"])
        subdir = self.repo / "nested"
        subdir.mkdir()
        with self.assertRaisesRegex(CodexSafeGitRefusal, "must be the Git worktree root"):
            codex_safe_git.git_status(str(subdir))

        with tempfile.TemporaryDirectory(prefix="codex-safe-git-outside-") as outside_raw:
            outside = Path(outside_raw)
            with self.assertRaisesRegex(CodexSafeGitRefusal, "not explicitly allowlisted"):
                codex_safe_git.git_status(str(outside))

    def test_refuses_outside_secret_and_likely_secret_files(self) -> None:
        self.write_file("normal.txt", "normal\n")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "outside repo"):
            self.codex_safe_git.commit_files(str(self.repo), [str(self.root / "outside.txt")], "Try outside")

        self.write_file(".env", "PLACEHOLDER=value\n")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "secret-bearing path"):
            self.codex_safe_git.commit_files(str(self.repo), [".env"], "Try env")

        self.write_file("config.txt", "api" + "_key = abcdefghijklmnop\n")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "secret material"):
            self.codex_safe_git.commit_files(str(self.repo), ["config.txt"], "Try secret material")

    def test_secret_scanner_ignores_keywords_inside_identifiers(self) -> None:
        line = "LIKELY_" + "SEC" + "RET = re.compile('placeholder')\n"
        self.write_file("scanner.py", line)

        result = self.codex_safe_git.commit_files(str(self.repo), ["scanner.py"], "Add scanner source")

        self.assertEqual(result["files"], ["scanner.py"])

    def test_refuses_bad_messages_empty_commits_and_staged_state(self) -> None:
        self.write_file("readme.txt", "readme\n")
        self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Add readme")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "not a single string"):
            self.codex_safe_git.commit_files(str(self.repo), "readme.txt", "Try string files")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "AI/tool attribution"):
            self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Generated by Codex")

        self.write_file("project-name.txt", "project name\n")
        result = self.codex_safe_git.commit_files(
            str(self.repo),
            ["project-name.txt"],
            "Allow codex-safe-git across project roots",
        )
        self.assertEqual(result["files"], ["project-name.txt"])

        with self.assertRaisesRegex(CodexSafeGitRefusal, "empty commit"):
            self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Try empty")

        self.write_file("staged.txt", "staged\n")
        run(["git", "add", "staged.txt"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "already has staged changes"):
            self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Try staged state")

    def test_refuses_ambiguous_states(self) -> None:
        self.write_file("readme.txt", "readme\n")
        self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Add readme")

        run(["git", "switch", "--detach", "HEAD"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "detached HEAD"):
            self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Try detached")
        run(["git", "switch", "work"], self.repo)

        git_dir = Path(run(["git", "rev-parse", "--git-dir"], self.repo).stdout.strip())
        if not git_dir.is_absolute():
            git_dir = self.repo / git_dir
        (git_dir / "MERGE_HEAD").write_text("0" * 40 + "\n", encoding="utf-8")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "MERGE_HEAD"):
            self.codex_safe_git.commit_files(str(self.repo), ["readme.txt"], "Try merge")

    def test_ensure_commit_branch_prepares_detached_head_then_commits(self) -> None:
        run(["git", "switch", "--detach", "HEAD"], self.repo)
        self.write_file("prepared.txt", "prepared\n")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "detached HEAD"):
            self.codex_safe_git.commit_files(str(self.repo), ["prepared.txt"], "Try detached")

        prepared = self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/prepared")

        self.assertEqual(prepared["result"], "ok")
        self.assertEqual(prepared["branch"], "codex/prepared")
        self.assertEqual(prepared["action"], "created")
        self.assertEqual(run(["git", "branch", "--show-current"], self.repo).stdout.strip(), "codex/prepared")
        committed = self.codex_safe_git.commit_files(str(self.repo), ["prepared.txt"], "Add prepared file")
        self.assertEqual(committed["files"], ["prepared.txt"])

    def test_ensure_commit_branch_attaches_existing_branch_at_current_head(self) -> None:
        head = run(["git", "rev-parse", "HEAD"], self.repo).stdout.strip()
        run(["git", "branch", "codex/existing", head], self.repo)
        run(["git", "switch", "--detach", head], self.repo)

        prepared = self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/existing")

        self.assertEqual(prepared["action"], "attached")
        self.assertEqual(run(["git", "branch", "--show-current"], self.repo).stdout.strip(), "codex/existing")

    def test_ensure_commit_branch_is_idempotent_on_requested_branch(self) -> None:
        run(["git", "switch", "-c", "codex/current"], self.repo)

        prepared = self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/current")

        self.assertEqual(prepared["action"], "already_on_branch")
        self.assertEqual(run(["git", "branch", "--show-current"], self.repo).stdout.strip(), "codex/current")

    def test_ensure_commit_branch_refusals(self) -> None:
        outside = self.root / "outside"
        outside.mkdir()
        with self.assertRaisesRegex(CodexSafeGitRefusal, "not explicitly allowlisted"):
            self.codex_safe_git.ensure_commit_branch(str(outside), "codex/outside")

        for branch in ("main", "master", "../bad", "origin/feature", "bad lock", "abcdef1"):
            with self.subTest(branch=branch):
                with self.assertRaises(CodexSafeGitRefusal):
                    self.codex_safe_git.ensure_commit_branch(str(self.repo), branch)

        with self.assertRaisesRegex(CodexSafeGitRefusal, "different branch: work"):
            self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/other")

        head = run(["git", "rev-parse", "HEAD"], self.repo).stdout.strip()
        run(["git", "branch", "codex/old", head], self.repo)
        self.write_file("advance.txt", "advance\n")
        self.codex_safe_git.commit_files(str(self.repo), ["advance.txt"], "Advance work")
        run(["git", "switch", "--detach", "HEAD"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "different commit"):
            self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/old")

        git_dir = Path(run(["git", "rev-parse", "--git-dir"], self.repo).stdout.strip())
        if not git_dir.is_absolute():
            git_dir = self.repo / git_dir
        (git_dir / "MERGE_HEAD").write_text("0" * 40 + "\n", encoding="utf-8")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "MERGE_HEAD"):
            self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/merge")

    def test_create_commit_branch_from_current_head(self) -> None:
        created = self.codex_safe_git.create_commit_branch(str(self.repo), "codex/new-work")

        self.assertEqual(created["result"], "ok")
        self.assertEqual(created["branch"], "codex/new-work")
        self.assertEqual(created["action"], "created")
        self.assertEqual(run(["git", "branch", "--show-current"], self.repo).stdout.strip(), "codex/new-work")

        repeated = self.codex_safe_git.create_commit_branch(str(self.repo), "codex/new-work")
        self.assertEqual(repeated["action"], "already_on_branch")
        self.assertEqual(self.audit_entries()[-1]["action"], "create_commit_branch")

    def test_create_commit_branch_refusals(self) -> None:
        for branch in ("main", "master", "../bad", "origin/feature", "bad lock", "abcdef1"):
            with self.subTest(branch=branch):
                with self.assertRaises(CodexSafeGitRefusal):
                    self.codex_safe_git.create_commit_branch(str(self.repo), branch)

        run(["git", "config", "init.defaultBranch", "trunk"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "protected branch: trunk"):
            self.codex_safe_git.create_commit_branch(str(self.repo), "trunk")

        run(["git", "branch", "codex/old", "HEAD"], self.repo)
        self.write_file("advance-branch.txt", "advance\n")
        self.codex_safe_git.commit_files(str(self.repo), ["advance-branch.txt"], "Advance branch")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "different commit"):
            self.codex_safe_git.create_commit_branch(str(self.repo), "codex/old")

        self.write_file("staged-branch.txt", "staged\n")
        run(["git", "add", "staged-branch.txt"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "already has staged changes"):
            self.codex_safe_git.create_commit_branch(str(self.repo), "codex/staged")

    def test_create_commit_branch_supports_detached_linked_worktree(self) -> None:
        linked = self.root / "linked-create"
        run(["git", "worktree", "add", "--detach", str(linked), "HEAD"], self.repo)
        codex_safe_git = CodexSafeGit(
            CodexSafeGitConfig(
                allowed_repos=frozenset({self.repo.resolve(), linked.resolve()}),
                audit_log=self.audit_log,
            )
        )

        created = codex_safe_git.create_commit_branch(str(linked), "codex/linked-create")

        self.assertEqual(created["action"], "created")
        self.assertEqual(
            run(["git", "branch", "--show-current"], linked).stdout.strip(),
            "codex/linked-create",
        )

    def test_merge_branch_fast_forwards_non_default_target(self) -> None:
        run(["git", "switch", "-c", "codex/source"], self.repo)
        self.write_file("merge.txt", "merge\n")
        self.codex_safe_git.commit_files(str(self.repo), ["merge.txt"], "Add merge source")
        source_head = run(["git", "rev-parse", "HEAD"], self.repo).stdout.strip()
        run(["git", "switch", "work"], self.repo)

        merged = self.codex_safe_git.merge_branch(str(self.repo), "codex/source")

        self.assertEqual(merged["result"], "ok")
        self.assertEqual(merged["source_branch"], "codex/source")
        self.assertEqual(merged["target_branch"], "work")
        self.assertEqual(merged["target_head_after"], source_head)
        self.assertEqual(merged["action"], "fast_forwarded")
        self.assertEqual(run(["git", "status", "--short"], self.repo).stdout, "")
        self.assertEqual(self.audit_entries()[-1]["action"], "merge_branch")
        self.assertNotIn("diff", self.audit_entries()[-1])
        self.assertNotIn("content", self.audit_entries()[-1])

    def test_merge_branch_fast_forwards_linked_worktree(self) -> None:
        run(["git", "switch", "-c", "codex/linked-source"], self.repo)
        self.write_file("linked-merge.txt", "linked merge\n")
        self.codex_safe_git.commit_files(
            str(self.repo),
            ["linked-merge.txt"],
            "Add linked merge source",
        )
        source_head = run(["git", "rev-parse", "HEAD"], self.repo).stdout.strip()
        run(["git", "switch", "work"], self.repo)
        linked = self.root / "linked-merge"
        run(["git", "worktree", "add", "-b", "codex/linked-target", str(linked), "HEAD"], self.repo)
        codex_safe_git = CodexSafeGit(
            CodexSafeGitConfig(
                allowed_repos=frozenset({self.repo.resolve(), linked.resolve()}),
                audit_log=self.audit_log,
            )
        )

        merged = codex_safe_git.merge_branch(str(linked), "codex/linked-source")

        self.assertEqual(merged["action"], "fast_forwarded")
        self.assertEqual(merged["target_branch"], "codex/linked-target")
        self.assertEqual(merged["target_head_after"], source_head)
        self.assertEqual(run(["git", "branch", "--show-current"], linked).stdout.strip(), "codex/linked-target")

    def test_merge_branch_refusals(self) -> None:
        run(["git", "branch", "codex/source", "HEAD"], self.repo)

        run(["git", "switch", "main"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "protected target branch: main"):
            self.codex_safe_git.merge_branch(str(self.repo), "codex/source")
        run(["git", "switch", "work"], self.repo)

        run(["git", "switch", "-c", "trunk"], self.repo)
        run(["git", "config", "init.defaultBranch", "trunk"], self.repo)
        with self.assertRaisesRegex(CodexSafeGitRefusal, "protected target branch: trunk"):
            self.codex_safe_git.merge_branch(str(self.repo), "codex/source")
        run(["git", "switch", "work"], self.repo)

        for branch in ("origin/feature", "../bad", "bad lock", "abcdef1"):
            with self.subTest(branch=branch):
                with self.assertRaises(CodexSafeGitRefusal):
                    self.codex_safe_git.merge_branch(str(self.repo), branch)

        with self.assertRaisesRegex(CodexSafeGitRefusal, "source_branch and target_branch must differ"):
            self.codex_safe_git.merge_branch(str(self.repo), "work")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "target_branch must match"):
            self.codex_safe_git.merge_branch(str(self.repo), "codex/source", "codex/other")

        self.write_file("dirty.txt", "dirty\n")
        with self.assertRaisesRegex(CodexSafeGitRefusal, "must be clean"):
            self.codex_safe_git.merge_branch(str(self.repo), "codex/source")

    def test_merge_branch_refuses_diverged_history_without_mutating(self) -> None:
        run(["git", "switch", "-c", "codex/source"], self.repo)
        self.write_file("source.txt", "source\n")
        self.codex_safe_git.commit_files(str(self.repo), ["source.txt"], "Advance source")
        run(["git", "switch", "work"], self.repo)
        self.write_file("target.txt", "target\n")
        self.codex_safe_git.commit_files(str(self.repo), ["target.txt"], "Advance target")
        before = run(["git", "rev-parse", "HEAD"], self.repo).stdout.strip()

        with self.assertRaisesRegex(CodexSafeGitRefusal, "ff-only|fast-forward|Not possible"):
            self.codex_safe_git.merge_branch(str(self.repo), "codex/source")

        self.assertEqual(run(["git", "rev-parse", "HEAD"], self.repo).stdout.strip(), before)
        self.assertEqual(run(["git", "status", "--short"], self.repo).stdout, "")

    def test_exact_file_staging_and_audit_after_branch_preparation(self) -> None:
        run(["git", "switch", "--detach", "HEAD"], self.repo)
        self.write_file("listed.txt", "listed\n")
        self.write_file("unlisted.txt", "unlisted\n")

        self.codex_safe_git.ensure_commit_branch(str(self.repo), "codex/exact")
        result = self.codex_safe_git.commit_files(str(self.repo), ["listed.txt"], "Add listed after prep")

        self.assertEqual(result["files"], ["listed.txt"])
        status = run(["git", "status", "--short"], self.repo).stdout
        self.assertIn("?? unlisted.txt", status)
        shown = run(["git", "show", "--name-only", "--format=%s", "HEAD"], self.repo).stdout
        self.assertIn("listed.txt", shown)
        self.assertNotIn("unlisted.txt", shown)
        entries = self.audit_entries()
        self.assertEqual(entries[-2]["action"], "ensure_commit_branch")
        self.assertEqual(entries[-2]["branch"], "codex/exact")
        self.assertEqual(entries[-1]["action"], "commit_files")
        self.assertEqual(entries[-1]["files"], ["listed.txt"])
        self.assertNotIn("diff", entries[-2])
        self.assertNotIn("content", entries[-1])

    def test_linked_worktree_on_non_default_branch_commits_successfully(self) -> None:
        linked = self.root / "linked"
        run(["git", "worktree", "add", "-b", "codex/linked", str(linked), "HEAD"], self.repo)
        codex_safe_git = CodexSafeGit(
            CodexSafeGitConfig(
                allowed_repos=frozenset({self.repo.resolve(), linked.resolve()}),
                audit_log=self.audit_log,
            )
        )
        self.write_file_in(linked, "linked.txt", "linked\n")

        result = codex_safe_git.commit_files(str(linked), ["linked.txt"], "Add linked worktree file")

        self.assertEqual(result["files"], ["linked.txt"])
        self.assertEqual(run(["git", "branch", "--show-current"], linked).stdout.strip(), "codex/linked")

    def test_detached_linked_worktree_can_be_prepared_then_committed(self) -> None:
        linked = self.root / "linked-detached"
        run(["git", "worktree", "add", "--detach", str(linked), "HEAD"], self.repo)
        codex_safe_git = CodexSafeGit(
            CodexSafeGitConfig(
                allowed_repos=frozenset({self.repo.resolve(), linked.resolve()}),
                audit_log=self.audit_log,
            )
        )
        self.write_file_in(linked, "detached-linked.txt", "linked detached\n")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "detached HEAD"):
            codex_safe_git.commit_files(str(linked), ["detached-linked.txt"], "Try detached linked")
        prepared = codex_safe_git.ensure_commit_branch(str(linked), "codex/linked-prepared")
        committed = codex_safe_git.commit_files(
            str(linked),
            ["detached-linked.txt"],
            "Add prepared linked worktree file",
        )

        self.assertEqual(prepared["action"], "created")
        self.assertEqual(committed["files"], ["detached-linked.txt"])
        self.assertEqual(
            run(["git", "branch", "--show-current"], linked).stdout.strip(),
            "codex/linked-prepared",
        )

    def test_status_and_diff_redact_secret_paths(self) -> None:
        self.write_file("visible.txt", "visible\n")
        self.write_file("private.pem", "PLACEHOLDER=value\n")

        status = self.codex_safe_git.git_status(str(self.repo))
        self.assertEqual(status["redacted_secret_path_count"], 1)
        self.assertEqual(status["entries"], [{"code": "??", "path": "visible.txt"}])

        summary = self.codex_safe_git.git_diff_summary(str(self.repo))
        self.assertEqual(summary["unstaged"]["files"], [])
        self.assertEqual(summary["unstaged"]["redacted_secret_path_count"], 0)
        self.assertEqual(summary["staged"]["files"], [])
        self.assertEqual(summary["staged"]["redacted_secret_path_count"], 0)
        self.assertEqual(summary["untracked"]["files"], ["visible.txt"])
        self.assertEqual(summary["untracked"]["redacted_secret_path_count"], 1)

    def test_refuses_execution_capable_git_config(self) -> None:
        run(["git", "config", "filter.secret.clean", "cat"], self.repo)
        self.write_file("normal.txt", "normal\n")

        with self.assertRaisesRegex(CodexSafeGitRefusal, "execution-capable Git config"):
            self.codex_safe_git.git_status(str(self.repo))
        with self.assertRaisesRegex(CodexSafeGitRefusal, "execution-capable Git config"):
            self.codex_safe_git.git_diff_summary(str(self.repo))
        with self.assertRaisesRegex(CodexSafeGitRefusal, "execution-capable Git config"):
            self.codex_safe_git.commit_files(str(self.repo), ["normal.txt"], "Try filter config")
