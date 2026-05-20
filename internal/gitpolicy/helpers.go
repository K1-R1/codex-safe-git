package gitpolicy

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"local/codex-safe-git/internal/secretcheck"
)

type repoState struct {
	Branch           *string
	AmbiguousReasons []string
	HasStagedChanges bool
}

func refuse(reason string) Refusal {
	return Refusal{Reason: reason}
}

func (p Policy) auditRefusal(action, repo string, err error) {
	p.Audit.WriteAuditCompat(action, repo, err)
}

func (p Policy) resolveAllowedRepo(repoPath string) (string, error) {
	repo, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}
	repo = filepath.Clean(repo)
	if resolved, err := filepath.EvalSymlinks(repo); err == nil {
		repo = filepath.Clean(resolved)
	}
	if _, ok := p.Config.AllowedRepos[repo]; ok {
		return repo, nil
	}
	for root := range p.Config.AllowedRepoRoots {
		if pathWithin(repo, root) {
			return repo, nil
		}
	}
	return "", refuse("repo_path is not explicitly allowlisted or under an allowed repo root")
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func (p Policy) requireGitRepo(repo string) error {
	info, err := os.Stat(repo)
	if err != nil || !info.IsDir() {
		return refuse("repo_path does not exist or is not a directory")
	}
	inside, err := p.Git.RunAllowFailure(repo, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return err
	}
	if inside.ExitCode != 0 || strings.TrimSpace(inside.Stdout) != "true" {
		return refuse("repo_path is not a Git worktree")
	}
	top, err := p.Git.Run(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	topPath, err := filepath.Abs(strings.TrimSpace(top.Stdout))
	if err != nil {
		return err
	}
	topPath = filepath.Clean(topPath)
	if resolved, err := filepath.EvalSymlinks(topPath); err == nil {
		topPath = filepath.Clean(resolved)
	}
	repoPath := filepath.Clean(repo)
	if resolved, err := filepath.EvalSymlinks(repoPath); err == nil {
		repoPath = filepath.Clean(resolved)
	}
	if topPath != repoPath {
		return refuse("repo_path must be the Git worktree root")
	}
	return nil
}

func (p Policy) requireNoExecutionConfig(repo string) error {
	result, err := p.Git.RunAllowFailure(repo, "config", "--get-regexp", executionConfigRE.String())
	if err != nil {
		return err
	}
	if result.ExitCode == 1 {
		return nil
	}
	if result.ExitCode != 0 {
		return refuse("git config inspection failed: " + strings.TrimSpace(result.Stderr+result.Stdout))
	}
	keys := map[string]struct{}{}
	for _, line := range strings.Split(result.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			keys[fields[0]] = struct{}{}
		}
	}
	if len(keys) == 0 {
		return nil
	}
	list := make([]string, 0, len(keys))
	for key := range keys {
		list = append(list, key)
	}
	sort.Strings(list)
	return refuse("repository has execution-capable Git config: " + strings.Join(list, ", "))
}

func (p Policy) repoState(repo string) (repoState, error) {
	var ambiguous []string
	branchResult, err := p.Git.RunAllowFailure(repo, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return repoState{}, err
	}
	var branch *string
	if branchResult.ExitCode == 0 {
		value := strings.TrimSpace(branchResult.Stdout)
		branch = &value
	} else {
		ambiguous = append(ambiguous, "detached HEAD")
	}
	gitDir, err := p.gitDir(repo)
	if err != nil {
		return repoState{}, err
	}
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG"} {
		if _, err := os.Stat(filepath.Join(gitDir, name)); err == nil {
			ambiguous = append(ambiguous, name)
		}
	}
	for _, name := range []string{"rebase-apply", "rebase-merge"} {
		if info, err := os.Stat(filepath.Join(gitDir, name)); err == nil && info.IsDir() {
			ambiguous = append(ambiguous, name)
		}
	}
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return repoState{}, err
	}
	for _, entry := range entries {
		if _, ok := conflictCodes[entry.Code]; ok || strings.Contains(entry.Code, "U") {
			ambiguous = append(ambiguous, "unresolved conflicts")
			break
		}
	}
	if ambiguous == nil {
		ambiguous = []string{}
	}
	staged, err := p.stagedFiles(repo)
	if err != nil {
		return repoState{}, err
	}
	return repoState{Branch: branch, AmbiguousReasons: ambiguous, HasStagedChanges: len(staged) > 0}, nil
}

