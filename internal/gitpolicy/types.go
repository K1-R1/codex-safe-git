package gitpolicy

const (
	StatusEntryLimit       = 200
	DiffFileLimit          = 200
	UntrackedLimit         = 200
	WorktreeLimit          = 100
	CommitFileLimit        = 200
	BranchLimit            = 200
	RefLimit               = 300
	LogCommitLimit         = 50
	CommitChangedFileLimit = 200
	PathStatusLimit        = 200
	SubmoduleLimit         = 100
	IntegrityIssueLimit    = 100
	ReflogLimit            = 50
)

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
	EntryCount              int           `json:"entry_count"`
	EntriesTruncated        bool          `json:"entries_truncated"`
	EntryLimit              int           `json:"entry_limit"`
	RedactedSecretPathCount int           `json:"redacted_secret_path_count"`
}

type FileStat struct {
	Path      string `json:"path"`
	Additions *int   `json:"additions"`
	Deletions *int   `json:"deletions"`
}

type FileSummary struct {
	Files                   []FileStat `json:"files"`
	FileCount               int        `json:"file_count"`
	FilesTruncated          bool       `json:"files_truncated"`
	FileLimit               int        `json:"file_limit"`
	RedactedSecretPathCount int        `json:"redacted_secret_path_count"`
}

