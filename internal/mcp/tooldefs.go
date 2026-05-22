package mcp

import "local/codex-safe-git/internal/gitpolicy"

func tools() []map[string]any {
	return []map[string]any{
		tool("git_status", "Git Status", "Return a redacted, bounded, read-only status summary for an explicitly allowed local Git repo.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("Git Status"), statusOutputSchema()),
		tool("git_diff_summary", "Git Diff Summary", "Return redacted, bounded file-level diff counts for an explicitly allowed local Git repo.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("Git Diff Summary"), diffSummaryOutputSchema()),
		tool("list_local_branches", "List Local Branches", "Return bounded local branch metadata without contacting remotes.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("List Local Branches"), listLocalBranchesOutputSchema()),
		tool("compare_refs", "Compare Refs", "Summarise merge-base, ahead/behind counts, and changed-file counts for two safe local refs.", []string{"repo_path", "base_ref", "target_ref"}, map[string]any{
			"repo_path":  stringInput("Exact Git worktree root to inspect."),
			"base_ref":   stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
			"target_ref": stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
		}, readOnlyAnnotations("Compare Refs"), compareRefsOutputSchema()),
		tool("commit_log_summary", "Commit Log Summary", "Return recent bounded commit metadata without patch text.", []string{"repo_path"}, map[string]any{
			"repo_path": stringInput("Exact Git worktree root to inspect."),
			"ref":       stringInput("Optional safe local branch name, HEAD, refs/heads/*, or full commit hash. Defaults to HEAD."),
			"limit":     limitInput(gitpolicy.LogCommitLimit, "Optional maximum number of commits to return."),
		}, readOnlyAnnotations("Commit Log Summary"), commitLogSummaryOutputSchema()),
		tool("show_commit_summary", "Show Commit Summary", "Return bounded metadata and changed-file names for one local commit without patch text.", []string{"repo_path", "commit_ref"}, map[string]any{
			"repo_path":  stringInput("Exact Git worktree root to inspect."),
			"commit_ref": stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
		}, readOnlyAnnotations("Show Commit Summary"), showCommitSummaryOutputSchema()),
		tool("list_local_refs", "List Local Refs", "Return bounded local branch and tag refs with target metadata.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("List Local Refs"), listLocalRefsOutputSchema()),
		tool("merge_base", "Merge Base", "Return the merge base for two safe local refs.", []string{"repo_path", "left_ref", "right_ref"}, map[string]any{
			"repo_path": stringInput("Exact Git worktree root to inspect."),
			"left_ref":  stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
			"right_ref": stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
		}, readOnlyAnnotations("Merge Base"), mergeBaseOutputSchema()),
		tool("changed_files_between_refs", "Changed Files Between Refs", "Return bounded changed-file names and counts between two safe local refs without patch text.", []string{"repo_path", "base_ref", "target_ref"}, map[string]any{
			"repo_path":  stringInput("Exact Git worktree root to inspect."),
			"base_ref":   stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
			"target_ref": stringInput("Safe local branch name, HEAD, refs/heads/*, or full commit hash."),
		}, readOnlyAnnotations("Changed Files Between Refs"), changedFilesBetweenRefsOutputSchema()),
		tool("path_status", "Path Status", "Return bounded exact-path Git state for explicit paths without reading file contents.", []string{"repo_path", "paths"}, map[string]any{
			"repo_path":             stringInput("Exact Git worktree root to inspect."),
			"paths":                 map[string]any{"type": "array", "items": stringInput("Repo-relative path or absolute path inside the repo."), "minItems": 1, "maxItems": gitpolicy.PathStatusLimit, "uniqueItems": true},
			"include_ignore_source": map[string]any{"type": "boolean", "description": "Whether to include redacted ignore-source metadata when a path is ignored."},
		}, readOnlyAnnotations("Path Status"), pathStatusOutputSchema()),
		tool("submodule_summary", "Submodule Summary", "Return bounded read-only submodule state without cloning, fetching, or updating.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("Submodule Summary"), submoduleSummaryOutputSchema()),
		tool("repository_integrity_check", "Repository Integrity Check", "Run a bounded read-only connectivity diagnostic without repair or lost-found writes.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("Repository Integrity Check"), repositoryIntegrityCheckOutputSchema()),
		tool("reflog_summary", "Reflog Summary", "Return recent bounded HEAD or local-branch reflog movements without expiry, deletion, or mutation.", []string{"repo_path"}, map[string]any{
			"repo_path": stringInput("Exact Git worktree root to inspect."),
			"ref":       stringInput("Optional HEAD or safe local branch name. Defaults to HEAD."),
			"limit":     limitInput(gitpolicy.ReflogLimit, "Optional maximum number of reflog entries to return."),
		}, readOnlyAnnotations("Reflog Summary"), reflogSummaryOutputSchema()),
		tool("self_check", "Self Check", "Return bounded MCP version, tool-surface, checksum, audit, and redacted allowlist health metadata.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to use for local Git health checks.")}, readOnlyAnnotations("Self Check"), selfCheckOutputSchema()),
		tool("commit_files", "Commit Files", "Create a local commit from exactly listed files after deterministic safety checks.", []string{"repo_path", "files", "message"}, map[string]any{
			"repo_path": stringInput("Exact Git worktree root where the commit will be created."),
			"files":     map[string]any{"type": "array", "items": stringInput("Repo-relative file path."), "minItems": 1, "maxItems": gitpolicy.CommitFileLimit, "uniqueItems": true},
			"message":   boundedStringInput("Commit subject without AI/tool attribution or secret material.", gitpolicy.CommitMessageSubjectLimit),
			"body":      optionalBoundedStringInput("Optional commit body without AI/tool attribution or secret material.", gitpolicy.CommitMessageBodyLimit),
		}, mutatingAnnotations("Commit Files", false), commitOutputSchema()),
		tool("ensure_commit_branch", "Ensure Commit Branch", "Attach a detached worktree to a safe local non-default branch at current HEAD.", []string{"repo_path", "branch_name"}, map[string]any{
			"repo_path":   stringInput("Exact Git worktree root to prepare."),
			"branch_name": stringInput("Safe local branch name to create or attach."),
		}, mutatingAnnotations("Ensure Commit Branch", false), branchOutputSchema()),
		tool("create_commit_branch", "Create Commit Branch", "Create or switch to a safe local non-default branch at current HEAD.", []string{"repo_path", "branch_name"}, map[string]any{
			"repo_path":   stringInput("Exact Git worktree root to prepare."),
			"branch_name": stringInput("Safe local branch name to create or switch to."),
		}, mutatingAnnotations("Create Commit Branch", false), branchOutputSchema()),
		tool("merge_branch", "Merge Branch", "Fast-forward a clean non-default local target branch from another local branch.", []string{"repo_path", "source_branch"}, map[string]any{
			"repo_path":     stringInput("Exact Git worktree root currently on the target branch."),
			"source_branch": stringInput("Existing safe local branch to fast-forward from."),
			"target_branch": stringInput("Optional current target branch guard."),
		}, mutatingAnnotations("Merge Branch", false), mergeOutputSchema()),
		tool("list_worktrees", "List Worktrees", "Return local worktrees visible under the configured allowlist, redacting unallowlisted paths.", []string{"repo_path"}, map[string]any{
			"repo_path": stringInput("Exact Git worktree root whose linked worktrees should be listed."),
		}, readOnlyAnnotations("List Worktrees"), listWorktreesOutputSchema()),
		tool("create_worktree", "Create Worktree", "Create a linked local worktree on a new safe non-protected branch under an allowed root.", []string{"repo_path", "worktree_path", "branch_name"}, map[string]any{
			"repo_path":     stringInput("Exact source Git worktree root."),
			"worktree_path": stringInput("New allowed path for the linked worktree."),
			"branch_name":   stringInput("New safe local branch for the linked worktree."),
			"base_branch":   stringInput("Optional existing safe local branch to base from."),
		}, mutatingAnnotations("Create Worktree", false), createWorktreeOutputSchema()),
		tool("safe_checkout", "Safe Checkout", "Switch a clean worktree to an existing safe non-protected local branch.", []string{"repo_path", "branch_name"}, map[string]any{
			"repo_path":   stringInput("Exact Git worktree root to switch."),
			"branch_name": stringInput("Existing safe local branch to switch to."),
		}, mutatingAnnotations("Safe Checkout", false), branchOutputSchema()),
	}
}

func toolNames() []string {
	names := make([]string, 0, len(tools()))
	for _, tool := range tools() {
		if name, ok := tool["name"].(string); ok {
			names = append(names, name)
		}
	}
	return names
}

func tool(name, title, description string, required []string, properties map[string]any, annotations map[string]any, outputSchema map[string]any) map[string]any {
	return map[string]any{
		"name":        name,
		"title":       title,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
		"annotations":  annotations,
		"outputSchema": outputSchema,
	}
}

func stringInput(description string) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "description": description}
}

func boundedStringInput(description string, max int) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": max, "description": description}
}

func optionalBoundedStringInput(description string, max int) map[string]any {
	return map[string]any{"type": "string", "maxLength": max, "description": description}
}

func limitInput(max int, description string) map[string]any {
	return map[string]any{"type": "integer", "minimum": 1, "maximum": max, "description": description}
}

func readOnlyAnnotations(title string) map[string]any {
	return map[string]any{"title": title, "readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false}
}

func mutatingAnnotations(title string, idempotent bool) map[string]any {
	return map[string]any{"title": title, "readOnlyHint": false, "destructiveHint": false, "idempotentHint": idempotent, "openWorldHint": false}
}

func statusOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "branch", "is_detached", "ambiguous_reasons", "has_staged_changes", "clean", "entries", "entry_count", "entries_truncated", "entry_limit", "redacted_secret_path_count"},
		map[string]any{
			"result":                     map[string]any{"const": "ok"},
			"repo":                       stringSchema(),
			"branch":                     nullableStringSchema(),
			"is_detached":                boolSchema(),
			"ambiguous_reasons":          arraySchema(stringSchema()),
			"has_staged_changes":         boolSchema(),
			"clean":                      boolSchema(),
			"entries":                    arraySchema(statusEntrySchema()),
			"entry_count":                intSchema(),
			"entries_truncated":          boolSchema(),
			"entry_limit":                intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func diffSummaryOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "unstaged", "staged", "untracked"},
		map[string]any{
			"result":    map[string]any{"const": "ok"},
			"repo":      stringSchema(),
			"unstaged":  fileSummarySchema(),
			"staged":    fileSummarySchema(),
			"untracked": untrackedSummarySchema(),
		},
	)
}

func listLocalBranchesOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "branches", "branch_count", "branches_truncated", "branch_limit"},
		map[string]any{
			"result":             map[string]any{"const": "ok"},
			"repo":               stringSchema(),
			"branches":           arraySchema(localBranchEntrySchema()),
			"branch_count":       intSchema(),
			"branches_truncated": boolSchema(),
			"branch_limit":       intSchema(),
		},
	)
}

func listLocalRefsOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "refs", "ref_count", "refs_truncated", "ref_limit"},
		map[string]any{
			"result":         map[string]any{"const": "ok"},
			"repo":           stringSchema(),
			"refs":           arraySchema(localRefEntrySchema()),
			"ref_count":      intSchema(),
			"refs_truncated": boolSchema(),
			"ref_limit":      intSchema(),
		},
	)
}

func mergeBaseOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "left_ref", "right_ref", "left_commit", "right_commit", "merge_base", "is_ancestor"},
		map[string]any{
			"result":       map[string]any{"const": "ok"},
			"repo":         stringSchema(),
			"left_ref":     stringSchema(),
			"right_ref":    stringSchema(),
			"left_commit":  stringSchema(),
			"right_commit": stringSchema(),
			"merge_base":   stringSchema(),
			"is_ancestor":  boolSchema(),
		},
	)
}

func compareRefsOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "base_ref", "target_ref", "base_commit", "target_commit", "merge_base", "ahead_count", "behind_count", "changed_file_count", "redacted_secret_path_count"},
		map[string]any{
			"result":                     map[string]any{"const": "ok"},
			"repo":                       stringSchema(),
			"base_ref":                   stringSchema(),
			"target_ref":                 stringSchema(),
			"base_commit":                stringSchema(),
			"target_commit":              stringSchema(),
			"merge_base":                 stringSchema(),
			"ahead_count":                intSchema(),
			"behind_count":               intSchema(),
			"changed_file_count":         intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func changedFilesBetweenRefsOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "base_ref", "target_ref", "base_commit", "target_commit", "changed"},
		map[string]any{
			"result":        map[string]any{"const": "ok"},
			"repo":          stringSchema(),
			"base_ref":      stringSchema(),
			"target_ref":    stringSchema(),
			"base_commit":   stringSchema(),
			"target_commit": stringSchema(),
			"changed":       pathListSummarySchema(),
		},
	)
}

func commitLogSummaryOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "ref", "resolved_commit", "commits", "commit_count", "commits_truncated", "commit_limit"},
		map[string]any{
			"result":            map[string]any{"const": "ok"},
			"repo":              stringSchema(),
			"ref":               stringSchema(),
			"resolved_commit":   stringSchema(),
			"commits":           arraySchema(commitSummaryEntrySchema()),
			"commit_count":      intSchema(),
			"commits_truncated": boolSchema(),
			"commit_limit":      intSchema(),
		},
	)
}

func showCommitSummaryOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "commit_ref", "commit"},
		map[string]any{
			"result":     map[string]any{"const": "ok"},
			"repo":       stringSchema(),
			"commit_ref": stringSchema(),
			"commit":     commitSummaryEntrySchema(),
		},
	)
}

func pathStatusOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "entries", "entry_count", "entries_truncated", "entry_limit", "redacted_secret_path_count"},
		map[string]any{
			"result":                     map[string]any{"const": "ok"},
			"repo":                       stringSchema(),
			"entries":                    arraySchema(pathStatusEntrySchema()),
			"entry_count":                intSchema(),
			"entries_truncated":          boolSchema(),
			"entry_limit":                intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func submoduleSummaryOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "submodules", "submodule_count", "submodules_truncated", "submodule_limit", "redacted_secret_path_count"},
		map[string]any{
			"result":                     map[string]any{"const": "ok"},
			"repo":                       stringSchema(),
			"submodules":                 arraySchema(submoduleEntrySchema()),
			"submodule_count":            intSchema(),
			"submodules_truncated":       boolSchema(),
			"submodule_limit":            intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func repositoryIntegrityCheckOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "integrity_ok", "exit_code", "issues", "issue_count", "issues_truncated", "issue_limit"},
		map[string]any{
			"result":           map[string]any{"const": "ok"},
			"repo":             stringSchema(),
			"integrity_ok":     boolSchema(),
			"exit_code":        signedIntSchema(),
			"issues":           arraySchema(integrityIssueCountSchema()),
			"issue_count":      intSchema(),
			"issues_truncated": boolSchema(),
			"issue_limit":      intSchema(),
		},
	)
}

func reflogSummaryOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "ref", "entries", "entry_count", "entries_truncated", "entry_limit", "redacted_sensitive_summary_count"},
		map[string]any{
			"result":                           map[string]any{"const": "ok"},
			"repo":                             stringSchema(),
			"ref":                              stringSchema(),
			"entries":                          arraySchema(reflogEntrySchema()),
			"entry_count":                      intSchema(),
			"entries_truncated":                boolSchema(),
			"entry_limit":                      intSchema(),
			"redacted_sensitive_summary_count": intSchema(),
		},
	)
}

func selfCheckOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "server_version", "protocol_version", "git_version", "tool_names", "tool_count", "binary_path_hash", "binary_checksum", "checksum_status", "audit_log_configured", "audit_log_writable", "allowed_repo_count", "allowed_repo_root_count", "allowlist_fingerprints", "protected_branch_count", "redacted_config_path_count"},
		map[string]any{
			"result":                     map[string]any{"const": "ok"},
			"repo":                       stringSchema(),
			"server_version":             stringSchema(),
			"protocol_version":           stringSchema(),
			"git_version":                stringSchema(),
			"tool_names":                 arraySchema(stringSchema()),
			"tool_count":                 intSchema(),
			"binary_path_hash":           stringSchema(),
			"binary_checksum":            stringSchema(),
			"checksum_status":            stringSchema(),
			"audit_log_configured":       boolSchema(),
			"audit_log_writable":         boolSchema(),
			"allowed_repo_count":         intSchema(),
			"allowed_repo_root_count":    intSchema(),
			"allowlist_fingerprints":     arraySchema(stringSchema()),
			"protected_branch_count":     intSchema(),
			"redacted_config_path_count": intSchema(),
		},
	)
}

func commitOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "commit_hash", "files", "audit_summary"},
		map[string]any{
			"result":        map[string]any{"const": "committed"},
			"repo":          stringSchema(),
			"commit_hash":   stringSchema(),
			"files":         arraySchema(stringSchema()),
			"audit_summary": stringSchema(),
		},
	)
}

func branchOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "branch", "action", "head_commit"},
		map[string]any{
			"result":      map[string]any{"const": "ok"},
			"repo":        stringSchema(),
			"branch":      stringSchema(),
			"action":      stringSchema(),
			"head_commit": stringSchema(),
		},
	)
}

func mergeOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "source_branch", "target_branch", "action", "source_head", "target_head_before", "target_head_after"},
		map[string]any{
			"result":             map[string]any{"const": "ok"},
			"repo":               stringSchema(),
			"source_branch":      stringSchema(),
			"target_branch":      stringSchema(),
			"action":             stringSchema(),
			"source_head":        stringSchema(),
			"target_head_before": stringSchema(),
			"target_head_after":  stringSchema(),
		},
	)
}

func listWorktreesOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "worktrees", "worktree_count", "worktrees_truncated", "worktree_limit", "redacted_unallowlisted_count"},
		map[string]any{
			"result":                       map[string]any{"const": "ok"},
			"repo":                         stringSchema(),
			"worktrees":                    arraySchema(worktreeEntrySchema()),
			"worktree_count":               intSchema(),
			"worktrees_truncated":          boolSchema(),
			"worktree_limit":               intSchema(),
			"redacted_unallowlisted_count": intSchema(),
		},
	)
}

func createWorktreeOutputSchema() map[string]any {
	return objectSchema(
		[]string{"result", "repo", "worktree_path", "branch", "base_branch", "base_head", "head_commit", "action"},
		map[string]any{
			"result":        map[string]any{"const": "ok"},
			"repo":          stringSchema(),
			"worktree_path": stringSchema(),
			"branch":        stringSchema(),
			"base_branch":   nullableStringSchema(),
			"base_head":     stringSchema(),
			"head_commit":   stringSchema(),
			"action":        stringSchema(),
		},
	)
}

func objectSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{"type": "object", "required": required, "properties": properties, "additionalProperties": false}
}

func statusEntrySchema() map[string]any {
	return objectSchema([]string{"code", "path"}, map[string]any{"code": stringSchema(), "path": stringSchema()})
}