func (p Policy) status(repo string) (StatusResult, error) {
	state, err := p.repoState(repo)
	if err != nil {
		return StatusResult{}, err
	}
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return StatusResult{}, err
	}
	visible := make([]StatusEntry, 0, len(entries))
	redacted := 0
	for _, entry := range entries {
		if secretcheck.IsSecretPath(entry.Path) {
			redacted++
			continue
		}
		visible = append(visible, entry)
	}
	return StatusResult{
		Result:                  "ok",
		Repo:                    repo,
		Branch:                  state.Branch,
		IsDetached:              state.Branch == nil,
		AmbiguousReasons:        state.AmbiguousReasons,
		HasStagedChanges:        state.HasStagedChanges,
		Clean:                   len(visible) == 0 && redacted == 0,
		Entries:                 visible,
		RedactedSecretPathCount: redacted,
	}, nil
}

func (p Policy) requireClearCommitState(repo string) (repoState, error) {
	state, err := p.repoState(repo)
	if err != nil {
		return repoState{}, err
	}
	if len(state.AmbiguousReasons) > 0 {
		return repoState{}, refuse("repository has ambiguous state: " + strings.Join(state.AmbiguousReasons, ", "))
	}
	if state.HasStagedChanges {
		return repoState{}, refuse("repository already has staged changes")
	}
	return state, nil
}

func (p Policy) requireCleanWorktreeState(repo string) (repoState, error) {
	state, err := p.requireClearCommitState(repo)
	if err != nil {
		return repoState{}, err
	}
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return repoState{}, err
	}
	if len(entries) > 0 {
		return repoState{}, refuse("repository must be clean before merging")
	}
	return state, nil
}

func requireBranchPrepState(state repoState) error {
	var blocking []string
	for _, reason := range state.AmbiguousReasons {
		if reason != "detached HEAD" {
			blocking = append(blocking, reason)
		}
	}
	if len(blocking) > 0 {
		return refuse("repository has ambiguous state: " + strings.Join(blocking, ", "))
	}
	if state.HasStagedChanges {
		return refuse("repository already has staged changes")
	}
	return nil
}

func (p Policy) requireCommitBranchAllowed(repo string, state repoState) error {
	if state.Branch == nil {
		return nil
	}
	protected, err := p.protectedBranches(repo)
	if err != nil {
		return err
	}
	if _, ok := protected[*state.Branch]; ok {
		return refuse("refusing commit on protected branch: " + *state.Branch)
	}
	return nil
}

func (p Policy) requireBranchNotProtected(repo, branch, label string) error {
	protected, err := p.protectedBranches(repo)
	if err != nil {
		return err
	}
	if _, ok := protected[branch]; ok {
		return refuse("refusing protected " + label + ": " + branch)
	}
	return nil
}

func (p Policy) protectedBranches(repo string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	for branch := range protectedBranches {
		result[branch] = struct{}{}
	}
	configured, err := p.Git.RunAllowFailure(repo, "config", "--get", "init.defaultBranch")
	if err != nil {
		return nil, err
	}
	if configured.ExitCode == 0 {
		branch := strings.TrimSpace(configured.Stdout)
		if branch != "" {
			result[branch] = struct{}{}
		}
	} else if configured.ExitCode != 1 {
		return nil, refuse("default branch inspection failed: " + strings.TrimSpace(configured.Stderr+configured.Stdout))
	}
	return result, nil
}

