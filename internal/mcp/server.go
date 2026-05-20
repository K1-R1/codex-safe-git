package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"local/codex-safe-git/internal/config"
	"local/codex-safe-git/internal/gitpolicy"
)

const ProtocolVersion = "2025-11-25"
const ServerVersion = "0.3.0"

type Server struct {
	Policy *gitpolicy.Policy
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id"`
	Result  any          `json:"result,omitempty"`
	Error   *errorObject `json:"error,omitempty"`
}

type errorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolPayload struct {
	Content           []contentBlock `json:"content"`
	StructuredContent any            `json:"structuredContent"`
	IsError           bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type repoPathArgs struct {
	RepoPath string `json:"repo_path"`
}

type commitFilesArgs struct {
	RepoPath string   `json:"repo_path"`
	Files    []string `json:"files"`
	Message  string   `json:"message"`
	Body     *string  `json:"body,omitempty"`
}

type branchArgs struct {
	RepoPath   string `json:"repo_path"`
	BranchName string `json:"branch_name"`
}

type mergeArgs struct {
	RepoPath     string  `json:"repo_path"`
	SourceBranch string  `json:"source_branch"`
	TargetBranch *string `json:"target_branch,omitempty"`
}

type createWorktreeArgs struct {
	RepoPath     string  `json:"repo_path"`
	WorktreePath string  `json:"worktree_path"`
	BranchName   string  `json:"branch_name"`
	BaseBranch   *string `json:"base_branch,omitempty"`
}

func Main(stdin io.Reader, stdout io.Writer) int {
	server := Server{}
	scanner := bufio.NewScanner(stdin)
	writer := bufio.NewWriter(stdout)
	defer writer.Flush()
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		reply := server.Handle(line)
		if reply == nil {
			continue
		}
		encoded, err := json.Marshal(reply)
		if err != nil {
			continue
		}
		_, _ = writer.Write(append(encoded, '\n'))
		_ = writer.Flush()
	}
	if scanner.Err() != nil {
		return 1
	}
	return 0
}

func (s Server) Handle(raw []byte) *response {
	var req request
	if err := json.Unmarshal(raw, &req); err != nil {
		return &response{JSONRPC: "2.0", ID: nil, Error: &errorObject{Code: -32700, Message: "Parse error"}}
	}
	if req.Method == "notifications/initialized" {
		return nil
	}
	switch req.Method {
	case "initialize":
		return result(req.ID, map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "codex-safe-git",
				"title":   "Codex Safe Git",
				"version": ServerVersion,
			},
			"instructions": "Use only git_status, git_diff_summary, commit_files, ensure_commit_branch, create_commit_branch, merge_branch, list_worktrees, create_worktree, and safe_checkout. Repos must be explicitly allowlisted by environment or be exact Git worktree roots under an explicit allowed repo root.",
		})
	case "tools/list":
		return result(req.ID, map[string]any{"tools": tools()})
	case "tools/call":
		payload := s.callTool(req.Params)
		return result(req.ID, payload)
	default:
		return &response{JSONRPC: "2.0", ID: req.ID, Error: &errorObject{Code: -32601, Message: "Unsupported method: " + req.Method}}
	}
}

func (s Server) callTool(raw json.RawMessage) toolPayload {
	var params toolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return refusalPayload("tool params must be an object")
	}
	if len(params.Arguments) == 0 {
		params.Arguments = []byte(`{}`)
	}
	policy, err := s.policy()
	if err != nil {
		return refusalPayload(err.Error())
	}
	var payload any
	switch params.Name {
	case "git_status":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.GitStatus(args.RepoPath)
	case "git_diff_summary":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.GitDiffSummary(args.RepoPath)
	case "commit_files":
		var args commitFilesArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.CommitFiles(args.RepoPath, args.Files, args.Message, args.Body)
	case "ensure_commit_branch":
		var args branchArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.EnsureCommitBranch(args.RepoPath, args.BranchName)
	case "create_commit_branch":
		var args branchArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.CreateCommitBranch(args.RepoPath, args.BranchName)
	case "merge_branch":
		var args mergeArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.MergeBranch(args.RepoPath, args.SourceBranch, args.TargetBranch)
	case "list_worktrees":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.ListWorktrees(args.RepoPath)
	case "create_worktree":
		var args createWorktreeArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.CreateWorktree(args.RepoPath, args.WorktreePath, args.BranchName, args.BaseBranch)
	case "safe_checkout":
		var args branchArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.SafeCheckout(args.RepoPath, args.BranchName)
	default:
		return refusalPayload("unsupported tool: " + params.Name)
	}
	if err != nil {
		return refusalPayload(err.Error())
	}
	return successPayload(payload)
}

func (s Server) policy() (gitpolicy.Policy, error) {
	if s.Policy != nil {
		return *s.Policy, nil
	}
	cfg, err := config.FromEnv(nil)
	if err != nil {
		return gitpolicy.Policy{}, err
	}
	return gitpolicy.New(cfg)
}

func decodeExact(raw json.RawMessage, target any) error {
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		return errors.New("tool arguments must be an object")
	}
	decoder := json.NewDecoder(bytesReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("unexpected tool arguments or invalid argument type: %w", err)
	}
	return nil
}

func bytesReader(raw []byte) io.Reader {
	return bytes.NewReader(raw)
}

func successPayload(payload any) toolPayload {
	return toolPayload{Content: []contentBlock{{Type: "text", Text: marshalText(payload)}}, StructuredContent: payload}
}

func refusalPayload(reason string) toolPayload {
	payload := gitpolicy.RefusalResult{Result: "refused", Reason: reason}
	return toolPayload{Content: []contentBlock{{Type: "text", Text: marshalText(payload)}}, StructuredContent: payload, IsError: true}
}

func marshalText(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{"reason":"json marshal failed","result":"refused"}`
	}
	return string(encoded)
}

func result(id any, payload any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: payload}
}

func tools() []map[string]any {
	return []map[string]any{
		tool("git_status", "Git Status", "Return a redacted, bounded, read-only status summary for an explicitly allowed local Git repo.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("Git Status"), statusOutputSchema()),
		tool("git_diff_summary", "Git Diff Summary", "Return redacted, bounded file-level diff counts for an explicitly allowed local Git repo.", []string{"repo_path"}, map[string]any{"repo_path": stringInput("Exact Git worktree root to inspect.")}, readOnlyAnnotations("Git Diff Summary"), diffSummaryOutputSchema()),
		tool("commit_files", "Commit Files", "Create a local commit from exactly listed files after deterministic safety checks.", []string{"repo_path", "files", "message"}, map[string]any{
			"repo_path": stringInput("Exact Git worktree root where the commit will be created."),
			"files":     map[string]any{"type": "array", "items": stringInput("Repo-relative file path."), "minItems": 1, "maxItems": gitpolicy.CommitFileLimit, "uniqueItems": true},
			"message":   stringInput("Commit subject without AI/tool attribution."),
			"body":      map[string]any{"type": "string", "description": "Optional commit body without AI/tool attribution."},
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

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func nullableStringSchema() map[string]any {
	return map[string]any{"type": []string{"string", "null"}}
}

func intSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 0}
}

func nullableIntSchema() map[string]any {
	return map[string]any{"type": []string{"integer", "null"}, "minimum": 0}
}

func boolSchema() map[string]any {
	return map[string]any{"type": "boolean"}
}
