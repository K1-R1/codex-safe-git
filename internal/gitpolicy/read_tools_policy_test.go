package gitpolicy_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K1-R1/codex-safe-git/internal/gitpolicy"
	"github.com/K1-R1/codex-safe-git/internal/testrepo"
)

func TestReadOnlyBranchRefAndHistoryTools(t *testing.T) {
	repo := testrepo.New(t)
	workHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Run("switch", "-c", "codex/topic")
	repo.Write("topic.txt", "topic\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"topic.txt"}, "Add topic", nil); err != nil {
		t.Fatal(err)
	}
	topicHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Run("switch", "work")

	branches, err := repo.Policy.ListLocalBranches(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if branches.BranchCount < 3 || !branchListed(branches.Branches, "main", true) || !branchListed(branches.Branches, "work", false) {
		t.Fatalf("unexpected branches: %#v", branches)
	}

	refs, err := repo.Policy.ListLocalRefs(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !refListed(refs.Refs, "refs/heads/codex/topic", "branch") {
		t.Fatalf("missing topic ref: %#v", refs.Refs)
	}

	base, err := repo.Policy.MergeBase(repo.Path, "work", "codex/topic")
	if err != nil {
		t.Fatal(err)
	}
	if base.MergeBase != workHead || !base.IsAncestor {
		t.Fatalf("unexpected merge-base result: %#v", base)
	}

	compare, err := repo.Policy.CompareRefs(repo.Path, "work", "codex/topic")
	if err != nil {
		t.Fatal(err)
	}
	if compare.AheadCount != 1 || compare.BehindCount != 0 || compare.ChangedFileCount != 1 || compare.TargetCommit != topicHead {
		t.Fatalf("unexpected compare result: %#v", compare)
	}

	changed, err := repo.Policy.ChangedFilesBetweenRefs(repo.Path, "work", "codex/topic")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changed.Changed.Files, ",") != "topic.txt" {
		t.Fatalf("unexpected changed files: %#v", changed.Changed)
	}

	logSummary, err := repo.Policy.CommitLogSummary(repo.Path, stringPtr("codex/topic"), intPtr(2))
	if err != nil {
		t.Fatal(err)
	}
	if logSummary.CommitCount != 2 || logSummary.Commits[0].Hash != topicHead {
		t.Fatalf("unexpected log summary: %#v", logSummary)
	}

	show, err := repo.Policy.ShowCommitSummary(repo.Path, "codex/topic")
	if err != nil {
		t.Fatal(err)
	}
	if show.Commit.Hash != topicHead || strings.Join(show.Commit.ChangedFiles, ",") != "topic.txt" {
		t.Fatalf("unexpected commit summary: %#v", show)
	}

	if _, err := repo.Policy.CompareRefs(repo.Path, "origin/main", "codex/topic"); !hasReason(err, "remote/ref path") {
		t.Fatalf("expected remote ref refusal, got %v", err)
	}
	if _, err := repo.Policy.CommitLogSummary(repo.Path, stringPtr("codex/topic"), intPtr(gitpolicy.LogCommitLimit+1)); !hasReason(err, "limit exceeds maximum") {
		t.Fatalf("expected limit refusal, got %v", err)
	}
}

func TestPathStatusRedactionIgnoreAndBounds(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write(".gitignore", "ignored.log\n")
	repo.Write("tracked.txt", "tracked\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{".gitignore", "tracked.txt"}, "Add tracked and ignore", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(repo.Abs("tracked.txt")); err != nil {
		t.Fatal(err)
	}
	repo.Write("ignored.log", "ignored\n")
	repo.Write(".env", "PLACEHOLDER=value\n")
	repo.Write(":(glob)*.txt", "literal\n")
	if err := os.Mkdir(repo.Abs("real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", repo.Abs("linkdir")); err != nil {
		t.Fatal(err)
	}

	status, err := repo.Policy.PathStatus(repo.Path, []string{"tracked.txt", "ignored.log", ".env", "linkdir/file.txt", ":(glob)*.txt"}, boolPtr(true))
	if err != nil {
		t.Fatal(err)
	}
	if status.RedactedSecretPathCount != 1 || status.EntryCount != 4 {
		t.Fatalf("unexpected path-status counts: %#v", status)
	}
	tracked := pathStatusEntry(status.Entries, "tracked.txt")
	if tracked == nil || !tracked.Tracked || !tracked.Deleted {
		t.Fatalf("expected tracked deletion metadata, got %#v", tracked)
	}
	ignored := pathStatusEntry(status.Entries, "ignored.log")
	if ignored == nil || !ignored.Ignored || ignored.IgnoreSource == nil {
		t.Fatalf("expected ignored metadata, got %#v", ignored)
	}
	linkChild := pathStatusEntry(status.Entries, "linkdir/file.txt")
	if linkChild == nil || !linkChild.HasSymlinkComponents || linkChild.Exists {
		t.Fatalf("expected symlink component metadata, got %#v", linkChild)
	}
	magic := pathStatusEntry(status.Entries, ":(glob)*.txt")
	if magic == nil || !magic.Exists || !magic.Untracked {
		t.Fatalf("expected literal pathspec metadata, got %#v", magic)
	}

	tooMany := make([]string, gitpolicy.PathStatusLimit+1)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("path-%03d.txt", index)
	}
	if _, err := repo.Policy.PathStatus(repo.Path, tooMany, nil); !hasReason(err, "too many requested paths") {
		t.Fatalf("expected path limit refusal, got %v", err)
	}
}

func TestPathStatusDoesNotFollowSymlinkComponents(t *testing.T) {
	repo := testrepo.New(t)
	outside := filepath.Join(repo.Root, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "probe.txt"), []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, repo.Abs("linkout")); err != nil {
		t.Fatal(err)
	}

	status, err := repo.Policy.PathStatus(repo.Path, []string{"linkout/probe.txt"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	entry := pathStatusEntry(status.Entries, "linkout/probe.txt")
	if entry == nil {
		t.Fatalf("missing path status entry: %#v", status)
	}
	if !entry.HasSymlinkComponents || entry.Exists || entry.IsDir || entry.IsSymlink {
		t.Fatalf("path_status followed symlink component: %#v", entry)
	}
}

func TestCommitSummaryBoundsAndSecretPathRedaction(t *testing.T) {
	repo := testrepo.New(t)
	for index := 0; index < gitpolicy.CommitChangedFileLimit+5; index++ {
		repo.Write(fmt.Sprintf("bulk/raw-%03d.txt", index), "bulk\n")
	}
	repo.Write(".env", "PLACEHOLDER=value\n")
	repo.Run("add", "bulk")
	repo.Run("add", "-f", ".env")
	repo.Run("commit", "-m", "Raw bulk commit")

	summary, err := repo.Policy.ShowCommitSummary(repo.Path, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if summary.Commit.ChangedFileCount != gitpolicy.CommitChangedFileLimit+5 || !summary.Commit.ChangedFilesTruncated {
		t.Fatalf("expected bounded changed files, got %#v", summary.Commit)
	}
	if summary.Commit.RedactedSecretPathCount != 1 {
		t.Fatalf("expected secret path redaction, got %#v", summary.Commit)
	}
	for _, path := range summary.Commit.ChangedFiles {
		if path == ".env" {
			t.Fatalf("secret path leaked in changed files: %#v", summary.Commit.ChangedFiles)
		}
	}
}

func TestReadOnlyToolsDoNotWriteAuditLog(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("switch", "-c", "codex/read-only-topic")
	repo.Write("topic.txt", "topic\n")
	repo.Run("add", "topic.txt")
	repo.Run("commit", "-m", "Add read-only topic")
	topicHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Run("switch", "work")

	if _, err := repo.Policy.GitStatus(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.GitDiffSummary(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.ListLocalBranches(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.ListLocalRefs(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.MergeBase(repo.Path, "work", "codex/read-only-topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.CompareRefs(repo.Path, "work", "codex/read-only-topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.ChangedFilesBetweenRefs(repo.Path, "work", "codex/read-only-topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.CommitLogSummary(repo.Path, stringPtr("codex/read-only-topic"), intPtr(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.ShowCommitSummary(repo.Path, topicHead); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.PathStatus(repo.Path, []string{"README.md"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.SubmoduleSummary(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.RepositoryIntegrityCheck(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.ReflogSummary(repo.Path, nil, intPtr(2)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.ListWorktrees(repo.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.SelfCheck(repo.Path, "test-version", "test-protocol", []string{"git_status"}); err != nil {
		t.Fatal(err)
	}

	for name, call := range map[string]func() error{
		"compare_refs":       func() error { _, err := repo.Policy.CompareRefs(repo.Path, "-bad", "HEAD"); return err },
		"commit_log_summary": func() error { _, err := repo.Policy.CommitLogSummary(repo.Path, stringPtr("abcdef1"), nil); return err },
		"changed_files_between_refs": func() error {
			_, err := repo.Policy.ChangedFilesBetweenRefs(repo.Path, "HEAD", "origin/main")
			return err
		},
		"path_status": func() error { _, err := repo.Policy.PathStatus(repo.Path, []string{"../outside"}, nil); return err },
		"reflog_summary": func() error {
			_, err := repo.Policy.ReflogSummary(repo.Path, stringPtr("origin/main"), nil)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Fatalf("expected read-only refusal from %s", name)
		}
	}
	if _, err := os.Stat(repo.AuditLog); !os.IsNotExist(err) {
		t.Fatalf("read-only tools should not write audit log, stat error: %v", err)
	}
}

func TestReadOnlyRefsSupportSHA256ObjectFormat(t *testing.T) {
	t.Setenv("GIT_DEFAULT_HASH", "sha256")
	repo := testrepo.New(t)
	workHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if len(workHead) != 64 {
		t.Skip("git did not create a SHA-256 test repository")
	}
	repo.Run("switch", "-c", "codex/sha256-topic")
	repo.Write("sha256.txt", "sha256\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"sha256.txt"}, "Add sha256 file", nil); err != nil {
		t.Fatal(err)
	}
	topicHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Run("switch", "work")

	base, err := repo.Policy.MergeBase(repo.Path, workHead, topicHead)
	if err != nil {
		t.Fatal(err)
	}
	if base.MergeBase != workHead || len(base.LeftCommit) != 64 || len(base.RightCommit) != 64 {
		t.Fatalf("unexpected SHA-256 merge-base result: %#v", base)
	}

	compare, err := repo.Policy.CompareRefs(repo.Path, "work", topicHead)
	if err != nil {
		t.Fatal(err)
	}
	if compare.TargetCommit != topicHead || compare.AheadCount != 1 {
		t.Fatalf("unexpected SHA-256 compare result: %#v", compare)
	}

	show, err := repo.Policy.ShowCommitSummary(repo.Path, topicHead)
	if err != nil {
		t.Fatal(err)
	}
	if show.Commit.Hash != topicHead || strings.Join(show.Commit.ChangedFiles, ",") != "sha256.txt" {
		t.Fatalf("unexpected SHA-256 commit summary: %#v", show)
	}
}

func TestSubmoduleIntegrityReflogAndSelfCheck(t *testing.T) {
	repo := testrepo.New(t)
	childDir := filepath.Join(repo.Root, "child")
	testrepo.Run(t, "", "git", "init", "-b", "main", childDir)
	testrepo.Run(t, childDir, "git", "config", "user.name", "Codex Safe Git Test")
	testrepo.Run(t, childDir, "git", "config", "user.email", "codex-safe-git@example.invalid")
	testrepo.Write(t, childDir, "child.txt", "child\n")
	testrepo.Run(t, childDir, "git", "add", "child.txt")
	testrepo.Run(t, childDir, "git", "commit", "-m", "Initial child")

	repo.Run("-c", "protocol.file.allow=always", "submodule", "add", childDir, "vendor/child")
	repo.Run("add", ".gitmodules", "vendor/child")
	repo.Run("commit", "-m", "Add submodule")
	testrepo.Write(t, repo.Abs("vendor/child"), "dirty.txt", "dirty\n")

	submodules, err := repo.Policy.SubmoduleSummary(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if submodules.SubmoduleCount != 1 || submodules.Submodules[0].Path != "vendor/child" || !submodules.Submodules[0].Dirty {
		t.Fatalf("unexpected submodule summary: %#v", submodules)
	}

	integrity, err := repo.Policy.RepositoryIntegrityCheck(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !integrity.IntegrityOK || integrity.ExitCode != 0 || integrity.IssueCount != 0 {
		t.Fatalf("unexpected integrity result: %#v", integrity)
	}

	reflog, err := repo.Policy.ReflogSummary(repo.Path, nil, intPtr(5))
	if err != nil {
		t.Fatal(err)
	}
	if reflog.Ref != "HEAD" || reflog.EntryCount == 0 || reflog.EntryLimit != 5 {
		t.Fatalf("unexpected reflog summary: %#v", reflog)
	}

	self, err := repo.Policy.SelfCheck(repo.Path, "test-version", "test-protocol", []string{"b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if self.ServerVersion != "test-version" || self.ProtocolVersion != "test-protocol" || self.ToolCount != 2 {
		t.Fatalf("unexpected self check: %#v", self)
	}
	if self.AllowedRepoCount != 1 || len(self.AllowlistFingerprints) != 1 || strings.Contains(strings.Join(self.AllowlistFingerprints, ","), repo.Path) {
		t.Fatalf("allowlist fingerprints should be redacted hashes: %#v", self)
	}
}

func TestSubmoduleSummarySupportsPathsWithSpaces(t *testing.T) {
	repo := testrepo.New(t)
	childDir := filepath.Join(repo.Root, "child spaced")
	testrepo.Run(t, "", "git", "init", "-b", "main", childDir)
	testrepo.Run(t, childDir, "git", "config", "user.name", "Codex Safe Git Test")
	testrepo.Run(t, childDir, "git", "config", "user.email", "codex-safe-git@example.invalid")
	testrepo.Write(t, childDir, "child.txt", "child\n")
	testrepo.Run(t, childDir, "git", "add", "child.txt")
	testrepo.Run(t, childDir, "git", "commit", "-m", "Initial child")

	repo.Run("-c", "protocol.file.allow=always", "submodule", "add", childDir, "vendor/child module")
	repo.Run("add", ".gitmodules", "vendor/child module")
	repo.Run("commit", "-m", "Add spaced submodule")
	testrepo.Write(t, repo.Abs("vendor/child module"), "dirty.txt", "dirty\n")

	submodules, err := repo.Policy.SubmoduleSummary(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if submodules.SubmoduleCount != 1 || submodules.Submodules[0].Path != "vendor/child module" || !submodules.Submodules[0].Dirty {
		t.Fatalf("unexpected submodule summary: %#v", submodules)
	}
}