func (p Policy) normaliseFiles(repo string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, refuse("at least one file must be listed")
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(files))
	for _, item := range files {
		if strings.TrimSpace(item) == "" {
			return nil, refuse("requested files must be non-empty strings")
		}
		path := item
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if _, err := os.Lstat(abs); err == nil {
			if resolved, err := filepath.EvalSymlinks(abs); err == nil {
				abs = resolved
			}
		}
		rel, err := filepath.Rel(repo, filepath.Clean(abs))
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return nil, refuse("requested file is outside repo: " + item)
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			return nil, refuse("duplicate requested file: " + rel)
		}
		seen[rel] = struct{}{}
		if secretcheck.IsSecretPath(rel) {
			return nil, refuse("refusing secret-bearing path: " + rel)
		}
		filePath := filepath.Join(repo, filepath.FromSlash(rel))
		info, lstatErr := os.Lstat(filePath)
		if lstatErr == nil {
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return nil, refuse("requested path must be a regular file: " + rel)
			}
		} else if errors.Is(lstatErr, os.ErrNotExist) {
			tracked, err := p.tracked(repo, rel)
			if err != nil {
				return nil, err
			}
			if !tracked {
				return nil, refuse("requested file does not exist and is not tracked: " + rel)
			}
		} else {
			return nil, lstatErr
		}
		result = append(result, rel)
	}
	return result, nil
}

func rejectAttribution(message string, body *string) error {
	if strings.TrimSpace(message) == "" {
		return refuse("commit message subject is empty")
	}
	text := message
	if body != nil && *body != "" {
		text += "\n" + *body
	}
	if strings.ContainsRune(text, '\x00') {
		return refuse("commit message contains NUL")
	}
	if aiAttributionPattern.MatchString(text) {
		return refuse("commit message contains AI/tool attribution")
	}
	return nil
}

func (p Policy) normaliseBranchName(branchName string) (string, error) {
	if strings.TrimSpace(branchName) == "" {
		return "", refuse("branch_name must be a non-empty string")
	}
	branch := strings.TrimSpace(branchName)
	if branch != branchName || strings.ContainsRune(branch, '\x00') {
		return "", refuse("branch_name contains invalid characters")
	}
	if strings.HasPrefix(branch, "-") || strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return "", refuse("branch_name has unsafe syntax")
	}
	prefix := strings.SplitN(branch, "/", 2)[0]
	if strings.HasPrefix(branch, "refs/") {
		return "", refuse("branch_name must be a local branch, not a remote/ref path")
	}
	if _, ok := remoteLikePrefixes[prefix]; ok {
		return "", refuse("branch_name must be a local branch, not a remote/ref path")
	}
	if hexBranchPattern.MatchString(branch) {
		return "", refuse("branch_name must be a local branch, not a commit hash")
	}
	for _, token := range []string{"..", "//", "@{", "\\", ":", "?", "[", "*", "~", "^", " "} {
		if strings.Contains(branch, token) {
			return "", refuse("branch_name has unsafe syntax")
		}
	}
	if strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") {
		return "", refuse("branch_name has unsafe syntax")
	}
	for _, part := range strings.Split(branch, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return "", refuse("branch_name has unsafe syntax")
		}
	}
	check, err := p.Git.RunAllowFailure(".", "check-ref-format", "--branch", branch)
	if err != nil {
		return "", err
	}
	if check.ExitCode != 0 {
		return "", refuse("branch_name is not a valid local branch name")
	}
	return branch, nil
}

func (p Policy) safeUntracked(repo string) (UntrackedSummary, error) {
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return UntrackedSummary{}, err
	}
	files := make([]string, 0)
	redacted := 0
	for _, entry := range entries {
		if entry.Code != "??" {
			continue
		}
		if secretcheck.IsSecretPath(entry.Path) {
			redacted++
		} else {
			files = append(files, entry.Path)
		}
	}
	sort.Strings(files)
	return UntrackedSummary{Files: files, RedactedSecretPathCount: redacted}, nil
}

func (p Policy) numstat(repo string, args []string) (FileSummary, error) {
	raw, err := p.Git.Run(repo, args...)
	if err != nil {
		return FileSummary{}, err
	}
	records := strings.Split(raw.Stdout, "\x00")
	files := make([]FileStat, 0)
	redacted := 0
	for _, record := range records {
		if record == "" {
			continue
		}
		fields := strings.Split(record, "\t")
		if len(fields) < 3 {
			continue
		}
		rel := fields[len(fields)-1]
		if secretcheck.IsSecretPath(rel) {
			redacted++
			continue
		}
		files = append(files, FileStat{
			Path:      rel,
			Additions: parseNumstatInt(fields[0]),
			Deletions: parseNumstatInt(fields[1]),
		})
	}
	return FileSummary{Files: files, RedactedSecretPathCount: redacted}, nil
}

