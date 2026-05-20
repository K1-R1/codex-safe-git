package gitpolicy_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"local/codex-safe-git/internal/audit"
	"local/codex-safe-git/internal/config"
	"local/codex-safe-git/internal/gitexec"
	"local/codex-safe-git/internal/gitpolicy"
	"local/codex-safe-git/internal/testrepo"
)

func TestCommitFilesCommitsOnlyExplicitFiles(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write("allowed.txt", "allowed\n")
	repo.Write("unlisted.txt", "unlisted\n")

	result, err := repo.Policy.CommitFiles(repo.Path, []string{"allowed.txt"}, "Add allowed file", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(result.Files, ","); got != "allowed.txt" {
		t.Fatalf("unexpected files: %s", got)
	}
	show := repo.Run("show", "--name-only", "--format=%s", "HEAD")
	if !strings.Contains(show, "allowed.txt") || strings.Contains(show, "unlisted.txt") {
		t.Fatalf("unexpected commit contents:\n%s", show)
	}
	if status := repo.Run("status", "--short"); !strings.Contains(status, "?? unlisted.txt") {
		t.Fatalf("unlisted file should remain untracked, got %q", status)
	}
}

func TestCommitFilesRefusesProtectedDefaultBranches(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("switch", "main")
	repo.Write("main.txt", "main\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"main.txt"}, "Try main commit", nil); !hasReason(err, "protected branch: main") {
		t.Fatalf("expected main refusal, got %v", err)
	}

	repo.Run("switch", "-c", "trunk")
	repo.Run("config", "init.defaultBranch", "trunk")
	repo.Write("trunk.txt", "trunk\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"trunk.txt"}, "Try trunk commit", nil); !hasReason(err, "protected branch: trunk") {
		t.Fatalf("expected trunk refusal, got %v", err)
	}

	repo.Run("switch", "-c", "develop")
	repo.Policy.Config.ProtectedBranches = map[string]struct{}{"develop": {}}
	repo.Write("develop.txt", "develop\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"develop.txt"}, "Try develop commit", nil); !hasReason(err, "protected branch: develop") {
		t.Fatalf("expected configured protected branch refusal, got %v", err)
	}
}

func TestCommitFilesSupportsTrackedDeletionsAndRenamePairs(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write("old/package.py", "same\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"old/package.py"}, "Add old package", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(repo.Abs("old/package.py")); err != nil {
		t.Fatal(err)
	}
	repo.Write("new/package.py", "same\n")

	result, err := repo.Policy.CommitFiles(repo.Path, []string{"old/package.py", "new/package.py"}, "Rename package path", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Files, ",") != "new/package.py,old/package.py" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
	if status := repo.Run("status", "--short"); status != "" {
		t.Fatalf("expected clean repo, got %q", status)
	}
}

func TestCommitFilesRefusesSymlinkWithoutCommittingTarget(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write("target.txt", "target\n")
	if err := os.Symlink("target.txt", repo.Abs("link.txt")); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"link.txt"}, "Add link", nil); !hasReason(err, "must not be a symlink") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
	status := repo.Run("status", "--short")
	if !strings.Contains(status, "?? link.txt") || !strings.Contains(status, "?? target.txt") {
		t.Fatalf("unexpected status after symlink refusal:\n%s", status)
	}
}

func TestCommitFilesTreatsPathspecMagicAsLiteral(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test filename uses POSIX pathspec characters")
	}
	repo := testrepo.New(t)
	magicName := ":(glob)*.txt"
	repo.Write(magicName, "literal\n")
	repo.Write("other.txt", "other\n")

	result, err := repo.Policy.CommitFiles(repo.Path, []string{magicName}, "Add literal pathspec file", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Files, ",") != magicName {
		t.Fatalf("unexpected committed files: %#v", result.Files)
	}
	show := repo.Run("show", "--name-only", "--format=%s", "HEAD")
	if !strings.Contains(show, magicName) || strings.Contains(show, "other.txt") {
		t.Fatalf("pathspec magic was not treated literally:\n%s", show)
	}
}

