package gitpolicy

type Refusal struct {
	Reason string
}

func (r Refusal) Error() string {
	return r.Reason
}

type StatusEntry struct {
	Code         string `json:"code"`
	Path         string `json:"path"`
	OriginalPath string `json:"-"`
}

type StatusResult struct {
	Result                  string        `json:"result"`
	Repo                    string        `json:"repo"`
	Branch                  *string       `json:"branch"`
	IsDetached              bool          `json:"is_detached"`
	AmbiguousReasons        []string      `json:"ambiguous_reasons"`
	HasStagedChanges        bool          `json:"has_staged_changes"`
	Clean                   bool          `json:"clean"`
	Entries                 []StatusEntry `json:"entries"`
	RedactedSecretPathCount int           `json:"redacted_secret_path_count"`
}

type FileStat struct {
	Path      string `json:"path"`
	Additions *int   `json:"additions"`
	Deletions *int   `json:"deletions"`
}

type FileSummary struct {
	Files                   []FileStat `json:"files"`
	RedactedSecretPathCount int        `json:"redacted_secret_path_count"`
}

type UntrackedSummary struct {
	Files                   []string `json:"files"`
	RedactedSecretPathCount int      `json:"redacted_secret_path_count"`
}

type DiffSummaryResult struct {
	Result    string           `json:"result"`
	Repo      string           `json:"repo"`
	Unstaged  FileSummary      `json:"unstaged"`
	Staged    FileSummary      `json:"staged"`
	Untracked UntrackedSummary `json:"untracked"`
}

type CommitResult struct {
	Result       string   `json:"result"`
	Repo         string   `json:"repo"`
	CommitHash   string   `json:"commit_hash"`
	Files        []string `json:"files"`
	AuditSummary string   `json:"audit_summary"`
}

type BranchResult struct {
	Result     string `json:"result"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Action     string `json:"action"`
	HeadCommit string `json:"head_commit"`
}

type MergeResult struct {
	Result           string `json:"result"`
	Repo             string `json:"repo"`
	SourceBranch     string `json:"source_branch"`
	TargetBranch     string `json:"target_branch"`
	Action           string `json:"action"`
	SourceHead       string `json:"source_head"`
	TargetHeadBefore string `json:"target_head_before"`
	TargetHeadAfter  string `json:"target_head_after"`
}

type WorktreeEntry struct {
	Path       string  `json:"path"`
	Head       string  `json:"head"`
	Branch     *string `json:"branch"`
	IsCurrent  bool    `json:"is_current"`
	IsDetached bool    `json:"is_detached"`
	IsBare     bool    `json:"is_bare"`
	IsLocked   bool    `json:"is_locked"`
	IsPrunable bool    `json:"is_prunable"`
}

type ListWorktreesResult struct {
	Result                     string          `json:"result"`
	Repo                       string          `json:"repo"`
	Worktrees                  []WorktreeEntry `json:"worktrees"`
	RedactedUnallowlistedCount int             `json:"redacted_unallowlisted_count"`
}

type CreateWorktreeResult struct {
	Result       string  `json:"result"`
	Repo         string  `json:"repo"`
	WorktreePath string  `json:"worktree_path"`
	Branch       string  `json:"branch"`
	BaseBranch   *string `json:"base_branch"`
	BaseHead     string  `json:"base_head"`
	HeadCommit   string  `json:"head_commit"`
	Action       string  `json:"action"`
}

type CheckoutResult struct {
	Result     string `json:"result"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	Action     string `json:"action"`
	HeadCommit string `json:"head_commit"`
}

type RefusalResult struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}