func fileSummarySchema() map[string]any {
	return objectSchema(
		[]string{"files", "file_count", "files_truncated", "file_limit", "redacted_secret_path_count"},
		map[string]any{
			"files":                      arraySchema(fileStatSchema()),
			"file_count":                 intSchema(),
			"files_truncated":            boolSchema(),
			"file_limit":                 intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func fileStatSchema() map[string]any {
	return objectSchema([]string{"path", "additions", "deletions"}, map[string]any{"path": stringSchema(), "additions": nullableIntSchema(), "deletions": nullableIntSchema()})
}

func untrackedSummarySchema() map[string]any {
	return objectSchema(
		[]string{"files", "file_count", "files_truncated", "file_limit", "redacted_secret_path_count"},
		map[string]any{
			"files":                      arraySchema(stringSchema()),
			"file_count":                 intSchema(),
			"files_truncated":            boolSchema(),
			"file_limit":                 intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func worktreeEntrySchema() map[string]any {
	return objectSchema(
		[]string{"path", "head", "branch", "is_current", "is_detached", "is_bare", "is_locked", "is_prunable"},
		map[string]any{
			"path":        stringSchema(),
			"head":        stringSchema(),
			"branch":      nullableStringSchema(),
			"is_current":  boolSchema(),
			"is_detached": boolSchema(),
			"is_bare":     boolSchema(),
			"is_locked":   boolSchema(),
			"is_prunable": boolSchema(),
		},
	)
}

func localBranchEntrySchema() map[string]any {
	return objectSchema(
		[]string{"name", "head", "is_current", "is_protected", "is_checked_out"},
		map[string]any{
			"name":           stringSchema(),
			"head":           stringSchema(),
			"is_current":     boolSchema(),
			"is_protected":   boolSchema(),
			"is_checked_out": boolSchema(),
		},
	)
}

func localRefEntrySchema() map[string]any {
	return objectSchema(
		[]string{"ref", "short_name", "kind", "target_hash", "target_type", "is_protected", "is_visible"},
		map[string]any{
			"ref":          stringSchema(),
			"short_name":   stringSchema(),
			"kind":         stringSchema(),
			"target_hash":  stringSchema(),
			"target_type":  stringSchema(),
			"is_protected": boolSchema(),
			"is_visible":   boolSchema(),
		},
	)
}

func pathListSummarySchema() map[string]any {
	return objectSchema(
		[]string{"files", "file_count", "files_truncated", "file_limit", "redacted_secret_path_count"},
		map[string]any{
			"files":                      arraySchema(stringSchema()),
			"file_count":                 intSchema(),
			"files_truncated":            boolSchema(),
			"file_limit":                 intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func commitSummaryEntrySchema() map[string]any {
	return objectSchema(
		[]string{"hash", "subject", "author_date", "changed_files", "changed_file_count", "changed_files_truncated", "changed_file_limit", "redacted_secret_path_count"},
		map[string]any{
			"hash":                       stringSchema(),
			"subject":                    stringSchema(),
			"author_date":                stringSchema(),
			"changed_files":              arraySchema(stringSchema()),
			"changed_file_count":         intSchema(),
			"changed_files_truncated":    boolSchema(),
			"changed_file_limit":         intSchema(),
			"redacted_secret_path_count": intSchema(),
		},
	)
}

func pathStatusEntrySchema() map[string]any {
	return objectSchema(
		[]string{"path", "exists", "is_dir", "is_symlink", "tracked", "ignored", "ignore_source", "untracked", "modified", "deleted", "staged", "unmerged", "sparse", "skip_worktree", "has_symlink_components"},
		map[string]any{
			"path":                   stringSchema(),
			"exists":                 boolSchema(),
			"is_dir":                 boolSchema(),
			"is_symlink":             boolSchema(),
			"tracked":                boolSchema(),
			"ignored":                boolSchema(),
			"ignore_source":          nullableObjectSchema(ignoreSourceSchema()),
			"untracked":              boolSchema(),
			"modified":               boolSchema(),
			"deleted":                boolSchema(),
			"staged":                 boolSchema(),
			"unmerged":               boolSchema(),
			"sparse":                 boolSchema(),
			"skip_worktree":          boolSchema(),
			"has_symlink_components": boolSchema(),
		},
	)
}

func ignoreSourceSchema() map[string]any {
	return objectSchema(
		[]string{"source", "line", "pattern"},
		map[string]any{"source": stringSchema(), "line": intSchema(), "pattern": stringSchema()},
	)
}

func submoduleEntrySchema() map[string]any {
	return objectSchema(
		[]string{"path", "head", "status", "status_code", "dirty", "uninitialised", "out_of_sync", "conflicted"},
		map[string]any{
			"path":          stringSchema(),
			"head":          stringSchema(),
			"status":        stringSchema(),
			"status_code":   stringSchema(),
			"dirty":         boolSchema(),
			"uninitialised": boolSchema(),
			"out_of_sync":   boolSchema(),
			"conflicted":    boolSchema(),
		},
	)
}

func integrityIssueCountSchema() map[string]any {
	return objectSchema(
		[]string{"code", "severity", "count"},
		map[string]any{"code": stringSchema(), "severity": stringSchema(), "count": intSchema()},
	)
}

func reflogEntrySchema() map[string]any {
	return objectSchema(
		[]string{"selector", "commit", "summary", "date"},
		map[string]any{"selector": stringSchema(), "commit": stringSchema(), "summary": stringSchema(), "date": stringSchema()},
	)
}

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func nullableStringSchema() map[string]any {
	return map[string]any{"type": []string{"string", "null"}}
}

func nullableObjectSchema(object map[string]any) map[string]any {
	return map[string]any{"anyOf": []map[string]any{object, map[string]any{"type": "null"}}}
}

func intSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 0}
}

func signedIntSchema() map[string]any {
	return map[string]any{"type": "integer"}
}

func nullableIntSchema() map[string]any {
	return map[string]any{"type": []string{"integer", "null"}, "minimum": 0}
}

func boolSchema() map[string]any {
	return map[string]any{"type": "boolean"}
}