func TestRefusesUnallowlistedAndNonWorktreeRoot(t *testing.T) {
	repo := testrepo.New(t)
	outside := filepath.Join(repo.Root, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Policy.GitStatus(outside); !hasReason(err, "not explicitly allowlisted") {
		t.Fatalf("expected allowlist refusal, got %v", err)
	}

	runner, err := gitexec.NewRunner()
	if err != nil {
		t.Fatal(err)
	}
	rootPolicy := gitpolicy.Policy{
		Config: config.Config{
			AllowedRepos:     map[string]struct{}{},
			AllowedRepoRoots: map[string]struct{}{canonicalPath(t, repo.Root): {}},
			AuditLog:         repo.AuditLog,
		},
		Git:   runner,
		Audit: audit.Logger{Path: repo.AuditLog},
	}
	nested := filepath.Join(repo.Path, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := rootPolicy.GitStatus(nested); !hasReason(err, "must be the Git worktree root") {
		t.Fatalf("expected exact-root refusal, got %v", err)
	}
}

func TestSecretPathAndMaterialRefusals(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write(".env", "PLACEHOLDER=value\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{".env"}, "Try env", nil); !hasReason(err, "secret-bearing path") {
		t.Fatalf("expected secret path refusal, got %v", err)
	}
	repo.Write("config.txt", strings.Join([]string{"api", "_key = abcdefghijklmnop\n"}, ""))
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"config.txt"}, "Try secret material", nil); !hasReason(err, "secret material") {
		t.Fatalf("expected secret material refusal, got %v", err)
	}
	repo.Write("scanner.py", "LIKELY_SECRET = re.compile('placeholder')\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"scanner.py"}, "Add scanner source", nil); err != nil {
		t.Fatalf("identifier-contained keyword should be allowed: %v", err)
	}
}

func TestCommitFilesRescansStagedDiffBeforeCommit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell git wrapper")
	}
	repo := testrepo.New(t)
	repo.Write("racy.txt", "safe\n")

	runner, err := gitexec.NewRunner()
	if err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(repo.Root, "git")
	realGit := strings.ReplaceAll(runner.GitPath, "'", "'\\''")
	script := fmt.Sprintf(`#!/bin/sh
repo=""
saw_c=0
is_add=0
for arg in "$@"; do
  if [ "$saw_c" = "1" ]; then
    repo="$arg"
    saw_c=0
    continue
  fi
  if [ "$arg" = "-C" ]; then
    saw_c=1
    continue
  fi
  if [ "$arg" = "add" ]; then
    is_add=1
  fi
done
if [ "$is_add" = "1" ]; then
  key_name="$(printf 'api_%%s' 'key')"
  key_value="$(printf '%%s%%s' 'abcdefgh' 'ijklmnop')"
  printf '%%s = %%s\n' "$key_name" "$key_value" > "$repo/racy.txt"
fi
exec '%s' "$@"
`, realGit)
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	repo.Policy.Git = gitexec.Runner{GitPath: wrapper, Timeout: gitexec.DefaultTimeout}

	before := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"racy.txt"}, "Add racy file", nil); !hasReason(err, "staged diff appears to contain secret material") {
		t.Fatalf("expected staged secret refusal, got %v", err)
	}
	after := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if before != after {
		t.Fatalf("commit happened despite staged secret refusal: before %s after %s", before, after)
	}
	if staged := repo.Run("diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("staged file should have been cleared after refusal, got %q", staged)
	}
}