func parseNumstatInt(value string) *int {
	if value == "-" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}

func (p Policy) porcelainEntries(repo string) ([]StatusEntry, error) {
	raw, err := p.Git.Run(repo, "status", "--porcelain=v1", "-z")
	if err != nil {
		return nil, err
	}
	parts := splitNUL(raw.Stdout)
	entries := make([]StatusEntry, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		item := parts[index]
		code := ""
		if len(item) >= 2 {
			code = item[:2]
		}
		path := ""
		if len(item) > 3 {
			path = item[3:]
		}
		if strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C") {
			index++
			if index < len(parts) {
				path = parts[index]
			}
		}
		entries = append(entries, StatusEntry{Code: code, Path: path})
	}
	return entries, nil
}

func (p Policy) stagedFiles(repo string) ([]string, error) {
	raw, err := p.Git.Run(repo, "diff", "--cached", "--no-renames", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	return splitNUL(raw.Stdout), nil
}

func splitNUL(raw string) []string {
	parts := strings.Split(raw, "\x00")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func (p Policy) isIndexEmpty(repo string) bool {
	result, err := p.Git.RunAllowFailure(repo, "diff", "--cached", "--quiet", "--exit-code")
	return err == nil && result.ExitCode == 0
}

func (p Policy) tracked(repo, rel string) (bool, error) {
	result, err := p.Git.RunAllowFailure(repo, "ls-files", "--error-unmatch", "--", rel)
	if err != nil {
		return false, err
	}
	return result.ExitCode == 0, nil
}

func (p Policy) currentHead(repo string) (string, error) {
	result, err := p.Git.Run(repo, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (p Policy) localBranchHead(repo, branch string) (string, error) {
	result, err := p.Git.RunAllowFailure(repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return "", err
	}
	if result.ExitCode == 0 {
		return strings.TrimSpace(result.Stdout), nil
	}
	if result.ExitCode == 1 {
		return "", nil
	}
	return "", refuse("git branch inspection failed: " + strings.TrimSpace(result.Stderr+result.Stdout))
}

func (p Policy) requireLocalBranch(repo, branch, label string) (string, error) {
	head, err := p.localBranchHead(repo, branch)
	if err != nil {
		return "", err
	}
	if head == "" {
		return "", refuse(label + " does not exist as a local branch: " + branch)
	}
	return head, nil
}

func (p Policy) gitDir(repo string) (string, error) {
	result, err := p.Git.Run(repo, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(result.Stdout)
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}
	return filepath.Clean(filepath.Join(repo, raw)), nil
}

func (p Policy) unstageBestEffort(repo string, rels []string) {
	if len(rels) == 0 {
		return
	}
	args := append([]string{"reset", "-q", "HEAD", "--"}, rels...)
	_, _ = p.Git.RunAllowFailure(repo, args...)
}

func (p Policy) rejectLikelySecretMaterial(repo string, rels []string) error {
	for _, rel := range rels {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		tracked, err := p.tracked(repo, rel)
		if err != nil {
			return err
		}
		var additions []string
		if _, statErr := os.Stat(path); statErr == nil && !tracked {
			lines, err := readLines(path)
			if err != nil {
				return err
			}
			additions = lines
		} else {
			diff, err := p.Git.Run(repo, "diff", "--no-ext-diff", "--unified=0", "--", rel)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(diff.Stdout, "\n") {
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					additions = append(additions, strings.TrimPrefix(line, "+"))
				}
			}
		}
		for _, line := range additions {
			if secretcheck.ContainsLikelySecret(line) {
				return refuse("requested diff appears to contain secret material: " + rel)
			}
		}
	}
	return nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func sameStringSet(left, right []string) bool {
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	if len(leftCopy) != len(rightCopy) {
		return false
	}
	for i := range leftCopy {
		if leftCopy[i] != rightCopy[i] {
			return false
		}
	}
	return true
}

func branchOrInput(branch, input string) string {
	if branch != "" {
		return branch
	}
	return input
}

func optionalBranch(branch *string) string {
	if branch == nil {
		return ""
	}
	return *branch
}

func (p Policy) requireCleanRepoForTests(repo string) error {
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("repo is not clean: %#v", entries)
	}
	return nil
}
