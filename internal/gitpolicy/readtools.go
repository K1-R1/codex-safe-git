package gitpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"local/codex-safe-git/internal/secretcheck"
)

var fullObjectIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$|^[0-9a-fA-F]{64}$`)
var hexObjectIDLikePattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

type resolvedCommit struct {
	Input  string
	Commit string
}

type inspectionPath struct {
	Input    string
	Rel      string
	Abs      string
	Redacted bool
}

func (p Policy) ListLocalBranches(repoPath string) (ListLocalBranchesResult, error) {
	repo, err := p.readOnlyRepo("list_local_branches", repoPath)
	if err != nil {
		return ListLocalBranchesResult{}, err
	}
	branches, err := p.localBranches(repo)
	if err != nil {
		return ListLocalBranchesResult{}, err
	}
	protected, err := p.protectedBranches(repo)
	if err != nil {
		return ListLocalBranchesResult{}, err
	}
	state, err := p.repoState(repo)
	if err != nil {
		return ListLocalBranchesResult{}, err
	}
	checkedOut := p.checkedOutBranches(repo)
	for index := range branches {
		if state.Branch != nil && branches[index].Name == *state.Branch {
			branches[index].IsCurrent = true
		}
		_, branches[index].IsProtected = protected[branches[index].Name]
		_, branches[index].IsCheckedOut = checkedOut[branches[index].Name]
	}
	sort.SliceStable(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
	count := len(branches)
	branches, truncated := limitedSlice(branches, BranchLimit)
	return ListLocalBranchesResult{Result: "ok", Repo: repo, Branches: branches, BranchCount: count, BranchesTruncated: truncated, BranchLimit: BranchLimit}, nil
}

func (p Policy) ListLocalRefs(repoPath string) (ListLocalRefsResult, error) {
	repo, err := p.readOnlyRepo("list_local_refs", repoPath)
	if err != nil {
		return ListLocalRefsResult{}, err
	}
	refs, err := p.localRefs(repo)
	if err != nil {
		return ListLocalRefsResult{}, err
	}
	count := len(refs)
	refs, truncated := limitedSlice(refs, RefLimit)
	return ListLocalRefsResult{Result: "ok", Repo: repo, Refs: refs, RefCount: count, RefsTruncated: truncated, RefLimit: RefLimit}, nil
}

func (p Policy) MergeBase(repoPath, leftRef, rightRef string) (MergeBaseResult, error) {
	repo, err := p.readOnlyRepo("merge_base", repoPath)
	if err != nil {
		return MergeBaseResult{}, err
	}
	left, right, base, err := p.resolveRefPair(repo, leftRef, rightRef)
	if err != nil {
		return MergeBaseResult{}, err
	}
	isAncestor, err := p.isAncestor(repo, left.Commit, right.Commit)
	if err != nil {
		return MergeBaseResult{}, err
	}
	return MergeBaseResult{Result: "ok", Repo: repo, LeftRef: left.Input, RightRef: right.Input, LeftCommit: left.Commit, RightCommit: right.Commit, MergeBase: base, IsAncestor: isAncestor}, nil
}

func (p Policy) CompareRefs(repoPath, baseRef, targetRef string) (CompareRefsResult, error) {
	repo, err := p.readOnlyRepo("compare_refs", repoPath)
	if err != nil {
		return CompareRefsResult{}, err
	}
	base, target, mergeBase, err := p.resolveRefPair(repo, baseRef, targetRef)
	if err != nil {
		return CompareRefsResult{}, err
	}
	ahead, err := p.revCount(repo, base.Commit+".."+target.Commit)
	if err != nil {
		return CompareRefsResult{}, err
	}
	behind, err := p.revCount(repo, target.Commit+".."+base.Commit)
	if err != nil {
		return CompareRefsResult{}, err
	}
	changed, err := p.changedPathSummary(repo, base.Commit, target.Commit)
	if err != nil {
		return CompareRefsResult{}, err
	}
	return CompareRefsResult{
		Result:                  "ok",
		Repo:                    repo,
		BaseRef:                 base.Input,
		TargetRef:               target.Input,
		BaseCommit:              base.Commit,
		TargetCommit:            target.Commit,
		MergeBase:               mergeBase,
		AheadCount:              ahead,
		BehindCount:             behind,
		ChangedFileCount:        changed.FileCount,
		RedactedSecretPathCount: changed.RedactedSecretPathCount,
	}, nil
}

func (p Policy) ChangedFilesBetweenRefs(repoPath, baseRef, targetRef string) (ChangedFilesBetweenRefsResult, error) {
	repo, err := p.readOnlyRepo("changed_files_between_refs", repoPath)
	if err != nil {
		return ChangedFilesBetweenRefsResult{}, err
	}
	base, target, _, err := p.resolveRefPair(repo, baseRef, targetRef)
	if err != nil {
		return ChangedFilesBetweenRefsResult{}, err
	}
	changed, err := p.changedPathSummary(repo, base.Commit, target.Commit)
	if err != nil {
		return ChangedFilesBetweenRefsResult{}, err
	}
	return ChangedFilesBetweenRefsResult{Result: "ok", Repo: repo, BaseRef: base.Input, TargetRef: target.Input, BaseCommit: base.Commit, TargetCommit: target.Commit, Changed: changed}, nil
}

func (p Policy) CommitLogSummary(repoPath string, ref *string, limit *int) (CommitLogSummaryResult, error) {
	repo, err := p.readOnlyRepo("commit_log_summary", repoPath)
	if err != nil {
		return CommitLogSummaryResult{}, err
	}
	refName := "HEAD"
	if ref != nil {
		refName = *ref
	}
	resolved, err := p.resolveCommit(repo, refName, "ref")
	if err != nil {
		return CommitLogSummaryResult{}, err
	}
	requestedLimit, err := boundedLimit(limit, LogCommitLimit, "limit")
	if err != nil {
		return CommitLogSummaryResult{}, err
	}
	commits, truncated, err := p.commitSummaries(repo, resolved.Commit, requestedLimit)
	if err != nil {
		return CommitLogSummaryResult{}, err
	}
	return CommitLogSummaryResult{Result: "ok", Repo: repo, Ref: resolved.Input, ResolvedCommit: resolved.Commit, Commits: commits, CommitCount: len(commits), CommitsTruncated: truncated, CommitLimit: requestedLimit}, nil
}

func (p Policy) ShowCommitSummary(repoPath, commitRef string) (ShowCommitSummaryResult, error) {
	repo, err := p.readOnlyRepo("show_commit_summary", repoPath)
	if err != nil {
		return ShowCommitSummaryResult{}, err
	}
	resolved, err := p.resolveCommit(repo, commitRef, "commit_ref")
	if err != nil {
		return ShowCommitSummaryResult{}, err
	}
	commit, err := p.commitSummary(repo, resolved.Commit)
	if err != nil {
		return ShowCommitSummaryResult{}, err
	}
	return ShowCommitSummaryResult{Result: "ok", Repo: repo, CommitRef: resolved.Input, Commit: commit}, nil
}

func (p Policy) PathStatus(repoPath string, paths []string, includeIgnoreSource *bool) (PathStatusResult, error) {
	repo, err := p.readOnlyRepo("path_status", repoPath)
	if err != nil {
		return PathStatusResult{}, err
	}
	items, redacted, err := p.normaliseInspectionPaths(repo, paths)
	if err != nil {
		return PathStatusResult{}, err
	}
	includeSource := includeIgnoreSource != nil && *includeIgnoreSource
	statusByPath, err := p.statusByPath(repo)
	if err != nil {
		return PathStatusResult{}, err
	}
	lsTags, err := p.lsFileTags(repo, relsFromInspection(items))
	if err != nil {
		return PathStatusResult{}, err
	}
	entries := make([]PathStatusEntry, 0, len(items))
	for _, item := range items {
		if item.Redacted {
			continue
		}
		entry, err := p.onePathStatus(repo, item, statusByPath[item.Rel], lsTags[item.Rel], includeSource)
		if err != nil {
			return PathStatusResult{}, err
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	count := len(entries)
	entries, truncated := limitedSlice(entries, PathStatusLimit)
	return PathStatusResult{Result: "ok", Repo: repo, Entries: entries, EntryCount: count, EntriesTruncated: truncated, EntryLimit: PathStatusLimit, RedactedSecretPathCount: redacted}, nil
}

func (p Policy) SubmoduleSummary(repoPath string) (SubmoduleSummaryResult, error) {
	repo, err := p.readOnlyRepo("submodule_summary", repoPath)
	if err != nil {
		return SubmoduleSummaryResult{}, err
	}
	status, err := p.Git.Run(repo, "submodule", "status", "--recursive")
	if err != nil {
		return SubmoduleSummaryResult{}, err
	}
	dirtyPaths, err := p.submoduleDirtyPaths(repo)
	if err != nil {
		return SubmoduleSummaryResult{}, err
	}
	submodules := make([]SubmoduleEntry, 0)
	redacted := 0
	for _, line := range strings.Split(status.Stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry, ok := parseSubmoduleStatusLine(line)
		if !ok {
			continue
		}
		if secretcheck.IsSecretPath(entry.Path) {
			redacted++
			continue
		}
		if dirtyPaths[entry.Path] {
			entry.Dirty = true
		}
		submodules = append(submodules, entry)
	}
	sort.SliceStable(submodules, func(i, j int) bool { return submodules[i].Path < submodules[j].Path })
	count := len(submodules)
	submodules, truncated := limitedSlice(submodules, SubmoduleLimit)
	return SubmoduleSummaryResult{Result: "ok", Repo: repo, Submodules: submodules, SubmoduleCount: count, SubmodulesTruncated: truncated, SubmoduleLimit: SubmoduleLimit, RedactedSecretPathCount: redacted}, nil
}

func (p Policy) RepositoryIntegrityCheck(repoPath string) (RepositoryIntegrityCheckResult, error) {
	repo, err := p.readOnlyRepo("repository_integrity_check", repoPath)
	if err != nil {
		return RepositoryIntegrityCheckResult{}, err
	}
	result, err := p.Git.RunAllowFailure(repo, "fsck", "--connectivity-only", "--no-dangling", "--no-progress")
	if err != nil {
		return RepositoryIntegrityCheckResult{}, err
	}
	issues := parseIntegrityIssues(result.Stdout + "\n" + result.Stderr)
	issueCount := totalIntegrityIssueCount(issues)
	issues, truncated := limitedSlice(issues, IntegrityIssueLimit)
	return RepositoryIntegrityCheckResult{Result: "ok", Repo: repo, IntegrityOK: result.ExitCode == 0 && issueCount == 0, ExitCode: result.ExitCode, Issues: issues, IssueCount: issueCount, IssuesTruncated: truncated, IssueLimit: IntegrityIssueLimit}, nil
}

func totalIntegrityIssueCount(issues []IntegrityIssueCount) int {
	total := 0
	for _, issue := range issues {
		total += issue.Count
	}
	return total
}

func (p Policy) ReflogSummary(repoPath string, ref *string, limit *int) (ReflogSummaryResult, error) {
	repo, err := p.readOnlyRepo("reflog_summary", repoPath)
	if err != nil {
		return ReflogSummaryResult{}, err
	}
	refName := "HEAD"
	if ref != nil {
		refName = *ref
	}
	normalisedRef, err := p.normaliseReflogRef(repo, refName)
	if err != nil {
		return ReflogSummaryResult{}, err
	}
	requestedLimit, err := boundedLimit(limit, ReflogLimit, "limit")
	if err != nil {
		return ReflogSummaryResult{}, err
	}
	entries, redacted, truncated, err := p.reflogEntries(repo, normalisedRef, requestedLimit)
	if err != nil {
		return ReflogSummaryResult{}, err
	}
	return ReflogSummaryResult{Result: "ok", Repo: repo, Ref: normalisedRef, Entries: entries, EntryCount: len(entries), EntriesTruncated: truncated, EntryLimit: requestedLimit, RedactedSensitiveSummaryCount: redacted}, nil
}

func (p Policy) SelfCheck(repoPath, serverVersion, protocolVersion string, toolNames []string) (SelfCheckResult, error) {
	repo, err := p.readOnlyRepo("self_check", repoPath)
	if err != nil {
		return SelfCheckResult{}, err
	}
	gitVersion := ""
	if result, err := p.Git.Run(repo, "version"); err == nil {
		gitVersion = strings.TrimSpace(result.Stdout)
	}
	binaryHash, checksum, checksumStatus := binaryChecksumStatus()
	allowlist := allowlistFingerprints(p.Config.AllowedRepos, p.Config.AllowedRepoRoots)
	auditWritable := auditLogLikelyWritable(p.Config.AuditLog)
	names := append([]string(nil), toolNames...)
	sort.Strings(names)
	payload := SelfCheckResult{
		Result:                  "ok",
		Repo:                    repo,
		ServerVersion:           serverVersion,
		ProtocolVersion:         protocolVersion,
		GitVersion:              gitVersion,
		ToolNames:               names,
		ToolCount:               len(names),
		BinaryPathHash:          binaryHash,
		BinaryChecksum:          checksum,
		ChecksumStatus:          checksumStatus,
		AuditLogConfigured:      p.Config.AuditLog != "",
		AuditLogWritable:        auditWritable,
		AllowedRepoCount:        len(p.Config.AllowedRepos),
		AllowedRepoRootCount:    len(p.Config.AllowedRepoRoots),
		AllowlistFingerprints:   allowlist,
		ProtectedBranchCount:    len(p.Config.ProtectedBranches),
		RedactedConfigPathCount: len(p.Config.AllowedRepos) + len(p.Config.AllowedRepoRoots),
	}
	return payload, nil
}

func (p Policy) readOnlyRepo(_ string, repoPath string) (string, error) {
	repo, err := p.resolveAllowedRepo(repoPath)
	if err != nil {
		return "", err
	}
	if err := p.requireGitRepo(repo); err != nil {
		return "", err
	}
	if err := p.requireNoExecutionConfig(repo); err != nil {
		return "", err
	}
	return repo, nil
}

func (p Policy) localBranches(repo string) ([]LocalBranchEntry, error) {
	result, err := p.Git.Run(repo, "for-each-ref", "--format=%(refname:short)%00%(objectname)%00", "refs/heads")
	if err != nil {
		return nil, err
	}
	parts := splitNUL(result.Stdout)
	branches := make([]LocalBranchEntry, 0, len(parts)/2)
	for index := 0; index+1 < len(parts); index += 2 {
		branches = append(branches, LocalBranchEntry{Name: cleanGitField(parts[index]), Head: cleanGitField(parts[index+1])})
	}
	return branches, nil
}

func (p Policy) localRefs(repo string) ([]LocalRefEntry, error) {
	result, err := p.Git.Run(repo, "for-each-ref", "--format=%(refname)%00%(refname:short)%00%(objectname)%00%(objecttype)%00", "refs/heads", "refs/tags")
	if err != nil {
		return nil, err
	}
	protected, err := p.protectedBranches(repo)
	if err != nil {
		return nil, err
	}
	parts := splitNUL(result.Stdout)
	refs := make([]LocalRefEntry, 0, len(parts)/4)
	for index := 0; index+3 < len(parts); index += 4 {
		ref := cleanGitField(parts[index])
		short := cleanGitField(parts[index+1])
		kind := localRefKind(ref)
		_, isProtected := protected[short]
		refs = append(refs, LocalRefEntry{Ref: ref, ShortName: short, Kind: kind, TargetHash: cleanGitField(parts[index+2]), TargetType: cleanGitField(parts[index+3]), IsProtected: kind == "branch" && isProtected, IsVisible: true})
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Ref < refs[j].Ref })
	return refs, nil
}

func cleanGitField(value string) string {
	return strings.Trim(value, "\n")
}

func localRefKind(ref string) string {
	switch {
	case strings.HasPrefix(ref, "refs/heads/"):
		return "branch"
	case strings.HasPrefix(ref, "refs/tags/"):
		return "tag"
	default:
		return "other"
	}
}

func (p Policy) checkedOutBranches(repo string) map[string]struct{} {
	result := map[string]struct{}{}
	worktrees, err := p.worktrees(repo)
	if err != nil {
		return result
	}
	for _, worktree := range worktrees {
		if worktree.Branch != nil {
			result[*worktree.Branch] = struct{}{}
		}
	}
	return result
}

func (p Policy) resolveRefPair(repo, leftRef, rightRef string) (resolvedCommit, resolvedCommit, string, error) {
	left, err := p.resolveCommit(repo, leftRef, "left_ref")
	if err != nil {
		return resolvedCommit{}, resolvedCommit{}, "", err
	}
	right, err := p.resolveCommit(repo, rightRef, "right_ref")
	if err != nil {
		return resolvedCommit{}, resolvedCommit{}, "", err
	}
	base, err := p.mergeBase(repo, left.Commit, right.Commit)
	if err != nil {
		return resolvedCommit{}, resolvedCommit{}, "", err
	}
	return left, right, base, nil
}

func (p Policy) resolveCommit(repo, input, label string) (resolvedCommit, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return resolvedCommit{}, refuse(label + " must be a non-empty string")
	}
	if value != input || strings.ContainsRune(value, '\x00') || strings.HasPrefix(value, "-") {
		return resolvedCommit{}, refuse(label + " has unsafe syntax")
	}
	objectIDLength, err := p.objectIDLength(repo)
	if err != nil {
		return resolvedCommit{}, err
	}
	refArg := ""
	switch {
	case value == "HEAD":
		refArg = "HEAD^{commit}"
	case fullObjectIDPattern.MatchString(value):
		if len(value) != objectIDLength {
			return resolvedCommit{}, refuse(label + " must be a full object id for this repository format")
		}
		refArg = value + "^{commit}"
	case hexObjectIDLikePattern.MatchString(value):
		return resolvedCommit{}, refuse(label + " must be a full object id for this repository format")
	case strings.HasPrefix(value, "refs/heads/"):
		branch := strings.TrimPrefix(value, "refs/heads/")
		normalised, err := p.normaliseBranchName(branch)
		if err != nil {
			return resolvedCommit{}, err
		}
		refArg = "refs/heads/" + normalised + "^{commit}"
		value = normalised
	default:
		branch, err := p.normaliseBranchName(value)
		if err != nil {
			return resolvedCommit{}, err
		}
		refArg = "refs/heads/" + branch + "^{commit}"
		value = branch
	}
	result, err := p.Git.RunAllowFailure(repo, "rev-parse", "--verify", "--quiet", refArg)
	if err != nil {
		return resolvedCommit{}, err
	}
	if result.ExitCode != 0 {
		return resolvedCommit{}, refuse(label + " does not resolve to a local commit")
	}
	commit := strings.TrimSpace(result.Stdout)
	if !isFullObjectIDForLength(commit, objectIDLength) {
		return resolvedCommit{}, refuse(label + " resolved to an unexpected object id")
	}
	return resolvedCommit{Input: value, Commit: commit}, nil
}

func (p Policy) objectIDLength(repo string) (int, error) {
	result, err := p.Git.RunAllowFailure(repo, "rev-parse", "--show-object-format")
	if err != nil {
		return 0, err
	}
	if result.ExitCode != 0 {
		return 0, refuse("object format inspection failed")
	}
	switch strings.TrimSpace(result.Stdout) {
	case "sha1":
		return 40, nil
	case "sha256":
		return 64, nil
	default:
		return 0, refuse("unsupported Git object format")
	}
}

func isFullObjectIDForLength(value string, length int) bool {
	return len(value) == length && fullObjectIDPattern.MatchString(value)
}

func (p Policy) mergeBase(repo, left, right string) (string, error) {
	result, err := p.Git.RunAllowFailure(repo, "merge-base", left, right)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", refuse("refs do not have a merge base")
	}
	return strings.TrimSpace(result.Stdout), nil
}

func (p Policy) isAncestor(repo, left, right string) (bool, error) {
	result, err := p.Git.RunAllowFailure(repo, "merge-base", "--is-ancestor", left, right)
	if err != nil {
		return false, err
	}
	if result.ExitCode == 0 {
		return true, nil
	}
	if result.ExitCode == 1 {
		return false, nil
	}
	return false, refuse("ancestor check failed: " + strings.TrimSpace(result.Stderr+result.Stdout))
}

func (p Policy) revCount(repo, rangeSpec string) (int, error) {
	result, err := p.Git.Run(repo, "rev-list", "--count", rangeSpec)
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(result.Stdout))
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (p Policy) changedPathSummary(repo, base, target string) (PathListSummary, error) {
	result, err := p.Git.Run(repo, "diff", "--no-ext-diff", "--name-only", "-z", base, target, "--")
	if err != nil {
		return PathListSummary{}, err
	}
	paths := splitNUL(result.Stdout)
	visible := make([]string, 0, len(paths))
	redacted := 0
	for _, path := range paths {
		if secretcheck.IsSecretPath(path) {
			redacted++
			continue
		}
		visible = append(visible, path)
	}
	sort.Strings(visible)
	count := len(visible)
	visible, truncated := limitedSlice(visible, CommitChangedFileLimit)
	return PathListSummary{Files: visible, FileCount: count, FilesTruncated: truncated, FileLimit: CommitChangedFileLimit, RedactedSecretPathCount: redacted}, nil
}

func boundedLimit(limit *int, max int, label string) (int, error) {
	if limit == nil {
		return max, nil
	}
	if *limit < 1 {
		return 0, refuse(label + " must be at least 1")
	}
	if *limit > max {
		return 0, refuse(fmt.Sprintf("%s exceeds maximum %d", label, max))
	}
	return *limit, nil
}

func (p Policy) commitSummaries(repo, ref string, limit int) ([]CommitSummaryEntry, bool, error) {
	result, err := p.Git.Run(repo, "log", "--no-ext-diff", "--format=%H%x09%aI%x09%s", "-n", strconv.Itoa(limit+1), ref, "--")
	if err != nil {
		return nil, false, err
	}
	lines := strings.Split(strings.TrimRight(result.Stdout, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	truncated := len(lines) > limit
	if truncated {
		lines = lines[:limit]
	}
	commits := make([]CommitSummaryEntry, 0, len(lines))
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return nil, false, refuse("unexpected git log format")
		}
		commit, err := p.commitSummaryWithMetadata(repo, fields[0], fields[1], fields[2])
		if err != nil {
			return nil, false, err
		}
		commits = append(commits, commit)
	}
	return commits, truncated, nil
}

func (p Policy) commitSummary(repo, commit string) (CommitSummaryEntry, error) {
	result, err := p.Git.Run(repo, "show", "--no-ext-diff", "--no-patch", "--format=%H%x09%aI%x09%s", commit)
	if err != nil {
		return CommitSummaryEntry{}, err
	}
	fields := strings.SplitN(strings.TrimRight(result.Stdout, "\n"), "\t", 3)
	if len(fields) != 3 {
		return CommitSummaryEntry{}, refuse("unexpected git show format")
	}
	return p.commitSummaryWithMetadata(repo, fields[0], fields[1], fields[2])
}

func (p Policy) commitSummaryWithMetadata(repo, hash, authorDate, subject string) (CommitSummaryEntry, error) {
	changed, err := p.commitChangedFiles(repo, hash)
	if err != nil {
		return CommitSummaryEntry{}, err
	}
	summary, _ := redactSensitiveSummary(subject)
	return CommitSummaryEntry{
		Hash:                    hash,
		Subject:                 summary,
		AuthorDate:              authorDate,
		ChangedFiles:            changed.Files,
		ChangedFileCount:        changed.FileCount,
		ChangedFilesTruncated:   changed.FilesTruncated,
		ChangedFileLimit:        changed.FileLimit,
		RedactedSecretPathCount: changed.RedactedSecretPathCount,
	}, nil
}

func (p Policy) commitChangedFiles(repo, commit string) (PathListSummary, error) {
	result, err := p.Git.Run(repo, "show", "--no-ext-diff", "--format=", "--name-only", "-z", commit, "--")
	if err != nil {
		return PathListSummary{}, err
	}
	paths := splitNUL(result.Stdout)
	visible := make([]string, 0, len(paths))
	redacted := 0
	for _, path := range paths {
		if secretcheck.IsSecretPath(path) {
			redacted++
			continue
		}
		visible = append(visible, path)
	}
	sort.Strings(visible)
	count := len(visible)
	visible, truncated := limitedSlice(visible, CommitChangedFileLimit)
	return PathListSummary{Files: visible, FileCount: count, FilesTruncated: truncated, FileLimit: CommitChangedFileLimit, RedactedSecretPathCount: redacted}, nil
}

func (p Policy) normaliseInspectionPaths(repo string, paths []string) ([]inspectionPath, int, error) {
	if len(paths) == 0 {
		return nil, 0, refuse("at least one path must be listed")
	}
	if len(paths) > PathStatusLimit {
		return nil, 0, refuse(fmt.Sprintf("too many requested paths: limit is %d", PathStatusLimit))
	}
	seen := map[string]struct{}{}
	items := make([]inspectionPath, 0, len(paths))
	redacted := 0
	for _, input := range paths {
		if strings.TrimSpace(input) == "" {
			return nil, 0, refuse("requested paths must be non-empty strings")
		}
		if strings.TrimSpace(input) != input || strings.ContainsRune(input, '\x00') || strings.ContainsAny(input, "\n\r\t") {
			return nil, 0, refuse("requested path has unsafe syntax")
		}
		path := input
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, 0, err
		}
		clean := filepath.Clean(abs)
		rel, err := filepath.Rel(repo, clean)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, 0, refuse("requested path is outside repo")
		}
		rel = filepath.ToSlash(rel)
		if _, ok := seen[rel]; ok {
			return nil, 0, refuse("duplicate requested path: " + rel)
		}
		seen[rel] = struct{}{}
		redactedPath := secretcheck.IsSecretPath(rel)
		if redactedPath {
			redacted++
		}
		items = append(items, inspectionPath{Input: input, Rel: rel, Abs: clean, Redacted: redactedPath})
	}
	return items, redacted, nil
}

func relsFromInspection(items []inspectionPath) []string {
	rels := make([]string, 0, len(items))
	for _, item := range items {
		if !item.Redacted {
			rels = append(rels, item.Rel)
		}
	}
	return rels
}

func (p Policy) statusByPath(repo string) (map[string]StatusEntry, error) {
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return nil, err
	}
	result := map[string]StatusEntry{}
	for _, entry := range entries {
		result[entry.Path] = entry
		if entry.OriginalPath != "" {
			result[entry.OriginalPath] = entry
		}
	}
	return result, nil
}

func (p Policy) lsFileTags(repo string, rels []string) (map[string]string, error) {
	result := map[string]string{}
	if len(rels) == 0 {
		return result, nil
	}
	args := append([]string{"ls-files", "-t", "-z", "--"}, rels...)
	raw, err := p.Git.Run(repo, args...)
	if err != nil {
		return nil, err
	}
	for _, record := range splitNUL(raw.Stdout) {
		if len(record) < 3 {
			continue
		}
		tag := record[:1]
		path := record[2:]
		result[path] = tag
	}
	return result, nil
}

func (p Policy) onePathStatus(repo string, item inspectionPath, status StatusEntry, lsTag string, includeIgnoreSource bool) (PathStatusEntry, error) {
	hasSymlinkComponents, err := pathPrefixContainsSymlink(repo, item.Rel)
	if err != nil {
		return PathStatusEntry{}, err
	}
	var info os.FileInfo
	exists := false
	if !hasSymlinkComponents {
		statInfo, statErr := os.Lstat(item.Abs)
		exists = statErr == nil
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return PathStatusEntry{}, statErr
		}
		info = statInfo
	}
	ignored := false
	var source *IgnoreSource
	if !hasSymlinkComponents {
		ignored, source, err = p.ignoreMatch(repo, item.Rel, includeIgnoreSource)
		if err != nil {
			return PathStatusEntry{}, err
		}
	}
	code := status.Code
	staged := len(code) == 2 && code[0] != ' ' && code[0] != '?'
	modified := strings.Contains(code, "M")
	deleted := strings.Contains(code, "D")
	unmerged := false
	if _, ok := conflictCodes[code]; ok || strings.Contains(code, "U") {
		unmerged = true
	}
	tracked := lsTag != "" && lsTag != "?"
	return PathStatusEntry{
		Path:                 item.Rel,
		Exists:               exists,
		IsDir:                exists && info.IsDir(),
		IsSymlink:            exists && info.Mode()&os.ModeSymlink != 0,
		Tracked:              tracked,
		Ignored:              ignored,
		IgnoreSource:         source,
		Untracked:            code == "??",
		Modified:             modified,
		Deleted:              deleted,
		Staged:               staged,
		Unmerged:             unmerged,
		Sparse:               lsTag == "S",
		SkipWorktree:         lsTag == "S",
		HasSymlinkComponents: hasSymlinkComponents,
	}, nil
}

func pathPrefixContainsSymlink(repo, rel string) (bool, error) {
	current := repo
	parts := strings.Split(filepath.FromSlash(rel), string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 && index < len(parts)-1 {
			return true, nil
		}
	}
	return false, nil
}

func (p Policy) ignoreMatch(repo, rel string, includeSource bool) (bool, *IgnoreSource, error) {
	if strings.HasPrefix(rel, ":") {
		return false, nil, nil
	}
	args := []string{"check-ignore", "-z"}
	if includeSource {
		args = append(args, "-v")
	}
	args = append(args, "--stdin")
	result, err := p.Git.RunNoLiteralPathspecWithInputAllowFailure(repo, rel+"\x00", args...)
	if err != nil {
		return false, nil, err
	}
	if result.ExitCode == 1 {
		return false, nil, nil
	}
	if result.ExitCode != 0 {
		return false, nil, refuse("ignore inspection failed: " + strings.TrimSpace(result.Stderr+result.Stdout))
	}
	if !includeSource {
		return true, nil, nil
	}
	parts := splitNUL(result.Stdout)
	if len(parts) < 4 {
		return true, nil, nil
	}
	lineNumber, _ := strconv.Atoi(parts[1])
	source := parts[0]
	if secretcheck.IsSecretPath(source) {
		source = "[redacted]"
	}
	pattern := parts[2]
	if secretcheck.IsSecretPath(pattern) {
		pattern = "[redacted]"
	}
	return true, &IgnoreSource{Source: source, Line: lineNumber, Pattern: pattern}, nil
}

func (p Policy) submoduleDirtyPaths(repo string) (map[string]bool, error) {
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return nil, err
	}
	dirty := map[string]bool{}
	for _, entry := range entries {
		if entry.Path != "" {
			dirty[entry.Path] = true
		}
	}
	return dirty, nil
}

func parseSubmoduleStatusLine(line string) (SubmoduleEntry, bool) {
	if len(line) < 3 {
		return SubmoduleEntry{}, false
	}
	code := strings.TrimSpace(line[:1])
	rest := strings.TrimSpace(line[1:])
	fields := strings.Fields(rest)
	if len(fields) < 2 || !fullObjectIDPattern.MatchString(fields[0]) {
		return SubmoduleEntry{}, false
	}
	hash := fields[0]
	path := fields[1]
	status := "clean"
	entry := SubmoduleEntry{Path: path, Head: hash, Status: status, StatusCode: code}
	switch code {
	case "-":
		entry.Status = "uninitialised"
		entry.Uninitialised = true
	case "+":
		entry.Status = "out_of_sync"
		entry.OutOfSync = true
	case "U":
		entry.Status = "conflicted"
		entry.Conflicted = true
	case "":
		entry.Status = "clean"
	default:
		entry.Status = "unknown"
	}
	return entry, path != ""
}

func parseIntegrityIssues(raw string) []IntegrityIssueCount {
	counts := map[string]int{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Checking ") {
			continue
		}
		code := integrityIssueCode(line)
		counts[code]++
	}
	issues := make([]IntegrityIssueCount, 0, len(counts))
	for code, count := range counts {
		issues = append(issues, IntegrityIssueCount{Code: code, Severity: integritySeverity(code), Count: count})
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Code < issues[j].Code })
	return issues
}

func integrityIssueCode(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "unknown"
	}
	switch fields[0] {
	case "error:", "fatal:":
		if len(fields) > 1 {
			return strings.TrimSuffix(fields[1], ":")
		}
	case "warning:":
		if len(fields) > 1 {
			return "warning_" + strings.TrimSuffix(fields[1], ":")
		}
		return "warning"
	}
	return strings.TrimSuffix(fields[0], ":")
}

func integritySeverity(code string) string {
	if strings.HasPrefix(code, "warning") {
		return "warning"
	}
	return "error"
}

func (p Policy) normaliseReflogRef(repo, ref string) (string, error) {
	if strings.TrimSpace(ref) != ref || ref == "" || strings.ContainsRune(ref, '\x00') {
		return "", refuse("ref has unsafe syntax")
	}
	if ref == "HEAD" {
		return "HEAD", nil
	}
	branch, err := p.normaliseBranchName(ref)
	if err != nil {
		return "", err
	}
	if _, err := p.requireLocalBranch(repo, branch, "ref"); err != nil {
		return "", err
	}
	return branch, nil
}

func (p Policy) reflogEntries(repo, ref string, limit int) ([]ReflogEntry, int, bool, error) {
	result, err := p.Git.RunAllowFailure(repo, "reflog", "show", "--format=%H%x09%gD%x09%aI%x09%gs", "-n", strconv.Itoa(limit+1), ref)
	if err != nil {
		return nil, 0, false, err
	}
	if result.ExitCode != 0 {
		return nil, 0, false, refuse("reflog is not available for ref")
	}
	lines := strings.Split(strings.TrimRight(result.Stdout, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	truncated := len(lines) > limit
	if truncated {
		lines = lines[:limit]
	}
	entries := make([]ReflogEntry, 0, len(lines))
	redacted := 0
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			return nil, 0, false, refuse("unexpected git reflog format")
		}
		summary, didRedact := redactSensitiveSummary(fields[3])
		if didRedact {
			redacted++
		}
		entries = append(entries, ReflogEntry{Commit: fields[0], Selector: fields[1], Date: fields[2], Summary: summary})
	}
	return entries, redacted, truncated, nil
}

func redactSensitiveSummary(value string) (string, bool) {
	if secretcheck.ContainsLikelySecret(value) || textMentionsSecretPath(value) {
		return "[redacted]", true
	}
	return value, false
}

func textMentionsSecretPath(value string) bool {
	cleaned := strings.NewReplacer(",", " ", ";", " ", ":", " ", "'", " ", "\"", " ", "(", " ", ")", " ").Replace(value)
	for _, field := range strings.Fields(cleaned) {
		if secretcheck.IsSecretPath(strings.Trim(field, "./")) || hasSecretPathComponent(field) {
			return true
		}
	}
	return false
}

func binaryChecksumStatus() (string, string, string) {
	exe, err := os.Executable()
	if err != nil {
		return "", "", "unavailable"
	}
	exe = canonicalCleanPath(exe)
	pathHashBytes := sha256.Sum256([]byte(exe))
	pathHash := hex.EncodeToString(pathHashBytes[:])
	data, err := os.ReadFile(exe)
	if err != nil {
		return pathHash, "", "unavailable"
	}
	actualBytes := sha256.Sum256(data)
	actual := hex.EncodeToString(actualBytes[:])
	checksumPath := exe + ".sha256"
	raw, err := os.ReadFile(checksumPath)
	if errors.Is(err, os.ErrNotExist) {
		return pathHash, actual, "missing"
	}
	if err != nil {
		return pathHash, actual, "unavailable"
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return pathHash, actual, "mismatched"
	}
	if fields[0] != actual {
		return pathHash, actual, "mismatched"
	}
	return pathHash, actual, "matched"
}

func allowlistFingerprints(repos, roots map[string]struct{}) []string {
	values := make([]string, 0, len(repos)+len(roots))
	for repo := range repos {
		sum := sha256.Sum256([]byte("repo:" + repo))
		values = append(values, "repo:"+hex.EncodeToString(sum[:]))
	}
	for root := range roots {
		sum := sha256.Sum256([]byte("root:" + root))
		values = append(values, "root:"+hex.EncodeToString(sum[:]))
	}
	sort.Strings(values)
	return values
}

func auditLogLikelyWritable(path string) bool {
	if path == "" || hasSecretPathComponent(path) {
		return false
	}
	if info, err := os.Stat(path); err == nil {
		return !info.IsDir() && info.Mode().Perm()&0o200 != 0
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o200 != 0
}