func TestStatusAndDiffRedactRenameSecretSides(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write("normal/public.txt", "public\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"normal/public.txt"}, "Add public file", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo.Abs(".ssh"), 0o755); err != nil {
		t.Fatal(err)
	}
	repo.Run("mv", "normal/public.txt", ".ssh/public.txt")

	status, err := repo.Policy.GitStatus(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if status.RedactedSecretPathCount != 1 || len(status.Entries) != 0 {
		t.Fatalf("expected secret rename redaction, got %#v", status)
	}
	diff, err := repo.Policy.GitDiffSummary(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Staged.RedactedSecretPathCount != 1 || len(diff.Staged.Files) != 0 {
		t.Fatalf("expected staged secret rename redaction, got %#v", diff.Staged)
	}
}

func TestStatusAndDiffSummariesAreBoundedWithCounts(t *testing.T) {
	repo := testrepo.New(t)
	for index := 0; index < gitpolicy.StatusEntryLimit+5; index++ {
		repo.Write(fmt.Sprintf("bulk-%03d.txt", index), "bulk\n")
	}
	repo.Write(".env", "PLACEHOLDER=value\n")

	status, err := repo.Policy.GitStatus(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if status.EntryCount != gitpolicy.StatusEntryLimit+5 || len(status.Entries) != gitpolicy.StatusEntryLimit || !status.EntriesTruncated {
		t.Fatalf("expected bounded status entries, got count=%d len=%d truncated=%v", status.EntryCount, len(status.Entries), status.EntriesTruncated)
	}
	if status.EntryLimit != gitpolicy.StatusEntryLimit || status.RedactedSecretPathCount != 1 {
		t.Fatalf("unexpected status limit/redaction metadata: %#v", status)
	}

	diff, err := repo.Policy.GitDiffSummary(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Untracked.FileCount != gitpolicy.UntrackedLimit+5 || len(diff.Untracked.Files) != gitpolicy.UntrackedLimit || !diff.Untracked.FilesTruncated {
		t.Fatalf("expected bounded untracked files, got %#v", diff.Untracked)
	}
	if diff.Untracked.FileLimit != gitpolicy.UntrackedLimit || diff.Untracked.RedactedSecretPathCount != 1 {
		t.Fatalf("unexpected untracked limit/redaction metadata: %#v", diff.Untracked)
	}
}

func TestCommitFilesRefusesTooManyFiles(t *testing.T) {
	repo := testrepo.New(t)
	files := make([]string, gitpolicy.CommitFileLimit+1)
	for index := range files {
		files[index] = fmt.Sprintf("bulk-%03d.txt", index)
	}
	if _, err := repo.Policy.CommitFiles(repo.Path, files, "Too many files", nil); !hasReason(err, "too many requested files") {
		t.Fatalf("expected file limit refusal, got %v", err)
	}
}

func TestGlobalExecutionConfigIsIgnored(t *testing.T) {
	repo := testrepo.New(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[filter \"bad\"]\n\tclean = cat\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Policy.GitStatus(repo.Path); err != nil {
		t.Fatalf("global Git config should be ignored, got %v", err)
	}
}

func TestBadMessageDetachedAndStagedStateRefusals(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write("readme.txt", "readme\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"readme.txt"}, "Generated by Codex", nil); !hasReason(err, "AI/tool attribution") {
		t.Fatalf("expected attribution refusal, got %v", err)
	}
	repo.Run("switch", "--detach", "HEAD")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"readme.txt"}, "Try detached", nil); !hasReason(err, "detached HEAD") {
		t.Fatalf("expected detached refusal, got %v", err)
	}
	repo.Run("switch", "work")
	repo.Write("staged.txt", "staged\n")
	repo.Run("add", "staged.txt")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"readme.txt"}, "Try staged", nil); !hasReason(err, "already has staged changes") {
		t.Fatalf("expected staged refusal, got %v", err)
	}
}

func TestEnsureCommitBranchPreparesDetachedHead(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("switch", "--detach", "HEAD")
	repo.Write("prepared.txt", "prepared\n")

	prepared, err := repo.Policy.EnsureCommitBranch(repo.Path, "codex/prepared")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Action != "created" || prepared.Branch != "codex/prepared" {
		t.Fatalf("unexpected branch result: %#v", prepared)
	}
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"prepared.txt"}, "Add prepared file", nil); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureCommitBranchRefusals(t *testing.T) {
	repo := testrepo.New(t)
	for _, branch := range []string{"main", "master", "../bad", "origin/feature", "bad lock", "abcdef1"} {
		if _, err := repo.Policy.EnsureCommitBranch(repo.Path, branch); err == nil {
			t.Fatalf("expected refusal for branch %q", branch)
		}
	}
	if _, err := repo.Policy.EnsureCommitBranch(repo.Path, "codex/other"); !hasReason(err, "different branch: work") {
		t.Fatalf("expected different-branch refusal, got %v", err)
	}
}

