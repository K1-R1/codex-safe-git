package gitpolicy

import (
	"regexp"
	"sort"
	"strings"

	"local/codex-safe-git/internal/audit"
	"local/codex-safe-git/internal/config"
	"local/codex-safe-git/internal/gitexec"
)

var (
	aiAttributionPattern = regexp.MustCompile(`(?i)(generated[- ]?by\s+(codex|chatgpt|openai)|co-authored-by:.*\b(codex|chatgpt|openai)\b|\b(via|with|using)\s+(codex|chatgpt|openai)\b|\b(codex|chatgpt|openai)\s+(generated|assisted|authored)\b)`)
	hexBranchPattern     = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
	executionConfigRE    = regexp.MustCompile(`^(filter\..*\.(clean|process|smudge)|diff\..*\.(command|textconv)|core\.fsmonitor)$`)
)

var protectedBranches = map[string]struct{}{"main": {}, "master": {}}
var remoteLikePrefixes = map[string]struct{}{"origin": {}, "upstream": {}, "remotes": {}}
var conflictCodes = map[string]struct{}{"DD": {}, "AU": {}, "UD": {}, "UA": {}, "DU": {}, "AA": {}, "UU": {}}

type Policy struct {
	Config config.Config
	Git    gitexec.Runner
	Audit  audit.Logger
}

func New(cfg config.Config) (Policy, error) {
	runner, err := gitexec.NewRunner()
	if err != nil {
		return Policy{}, err
	}
	return Policy{Config: cfg, Git: runner, Audit: audit.Logger{Path: cfg.AuditLog}}, nil
}

func (p Policy) GitStatus(repoPath string) (StatusResult, error) {
	repo, err := p.resolveAllowedRepo(repoPath)
	if err != nil {
		p.auditRefusal("git_status", repoPath, err)
		return StatusResult{}, err
	}
	if err := p.requireGitRepo(repo); err != nil {
		p.auditRefusal("git_status", repoPath, err)
		return StatusResult{}, err
	}
	if err := p.requireNoExecutionConfig(repo); err != nil {
		p.auditRefusal("git_status", repoPath, err)
		return StatusResult{}, err
	}
	result, err := p.status(repo)
	if err != nil {
		p.auditRefusal("git_status", repoPath, err)
		return StatusResult{}, err
	}
	p.Audit.Write(audit.Entry{Action: "git_status", Result: "ok", Repo: repo, FileCount: audit.IntPtr(len(result.Entries))})
	return result, nil
}

