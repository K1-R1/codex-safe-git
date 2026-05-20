package gitpolicy

type Refusal struct {
	Reason string
}

func (r Refusal) Error() string {
	return r.Reason
}

type StatusEntry struct {
	Code string `json:"code"`
	Path string `json:"path"`
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

type RefusalResult struct {
	Result string `json:"result"`
	Reason string `json:"reason"`
}