func TestCreateCommitBranchAndRefusals(t *testing.T) {
	repo := testrepo.New(t)
	created, err := repo.Policy.CreateCommitBranch(repo.Path, "codex/new-work")
	if err != nil {
		t.Fatal(err)
	}
	if created.Action != "created" {
		t.Fatalf("unexpected action: %s", created.Action)
	}
	repeated, err := repo.Policy.CreateCommitBranch(repo.Path, "codex/new-work")
	if err != nil {
		t.Fatal(err)
	}
	if repeated.Action != "already_on_branch" {
		t.Fatalf("unexpected repeated action: %s", repeated.Action)
	}
	if _, err := repo.Policy.CreateCommitBranch(repo.Path, "main"); !hasReason(err, "protected branch: main") {
		t.Fatalf("expected protected branch refusal, got %v", err)
	}
}

func TestMergeBranchFastForwardsAndRefusesProtectedDirtyAndDiverged(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("switch", "-c", "codex/source")
	repo.Write("merge.txt", "merge\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"merge.txt"}, "Add merge source", nil); err != nil {
		t.Fatal(err)
	}
	sourceHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Run("switch", "work")

	merged, err := repo.Policy.MergeBranch(repo.Path, "codex/source", nil)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Action != "fast_forwarded" || merged.TargetHeadAfter != sourceHead {
		t.Fatalf("unexpected merge result: %#v", merged)
	}

	repo.Run("switch", "main")
	if _, err := repo.Policy.MergeBranch(repo.Path, "codex/source", nil); !hasReason(err, "protected target branch: main") {
		t.Fatalf("expected protected target refusal, got %v", err)
	}
	repo.Run("switch", "work")
	repo.Write("dirty.txt", "dirty\n")
	if _, err := repo.Policy.MergeBranch(repo.Path, "codex/source", nil); !hasReason(err, "must be clean") {
		t.Fatalf("expected dirty refusal, got %v", err)
	}
}

func TestMergeBranchRefusesDivergedHistoryWithoutMutating(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("switch", "-c", "codex/source")
	repo.Write("source.txt", "source\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"source.txt"}, "Advance source", nil); err != nil {
		t.Fatal(err)
	}
	repo.Run("switch", "work")
	repo.Write("target.txt", "target\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"target.txt"}, "Advance target", nil); err != nil {
		t.Fatal(err)
	}
	before := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if _, err := repo.Policy.MergeBranch(repo.Path, "codex/source", nil); err == nil {
		t.Fatal("expected non-fast-forward refusal")
	}
	after := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if before != after {
		t.Fatalf("merge mutated target: before %s after %s", before, after)
	}
}