func (p Policy) GitDiffSummary(repoPath string) (DiffSummaryResult, error) {
	repo, err := p.resolveAllowedRepo(repoPath)
	if err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	if err := p.requireGitRepo(repo); err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	if err := p.requireNoExecutionConfig(repo); err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	state, err := p.repoState(repo)
	if err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	if len(state.AmbiguousReasons) > 0 {
		err := refuse("repository has ambiguous state: " + strings.Join(state.AmbiguousReasons, ", "))
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	unstaged, err := p.numstat(repo, []string{"diff", "--no-ext-diff", "--numstat", "-z"})
	if err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	staged, err := p.numstat(repo, []string{"diff", "--cached", "--no-ext-diff", "--numstat", "-z"})
	if err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	untracked, err := p.safeUntracked(repo)
	if err != nil {
		p.auditRefusal("git_diff_summary", repoPath, err)
		return DiffSummaryResult{}, err
	}
	total := len(unstaged.Files) + len(staged.Files) + len(untracked.Files)
	redacted := unstaged.RedactedSecretPathCount + staged.RedactedSecretPathCount + untracked.RedactedSecretPathCount
	p.Audit.Write(audit.Entry{
		Action:                  "git_diff_summary",
		Result:                  "ok",
		Repo:                    repo,
		FileCount:               audit.IntPtr(total),
		RedactedSecretPathCount: audit.IntPtr(redacted),
	})
	return DiffSummaryResult{Result: "ok", Repo: repo, Unstaged: unstaged, Staged: staged, Untracked: untracked}, nil
}

func (p Policy) EnsureCommitBranch(repoPath, branchName string) (BranchResult, error) {
	return p.prepareBranch("ensure_commit_branch", repoPath, branchName, false)
}

func (p Policy) CreateCommitBranch(repoPath, branchName string) (BranchResult, error) {
	return p.prepareBranch("create_commit_branch", repoPath, branchName, true)
}

func (p Policy) prepareBranch(actionName, repoPath, branchName string, allowNamedBranchSwitch bool) (BranchResult, error) {
	branch, branchErr := p.normaliseBranchName(branchName)
	repo, err := p.resolveAllowedRepo(repoPath)
	if err == nil {
		err = p.requireGitRepo(repo)
	}
	if err == nil {
		err = p.requireNoExecutionConfig(repo)
	}
	if err == nil {
		err = p.requireBranchNotProtected(repo, branch, "branch")
	}
	var state repoState
	if err == nil {
		state, err = p.repoState(repo)
	}
	if err == nil {
		err = requireBranchPrepState(state)
	}
	if branchErr != nil {
		err = branchErr
	}
	if err != nil {
		p.Audit.Write(audit.Entry{Action: actionName, Result: "refused", Repo: repoPath, Branch: branchOrInput(branch, branchName), Reason: err.Error()})
		return BranchResult{}, err
	}

	head, err := p.currentHead(repo)
	if err != nil {
		p.Audit.Write(audit.Entry{Action: actionName, Result: "refused", Repo: repoPath, Branch: branch, Reason: err.Error()})
		return BranchResult{}, err
	}
	var mutation string
	if state.Branch != nil && *state.Branch == branch {
		mutation = "already_on_branch"
	} else if state.Branch != nil && !allowNamedBranchSwitch {
		err = refuse("repository is already on a different branch: " + *state.Branch)
	} else {
		var existing string
		existing, err = p.localBranchHead(repo, branch)
		if err == nil && existing == "" {
			_, err = p.Git.Run(repo, "switch", "-c", branch)
			mutation = "created"
		} else if err == nil && existing == head {
			_, err = p.Git.Run(repo, "switch", branch)
			if allowNamedBranchSwitch {
				mutation = "switched"
			} else {
				mutation = "attached"
			}
		} else if err == nil {
			err = refuse("requested branch already exists at a different commit")
		}
	}
	if err != nil {
		p.Audit.Write(audit.Entry{Action: actionName, Result: "refused", Repo: repoPath, Branch: branch, Reason: err.Error()})
		return BranchResult{}, err
	}
	p.Audit.Write(audit.Entry{Action: actionName, Result: mutation, Repo: repo, Branch: branch, CommitHash: head})
	return BranchResult{Result: "ok", Repo: repo, Branch: branch, Action: mutation, HeadCommit: head}, nil
}

func (p Policy) MergeBranch(repoPath, sourceBranch string, targetBranch *string) (MergeResult, error) {
	source, sourceErr := p.normaliseBranchName(sourceBranch)
	target := ""
	var targetErr error
	if targetBranch != nil {
		target, targetErr = p.normaliseBranchName(*targetBranch)
	}
	repo, err := p.resolveAllowedRepo(repoPath)
	if err == nil {
		err = p.requireGitRepo(repo)
	}
	if err == nil {
		err = p.requireNoExecutionConfig(repo)
	}
	var state repoState
	if err == nil {
		state, err = p.requireCleanWorktreeState(repo)
	}
	if sourceErr != nil {
		err = sourceErr
	}
	if targetErr != nil {
		err = targetErr
	}
	if err == nil {
		if state.Branch == nil {
			err = refuse("repository is detached; prepare a branch before merging")
		} else if target != "" && *state.Branch != target {
			err = refuse("target_branch must match the current branch")
		} else if target == "" {
			target = *state.Branch
		}
	}
	if err == nil && source == target {
		err = refuse("source_branch and target_branch must differ")
	}
	if err == nil {
		err = p.requireBranchNotProtected(repo, target, "target branch")
	}
	var sourceHead, targetHead string
	if err == nil {
		sourceHead, err = p.requireLocalBranch(repo, source, "source_branch")
	}
	if err == nil {
		targetHead, err = p.requireLocalBranch(repo, target, "target_branch")
	}
	if err != nil {
		p.Audit.Write(audit.Entry{Action: "merge_branch", Result: "refused", Repo: repoPath, SourceBranch: branchOrInput(source, sourceBranch), TargetBranch: branchOrInput(target, optionalBranch(targetBranch)), Reason: err.Error()})
		return MergeResult{}, err
	}
	if _, err = p.Git.Run(repo, "merge", "--ff-only", source); err != nil {
		refusal := refuse(err.Error())
		p.Audit.Write(audit.Entry{Action: "merge_branch", Result: "refused", Repo: repoPath, SourceBranch: source, TargetBranch: target, Reason: refusal.Error()})
		return MergeResult{}, refusal
	}
	mergedHead, err := p.currentHead(repo)
	if err != nil {
		p.Audit.Write(audit.Entry{Action: "merge_branch", Result: "refused", Repo: repoPath, SourceBranch: source, TargetBranch: target, Reason: err.Error()})
		return MergeResult{}, err
	}
	action := "fast_forwarded"
	if mergedHead == targetHead {
		action = "already_up_to_date"
	}
	p.Audit.Write(audit.Entry{Action: "merge_branch", Result: action, Repo: repo, Branch: target, SourceBranch: source, TargetBranch: target, CommitHash: mergedHead})
	return MergeResult{
		Result:           "ok",
		Repo:             repo,
		SourceBranch:     source,
		TargetBranch:     target,
		Action:           action,
		SourceHead:       sourceHead,
		TargetHeadBefore: targetHead,
		TargetHeadAfter:  mergedHead,
	}, nil
}

func (p Policy) CommitFiles(repoPath string, files []string, message string, body *string) (CommitResult, error) {
	fileList := append([]string(nil), files...)
	if err := rejectAttribution(message, body); err != nil {
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	repo, err := p.resolveAllowedRepo(repoPath)
	if err == nil {
		var requested []string
		requested, err = p.normaliseFiles(repo, fileList)
		fileList = requested
	}
	if err == nil {
		err = p.requireGitRepo(repo)
	}
	if err == nil {
		err = p.requireNoExecutionConfig(repo)
	}
	var state repoState
	if err == nil {
		state, err = p.requireClearCommitState(repo)
	}
	if err == nil {
		err = p.requireCommitBranchAllowed(repo, state)
	}
	if err == nil {
		err = p.rejectLikelySecretMaterial(repo, fileList)
	}
	if err != nil {
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	if _, err = p.Git.Run(repo, append([]string{"add", "--"}, fileList...)...); err != nil {
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	staged, err := p.stagedFiles(repo)
	if err != nil {
		p.unstageBestEffort(repo, fileList)
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	if p.isIndexEmpty(repo) {
		p.unstageBestEffort(repo, fileList)
		err := refuse("refusing empty commit")
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	if !sameStringSet(staged, fileList) {
		p.unstageBestEffort(repo, fileList)
		err := refuse("staged file set does not exactly match requested files")
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	commitArgs := []string{"commit", "--no-gpg-sign", "-m", message}
	if body != nil && *body != "" {
		commitArgs = append(commitArgs, "-m", *body)
	}
	if _, err = p.Git.Run(repo, commitArgs...); err != nil {
		p.unstageBestEffort(repo, fileList)
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	head, err := p.currentHead(repo)
	if err != nil {
		p.Audit.Write(audit.Entry{Action: "commit_files", Result: "refused", Repo: repoPath, Files: fileList, Reason: err.Error()})
		return CommitResult{}, err
	}
	sort.Strings(fileList)
	p.Audit.Write(audit.Entry{Action: "commit_files", Result: "committed", Repo: repo, Files: fileList, CommitHash: head})
	return CommitResult{
		Result:       "committed",
		Repo:         repo,
		CommitHash:   head,
		Files:        fileList,
		AuditSummary: "allowlisted repo; clear state; exact staged file set; no likely secrets; attribution-free message",
	}, nil
}
