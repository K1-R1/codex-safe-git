package gitpolicy

import (
	"regexp"
	"sort"
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