type UntrackedSummary struct {
	Files                   []string `json:"files"`
	FileCount               int      `json:"file_count"`
	FilesTruncated          bool     `json:"files_truncated"`
	FileLimit               int      `json:"file_limit"`
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

type LocalBranchEntry struct {
	Name         string `json:"name"`
	Head         string `json:"head"`
	IsCurrent    bool   `json:"is_current"`
	IsProtected  bool   `json:"is_protected"`
	IsCheckedOut bool   `json:"is_checked_out"`
}

type ListLocalBranchesResult struct {
	Result            string             `json:"result"`
	Repo              string             `json:"repo"`
	Branches          []LocalBranchEntry `json:"branches"`
	BranchCount       int                `json:"branch_count"`
	BranchesTruncated bool               `json:"branches_truncated"`
	BranchLimit       int                `json:"branch_limit"`
}

type LocalRefEntry struct {
	Ref         string `json:"ref"`
	ShortName   string `json:"short_name"`
	Kind        string `json:"kind"`
	TargetHash  string `json:"target_hash"`
	TargetType  string `json:"target_type"`
	IsProtected bool   `json:"is_protected"`
	IsVisible   bool   `json:"is_visible"`
}

type ListLocalRefsResult struct {
	Result        string          `json:"result"`
	Repo          string          `json:"repo"`
	Refs          []LocalRefEntry `json:"refs"`
	RefCount      int             `json:"ref_count"`
	RefsTruncated bool            `json:"refs_truncated"`
	RefLimit      int             `json:"ref_limit"`
}

type MergeBaseResult struct {
	Result      string `json:"result"`
	Repo        string `json:"repo"`
	LeftRef     string `json:"left_ref"`
	RightRef    string `json:"right_ref"`
	LeftCommit  string `json:"left_commit"`
	RightCommit string `json:"right_commit"`
	MergeBase   string `json:"merge_base"`
	IsAncestor  bool   `json:"is_ancestor"`
}

type CompareRefsResult struct {
	Result                  string `json:"result"`
	Repo                    string `json:"repo"`
	BaseRef                 string `json:"base_ref"`
	TargetRef               string `json:"target_ref"`
	BaseCommit              string `json:"base_commit"`
	TargetCommit            string `json:"target_commit"`
	MergeBase               string `json:"merge_base"`
	AheadCount              int    `json:"ahead_count"`
	BehindCount             int    `json:"behind_count"`
	ChangedFileCount        int    `json:"changed_file_count"`
	RedactedSecretPathCount int    `json:"redacted_secret_path_count"`
}

type PathListSummary struct {
	Files                   []string `json:"files"`
	FileCount               int      `json:"file_count"`
	FilesTruncated          bool     `json:"files_truncated"`
	FileLimit               int      `json:"file_limit"`
	RedactedSecretPathCount int      `json:"redacted_secret_path_count"`
}

type ChangedFilesBetweenRefsResult struct {
	Result       string          `json:"result"`
	Repo         string          `json:"repo"`
	BaseRef      string          `json:"base_ref"`
	TargetRef    string          `json:"target_ref"`
	BaseCommit   string          `json:"base_commit"`
	TargetCommit string          `json:"target_commit"`
	Changed      PathListSummary `json:"changed"`
}

type CommitSummaryEntry struct {
	Hash                    string   `json:"hash"`
	Subject                 string   `json:"subject"`
	AuthorDate              string   `json:"author_date"`
	ChangedFiles            []string `json:"changed_files"`
	ChangedFileCount        int      `json:"changed_file_count"`
	ChangedFilesTruncated   bool     `json:"changed_files_truncated"`
	ChangedFileLimit        int      `json:"changed_file_limit"`
	RedactedSecretPathCount int      `json:"redacted_secret_path_count"`
}

type CommitLogSummaryResult struct {
	Result           string               `json:"result"`
	Repo             string               `json:"repo"`
	Ref              string               `json:"ref"`
	ResolvedCommit   string               `json:"resolved_commit"`
	Commits          []CommitSummaryEntry `json:"commits"`
	CommitCount      int                  `json:"commit_count"`
	CommitsTruncated bool                 `json:"commits_truncated"`
	CommitLimit      int                  `json:"commit_limit"`
}

type ShowCommitSummaryResult struct {
	Result    string             `json:"result"`
	Repo      string             `json:"repo"`
	CommitRef string             `json:"commit_ref"`
	Commit    CommitSummaryEntry `json:"commit"`
}

type IgnoreSource struct {
	Source  string `json:"source"`
	Line    int    `json:"line"`
	Pattern string `json:"pattern"`
}

type PathStatusEntry struct {
	Path                 string        `json:"path"`
	Exists               bool          `json:"exists"`
	IsDir                bool          `json:"is_dir"`
	IsSymlink            bool          `json:"is_symlink"`
	Tracked              bool          `json:"tracked"`
	Ignored              bool          `json:"ignored"`
	IgnoreSource         *IgnoreSource `json:"ignore_source"`
	Untracked            bool          `json:"untracked"`
	Modified             bool          `json:"modified"`
	Deleted              bool          `json:"deleted"`
	Staged               bool          `json:"staged"`
	Unmerged             bool          `json:"unmerged"`
	Sparse               bool          `json:"sparse"`
	SkipWorktree         bool          `json:"skip_worktree"`
	HasSymlinkComponents bool          `json:"has_symlink_components"`
}

type PathStatusResult struct {
	Result                  string            `json:"result"`
	Repo                    string            `json:"repo"`
	Entries                 []PathStatusEntry `json:"entries"`
	EntryCount              int               `json:"entry_count"`
	EntriesTruncated        bool              `json:"entries_truncated"`
	EntryLimit              int               `json:"entry_limit"`
	RedactedSecretPathCount int               `json:"redacted_secret_path_count"`
}

type SubmoduleEntry struct {
	Path          string `json:"path"`
	Head          string `json:"head"`
	Status        string `json:"status"`
	StatusCode    string `json:"status_code"`
	Dirty         bool   `json:"dirty"`
	Uninitialised bool   `json:"uninitialised"`
	OutOfSync     bool   `json:"out_of_sync"`
	Conflicted    bool   `json:"conflicted"`
}

type SubmoduleSummaryResult struct {
	Result                  string           `json:"result"`
	Repo                    string           `json:"repo"`
	Submodules              []SubmoduleEntry `json:"submodules"`
	SubmoduleCount          int              `json:"submodule_count"`
	SubmodulesTruncated     bool             `json:"submodules_truncated"`
	SubmoduleLimit          int              `json:"submodule_limit"`
	RedactedSecretPathCount int              `json:"redacted_secret_path_count"`
}

type IntegrityIssueCount struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

type RepositoryIntegrityCheckResult struct {
	Result          string                `json:"result"`
	Repo            string                `json:"repo"`
	IntegrityOK     bool                  `json:"integrity_ok"`
	ExitCode        int                   `json:"exit_code"`
	Issues          []IntegrityIssueCount `json:"issues"`
	IssueCount      int                   `json:"issue_count"`
	IssuesTruncated bool                  `json:"issues_truncated"`
	IssueLimit      int                   `json:"issue_limit"`
}

type ReflogEntry struct {
	Selector string `json:"selector"`
	Commit   string `json:"commit"`
	Summary  string `json:"summary"`
	Date     string `json:"date"`
}

type ReflogSummaryResult struct {
	Result                        string        `json:"result"`
	Repo                          string        `json:"repo"`
	Ref                           string        `json:"ref"`
	Entries                       []ReflogEntry `json:"entries"`
	EntryCount                    int           `json:"entry_count"`
	EntriesTruncated              bool          `json:"entries_truncated"`
	EntryLimit                    int           `json:"entry_limit"`
	RedactedSensitiveSummaryCount int           `json:"redacted_sensitive_summary_count"`
}

type SelfCheckResult struct {
	Result                  string   `json:"result"`
	Repo                    string   `json:"repo"`
	ServerVersion           string   `json:"server_version"`
	ProtocolVersion         string   `json:"protocol_version"`
	GitVersion              string   `json:"git_version"`
	ToolNames               []string `json:"tool_names"`
	ToolCount               int      `json:"tool_count"`
	BinaryPathHash          string   `json:"binary_path_hash"`
	BinaryChecksum          string   `json:"binary_checksum"`
	ChecksumStatus          string   `json:"checksum_status"`
	AuditLogConfigured      bool     `json:"audit_log_configured"`
	AuditLogWritable        bool     `json:"audit_log_writable"`
	AllowedRepoCount        int      `json:"allowed_repo_count"`
	AllowedRepoRootCount    int      `json:"allowed_repo_root_count"`
	AllowlistFingerprints   []string `json:"allowlist_fingerprints"`
	ProtectedBranchCount    int      `json:"protected_branch_count"`
	RedactedConfigPathCount int      `json:"redacted_config_path_count"`
}

type ListWorktreesResult struct {
	Result                     string          `json:"result"`
	Repo                       string          `json:"repo"`
	Worktrees                  []WorktreeEntry `json:"worktrees"`
	WorktreeCount              int             `json:"worktree_count"`
	WorktreesTruncated         bool            `json:"worktrees_truncated"`
	WorktreeLimit              int             `json:"worktree_limit"`
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