func TestLinkedWorktreeBranchAndMerge(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("switch", "-c", "codex/linked-source")
	repo.Write("linked-merge.txt", "linked\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"linked-merge.txt"}, "Add linked source", nil); err != nil {
		t.Fatal(err)
	}
	sourceHead := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Run("switch", "work")
	linked := filepath.Join(repo.Root, "linked")
	repo.Run("worktree", "add", "-b", "codex/linked-target", linked, "HEAD")

	runner, err := gitexec.NewRunner()
	if err != nil {
		t.Fatal(err)
	}
	policy := gitpolicy.Policy{
		Config: config.Config{
			AllowedRepos: map[string]struct{}{
				canonicalPath(t, repo.Path): {},
				canonicalPath(t, linked):    {},
			},
			AuditLog: repo.AuditLog,
		},
		Git:   runner,
		Audit: audit.Logger{Path: repo.AuditLog},
	}
	merged, err := policy.MergeBranch(linked, "codex/linked-source", nil)
	if err != nil {
		t.Fatal(err)
	}
	if merged.TargetHeadAfter != sourceHead {
		t.Fatalf("unexpected linked merge result: %#v", merged)
	}
}

func TestCreateWorktreeListAndSafeCheckout(t *testing.T) {
	repo := testrepo.New(t)
	policy := rootAllowedPolicy(t, repo)
	target := filepath.Join(repo.Root, "linked-feature")

	created, err := policy.CreateWorktree(repo.Path, target, "codex/linked-feature", nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.WorktreePath != canonicalPath(t, target) || created.Branch != "codex/linked-feature" || created.Action != "created" {
		t.Fatalf("unexpected create worktree result: %#v", created)
	}
	if created.BaseBranch != nil || created.BaseHead == "" || created.HeadCommit != created.BaseHead {
		t.Fatalf("unexpected base metadata: %#v", created)
	}

	listed, err := policy.ListWorktrees(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if listed.RedactedUnallowlistedCount != 0 {
		t.Fatalf("did not expect redacted worktrees: %#v", listed)
	}
	if !containsWorktree(listed.Worktrees, canonicalPath(t, repo.Path), true) {
		t.Fatalf("missing current worktree: %#v", listed.Worktrees)
	}
	if !containsWorktree(listed.Worktrees, canonicalPath(t, target), false) {
		t.Fatalf("missing linked worktree: %#v", listed.Worktrees)
	}

	repo.Run("branch", "codex/checkout-target")
	checkout, err := policy.SafeCheckout(target, "codex/checkout-target")
	if err != nil {
		t.Fatal(err)
	}
	if checkout.Action != "switched" || checkout.Branch != "codex/checkout-target" {
		t.Fatalf("unexpected checkout result: %#v", checkout)
	}
}

func TestListWorktreesRedactsUnallowlistedPaths(t *testing.T) {
	repo := testrepo.New(t)
	linked := filepath.Join(repo.Root, "linked-redacted")
	repo.Run("worktree", "add", "-b", "codex/redacted", linked, "HEAD")

	listed, err := repo.Policy.ListWorktrees(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if listed.RedactedUnallowlistedCount != 1 {
		t.Fatalf("expected one redacted worktree, got %#v", listed)
	}
	for _, worktree := range listed.Worktrees {
		if samePathForTest(t, worktree.Path, linked) {
			t.Fatalf("unallowlisted worktree path leaked: %#v", listed.Worktrees)
		}
	}
}

func TestWorktreeToolsRefuseUnsafeInputsAndStates(t *testing.T) {
	repo := testrepo.New(t)
	policy := rootAllowedPolicy(t, repo)

	repo.Write("dirty.txt", "dirty\n")
	if _, err := policy.CreateWorktree(repo.Path, filepath.Join(repo.Root, "dirty-target"), "codex/dirty", nil); !hasReason(err, "must be clean") {
		t.Fatalf("expected dirty refusal, got %v", err)
	}
	if err := os.Remove(repo.Abs("dirty.txt")); err != nil {
		t.Fatal(err)
	}

	if _, err := policy.CreateWorktree(repo.Path, filepath.Join(t.TempDir(), "outside"), "codex/outside", nil); !hasReason(err, "not explicitly allowlisted") {
		t.Fatalf("expected outside target refusal, got %v", err)
	}
	existing := filepath.Join(repo.Root, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.CreateWorktree(repo.Path, existing, "codex/existing", nil); !hasReason(err, "already exists") {
		t.Fatalf("expected existing target refusal, got %v", err)
	}
	if _, err := policy.CreateWorktree(repo.Path, filepath.Join(repo.Root, "main-target"), "main", nil); !hasReason(err, "protected branch: main") {
		t.Fatalf("expected protected branch refusal, got %v", err)
	}
	if err := os.Mkdir(filepath.Join(repo.Root, ".ssh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := policy.CreateWorktree(repo.Path, filepath.Join(repo.Root, ".ssh", "secret-target"), "codex/secret-path", nil); !hasReason(err, "secret-bearing path") {
		t.Fatalf("expected secret-bearing target refusal, got %v", err)
	}
	repo.Run("branch", "codex/already-exists")
	if _, err := policy.CreateWorktree(repo.Path, filepath.Join(repo.Root, "duplicate-branch"), "codex/already-exists", nil); !hasReason(err, "branch already exists") {
		t.Fatalf("expected duplicate branch refusal, got %v", err)
	}
}

func TestSafeCheckoutRefusals(t *testing.T) {
	repo := testrepo.New(t)
	policy := rootAllowedPolicy(t, repo)
	repo.Run("branch", "codex/feature")
	if _, err := policy.SafeCheckout(repo.Path, "main"); !hasReason(err, "protected branch: main") {
		t.Fatalf("expected protected branch refusal, got %v", err)
	}
	if _, err := policy.SafeCheckout(repo.Path, "codex/missing"); !hasReason(err, "does not exist") {
		t.Fatalf("expected missing branch refusal, got %v", err)
	}

	linked := filepath.Join(repo.Root, "checkout-linked")
	repo.Run("worktree", "add", "-b", "codex/checked-out-elsewhere", linked, "HEAD")
	if _, err := policy.SafeCheckout(repo.Path, "codex/checked-out-elsewhere"); !hasReason(err, "already checked out") {
		t.Fatalf("expected checked-out-elsewhere refusal, got %v", err)
	}

	if _, err := policy.SafeCheckout(repo.Path, "codex/feature"); err != nil {
		t.Fatal(err)
	}
	repo.Write("dirty.txt", "dirty\n")
	if _, err := policy.SafeCheckout(repo.Path, "work"); !hasReason(err, "must be clean") {
		t.Fatalf("expected dirty checkout refusal, got %v", err)
	}
}

func TestUnsafeGitExecutionConfigRefused(t *testing.T) {
	repo := testrepo.New(t)
	repo.Run("config", "filter.bad.clean", "cat")
	if _, err := repo.Policy.GitStatus(repo.Path); !hasReason(err, "execution-capable Git config") {
		t.Fatalf("expected execution config refusal, got %v", err)
	}
}

func TestWorktreeMutationsFailClosedWhenAuditLogUnavailable(t *testing.T) {
	repo := testrepo.New(t)
	policy := rootAllowedPolicy(t, repo)
	policy.Audit.Path = repo.Root
	target := filepath.Join(repo.Root, "blocked-worktree")

	if _, err := policy.CreateWorktree(repo.Path, target, "codex/blocked-worktree", nil); !hasReason(err, "audit log is not writable") {
		t.Fatalf("expected audit refusal, got %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("worktree should not exist after audit refusal: %v", err)
	}
	if head := strings.TrimSpace(repo.Run("symbolic-ref", "--short", "HEAD")); head != "work" {
		t.Fatalf("branch changed despite audit refusal: %s", head)
	}

	repo.Run("branch", "codex/no-audit-checkout")
	if _, err := policy.SafeCheckout(repo.Path, "codex/no-audit-checkout"); !hasReason(err, "audit log is not writable") {
		t.Fatalf("expected checkout audit refusal, got %v", err)
	}
	if head := strings.TrimSpace(repo.Run("symbolic-ref", "--short", "HEAD")); head != "work" {
		t.Fatalf("checkout happened despite audit refusal: %s", head)
	}
}

func TestBranchAndMergeMutationsFailClosedWhenAuditLogUnavailable(t *testing.T) {
	repo := testrepo.New(t)
	policy := repo.Policy
	policy.Audit.Path = repo.Root

	if _, err := policy.CreateCommitBranch(repo.Path, "codex/no-audit-branch"); !hasReason(err, "audit log is not writable") {
		t.Fatalf("expected branch audit refusal, got %v", err)
	}
	if head := strings.TrimSpace(repo.Run("symbolic-ref", "--short", "HEAD")); head != "work" {
		t.Fatalf("branch changed despite audit refusal: %s", head)
	}
	if branch := strings.TrimSpace(repo.Run("branch", "--list", "codex/no-audit-branch")); branch != "" {
		t.Fatalf("branch was created despite audit refusal: %q", branch)
	}

	repo.Run("switch", "-c", "codex/audit-source")
	repo.Write("merge-audit.txt", "merge\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"merge-audit.txt"}, "Add merge audit source", nil); err != nil {
		t.Fatal(err)
	}
	repo.Run("switch", "work")
	before := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if _, err := policy.MergeBranch(repo.Path, "codex/audit-source", nil); !hasReason(err, "audit log is not writable") {
		t.Fatalf("expected merge audit refusal, got %v", err)
	}
	after := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if before != after {
		t.Fatalf("merge happened despite audit refusal: before %s after %s", before, after)
	}
}

func TestAuditMetadataOnly(t *testing.T) {
	repo := testrepo.New(t)
	repo.Write("listed.txt", "listed\n")
	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"listed.txt"}, "Add listed", nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(repo.AuditLog)
	if err != nil {
		t.Fatal(err)
	}
	var entry map[string]any
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"diff", "content", "env", "credential"} {
		if _, ok := entry[forbidden]; ok {
			t.Fatalf("audit entry contains forbidden key %q: %#v", forbidden, entry)
		}
	}
}

func TestCommitFilesFailsClosedWhenAuditLogUnavailable(t *testing.T) {
	repo := testrepo.New(t)
	repo.Policy.Audit.Path = repo.Root
	before := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	repo.Write("blocked.txt", "blocked\n")

	if _, err := repo.Policy.CommitFiles(repo.Path, []string{"blocked.txt"}, "Try blocked audit", nil); !hasReason(err, "audit log is not writable") {
		t.Fatalf("expected audit refusal, got %v", err)
	}
	after := strings.TrimSpace(repo.Run("rev-parse", "HEAD"))
	if before != after {
		t.Fatalf("commit happened despite audit failure: before %s after %s", before, after)
	}
	if status := repo.Run("status", "--short"); !strings.Contains(status, "?? blocked.txt") {
		t.Fatalf("file should remain uncommitted after audit failure, got %q", status)
	}
}

func hasReason(err error, fragment string) bool {
	return err != nil && strings.Contains(err.Error(), fragment)
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}

func rootAllowedPolicy(t *testing.T, repo testrepo.Repo) gitpolicy.Policy {
	t.Helper()
	runner, err := gitexec.NewRunner()
	if err != nil {
		t.Fatal(err)
	}
	return gitpolicy.Policy{
		Config: config.Config{
			AllowedRepos:     map[string]struct{}{},
			AllowedRepoRoots: map[string]struct{}{canonicalPath(t, repo.Root): {}},
			AuditLog:         repo.AuditLog,
		},
		Git:   runner,
		Audit: audit.Logger{Path: repo.AuditLog},
	}
}

func containsWorktree(entries []gitpolicy.WorktreeEntry, path string, current bool) bool {
	for _, entry := range entries {
		if entry.Path == path && entry.IsCurrent == current {
			return true
		}
	}
	return false
}

func samePathForTest(t *testing.T, left, right string) bool {
	t.Helper()
	return canonicalPath(t, left) == canonicalPath(t, right)
}
