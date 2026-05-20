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

	"local/codex-safe-git/internal/audit"
	"local/codex-safe-git/internal/secretcheck"
)

type repoState struct {
	Branch           *string
	AmbiguousReasons []string
	HasStagedChanges bool
}

type rawWorktree struct {
	Path       string
	Head       string
	Branch     *string
	IsBare     bool
	IsLocked   bool
	IsPrunable bool
}

func refuse(reason string) Refusal {
	return Refusal{Reason: reason}
}

func (p Policy) auditRefusal(action, repo string, err error) {
	p.Audit.WriteAuditCompat(action, repo, err)
}

func (p Policy) requireAuditWritable() error {
	if err := p.Audit.EnsureWritable(); err != nil {
		return refuse("audit log is not writable: " + err.Error())
	}
	return nil
}

func (p Policy) auditSuccess(entry audit.Entry) error {
	if err := p.Audit.WriteChecked(entry); err != nil {
		return refuse("audit log is not writable: " + err.Error())
	}
	return nil
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

func (p Policy) isAllowedPath(path string) bool {
	cleaned := canonicalCleanPath(path)
	if _, ok := p.Config.AllowedRepos[cleaned]; ok {
		return true
	}
	for root := range p.Config.AllowedRepoRoots {
		if pathWithin(cleaned, root) {
			return true
		}
	}
	return false
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
		if entryHasSecretPath(entry) {
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
		return repoState{}, refuse("repository must be clean before this operation")
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
	for branch := range p.Config.ProtectedBranches {
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
	repo = filepath.Clean(repo)
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
		filePath := filepath.Clean(abs)
		rel, err := filepath.Rel(repo, filePath)
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
		info, lstatErr := os.Lstat(filePath)
		if lstatErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, refuse("requested path must not be a symlink: " + rel)
			}
			if !info.Mode().IsRegular() {
				return nil, refuse("requested path must be a regular file: " + rel)
			}
			hasSymlink, err := pathContainsSymlink(repo, rel)
			if err != nil {
				return nil, err
			}
			if hasSymlink {
				return nil, refuse("requested path must not contain symlink components: " + rel)
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
		if entryHasSecretPath(entry) {
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
	records := splitNUL(raw.Stdout)
	files := make([]FileStat, 0)
	redacted := 0
	for index := 0; index < len(records); index++ {
		record := records[index]
		fields := strings.SplitN(record, "\t", 3)
		if len(fields) < 3 {
			return FileSummary{}, fmt.Errorf("unexpected git numstat format")
		}
		rel := fields[2]
		paths := []string{rel}
		if rel == "" {
			if index+2 >= len(records) {
				return FileSummary{}, fmt.Errorf("unexpected git rename numstat format")
			}
			oldPath := records[index+1]
			newPath := records[index+2]
			paths = []string{newPath, oldPath}
			rel = newPath
			index += 2
		}
		if anySecretPath(paths...) {
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
		originalPath := ""
		if len(item) > 3 {
			path = item[3:]
		}
		if strings.HasPrefix(code, "R") || strings.HasPrefix(code, "C") {
			index++
			if index < len(parts) {
				originalPath = parts[index]
			}
		}
		entries = append(entries, StatusEntry{Code: code, Path: path, OriginalPath: originalPath})
	}
	return entries, nil
}

func entryHasSecretPath(entry StatusEntry) bool {
	return anySecretPath(entry.Path, entry.OriginalPath)
}

func anySecretPath(paths ...string) bool {
	for _, path := range paths {
		if path != "" && secretcheck.IsSecretPath(path) {
			return true
		}
	}
	return false
}

func normaliseNewWorktreePath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", refuse("worktree_path must be a non-empty string")
	}
	if strings.TrimSpace(raw) != raw || strings.ContainsRune(raw, '\x00') {
		return "", refuse("worktree_path contains invalid characters")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	target := filepath.Clean(abs)
	if _, err := os.Lstat(target); err == nil {
		return "", refuse("worktree_path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent := filepath.Dir(target)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return "", refuse("worktree_path parent does not exist or is not a directory")
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	target = filepath.Clean(filepath.Join(resolvedParent, filepath.Base(target)))
	if hasSecretPathComponent(target) {
		return "", refuse("worktree_path must not be inside a secret-bearing path")
	}
	return target, nil
}

func hasSecretPathComponent(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(clean, "/")
	for index := range parts {
		if parts[index] == "" {
			continue
		}
		if secretcheck.IsSecretPath(strings.Join(parts[index:], "/")) {
			return true
		}
	}
	return false
}

func pathContainsSymlink(repo, rel string) (bool, error) {
	current := repo
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
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

func (p Policy) requireBranchNotCheckedOutElsewhere(repo, branch string) error {
	worktrees, err := p.worktrees(repo)
	if err != nil {
		return err
	}
	for _, worktree := range worktrees {
		if worktree.Branch != nil && *worktree.Branch == branch && !samePath(worktree.Path, repo) {
			return refuse("branch is already checked out in another worktree: " + branch)
		}
	}
	return nil
}

func (p Policy) requireWorktreePathAvailable(repo, target string) error {
	worktrees, err := p.worktrees(repo)
	if err != nil {
		return err
	}
	for _, worktree := range worktrees {
		if samePath(worktree.Path, target) || pathWithin(target, worktree.Path) || pathWithin(worktree.Path, target) {
			return refuse("worktree_path overlaps an existing worktree")
		}
	}
	return nil
}

func (p Policy) worktrees(repo string) ([]rawWorktree, error) {
	raw, err := p.Git.Run(repo, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	records := splitNULKeepEmpty(raw.Stdout)
	worktrees := make([]rawWorktree, 0)
	var current *rawWorktree
	flush := func() {
		if current != nil && current.Path != "" {
			current.Path = canonicalCleanPath(current.Path)
			worktrees = append(worktrees, *current)
		}
		current = nil
	}
	for _, record := range records {
		if record == "" {
			flush()
			continue
		}
		key, value, hasValue := strings.Cut(record, " ")
		if key == "worktree" {
			flush()
			current = &rawWorktree{Path: filepath.Clean(value)}
			continue
		}
		if current == nil {
			continue
		}
		switch key {
		case "HEAD":
			current.Head = value
		case "branch":
			if hasValue && strings.HasPrefix(value, "refs/heads/") {
				branch := strings.TrimPrefix(value, "refs/heads/")
				current.Branch = &branch
			}
		case "bare":
			current.IsBare = true
		case "locked":
			current.IsLocked = true
		case "prunable":
			current.IsPrunable = true
		}
	}
	flush()
	return worktrees, nil
}

func splitNULKeepEmpty(raw string) []string {
	return strings.Split(raw, "\x00")
}

func samePath(left, right string) bool {
	return canonicalCleanPath(left) == canonicalCleanPath(right)
}

func canonicalCleanPath(path string) string {
	cleaned := filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.Clean(resolved)
	}
	return cleaned
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

func (p Policy) rejectLikelySecretMaterialInStagedDiff(repo string, rels []string) error {
	for _, rel := range rels {
		diff, err := p.Git.Run(repo, "diff", "--cached", "--no-ext-diff", "--unified=0", "--", rel)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(diff.Stdout, "\n") {
			if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
				continue
			}
			if secretcheck.ContainsLikelySecret(strings.TrimPrefix(line, "+")) {
				return refuse("requested staged diff appears to contain secret material: " + rel)
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
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
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
