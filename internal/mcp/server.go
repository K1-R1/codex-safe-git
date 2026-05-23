package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/K1-R1/codex-safe-git/internal/config"
	"github.com/K1-R1/codex-safe-git/internal/gitpolicy"
)

const ProtocolVersion = "2025-11-25"
const ServerVersion = "0.4.2"
const maxJSONRPCLineBytes = 1024 * 1024

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

type compareRefsArgs struct {
	RepoPath  string `json:"repo_path"`
	BaseRef   string `json:"base_ref,omitempty"`
	TargetRef string `json:"target_ref,omitempty"`
}

type mergeBaseArgs struct {
	RepoPath string `json:"repo_path"`
	LeftRef  string `json:"left_ref,omitempty"`
	RightRef string `json:"right_ref,omitempty"`
}

type commitLogSummaryArgs struct {
	RepoPath string  `json:"repo_path"`
	Ref      *string `json:"ref,omitempty"`
	Limit    *int    `json:"limit,omitempty"`
}

type showCommitSummaryArgs struct {
	RepoPath  string `json:"repo_path"`
	CommitRef string `json:"commit_ref"`
}

type pathStatusArgs struct {
	RepoPath            string   `json:"repo_path"`
	Paths               []string `json:"paths"`
	IncludeIgnoreSource *bool    `json:"include_ignore_source,omitempty"`
}

type reflogSummaryArgs struct {
	RepoPath string  `json:"repo_path"`
	Ref      *string `json:"ref,omitempty"`
	Limit    *int    `json:"limit,omitempty"`
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
	reader := bufio.NewReaderSize(stdin, 64*1024)
	writer := bufio.NewWriter(stdout)
	defer writer.Flush()
	for {
		line, tooLarge, err := readBoundedLine(reader, maxJSONRPCLineBytes)
		if tooLarge {
			writeResponse(writer, &response{JSONRPC: "2.0", ID: nil, Error: &errorObject{Code: -32700, Message: "Parse error: request exceeds maximum size"}})
			if err == io.EOF {
				return 0
			}
			if err != nil {
				return 1
			}
			continue
		}
		if err == io.EOF && len(line) == 0 {
			return 0
		}
		if err != nil && err != io.EOF {
			return 1
		}
		if len(bytes.TrimSpace(line)) == 0 {
			if err == io.EOF {
				return 0
			}
			continue
		}
		reply := server.Handle(line)
		if reply != nil {
			writeResponse(writer, reply)
		}
		if err == io.EOF {
			return 0
		}
	}
}

func readBoundedLine(reader *bufio.Reader, limit int) ([]byte, bool, error) {
	var line []byte
	for {
		chunk, err := reader.ReadSlice('\n')
		if len(line)+len(chunk) > limit {
			drainErr := drainLine(reader, err)
			return nil, true, drainErr
		}
		line = append(line, chunk...)
		switch err {
		case nil:
			return line, false, nil
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			return line, false, io.EOF
		default:
			return nil, false, err
		}
	}
}

func drainLine(reader *bufio.Reader, currentErr error) error {
	if currentErr == nil || currentErr == io.EOF {
		return currentErr
	}
	if currentErr != bufio.ErrBufferFull {
		return currentErr
	}
	for {
		_, err := reader.ReadSlice('\n')
		switch err {
		case nil:
			return nil
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			return io.EOF
		default:
			return err
		}
	}
}

func writeResponse(writer *bufio.Writer, reply *response) {
	encoded, err := json.Marshal(reply)
	if err != nil {
		return
	}
	_, _ = writer.Write(append(encoded, '\n'))
	_ = writer.Flush()
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
			"instructions": "Use only the exposed codex_safe_git tools. Repos must be explicitly allowlisted by environment or be exact Git worktree roots under an explicit allowed repo root. Read-only tools return bounded summaries; mutating tools remain local, audited, and narrowly scoped.",
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
	case "list_local_branches":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.ListLocalBranches(args.RepoPath)
	case "compare_refs":
		var args compareRefsArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.CompareRefs(args.RepoPath, args.BaseRef, args.TargetRef)
	case "commit_log_summary":
		var args commitLogSummaryArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.CommitLogSummary(args.RepoPath, args.Ref, args.Limit)
	case "show_commit_summary":
		var args showCommitSummaryArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.ShowCommitSummary(args.RepoPath, args.CommitRef)
	case "list_local_refs":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.ListLocalRefs(args.RepoPath)
	case "merge_base":
		var args mergeBaseArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.MergeBase(args.RepoPath, args.LeftRef, args.RightRef)
	case "changed_files_between_refs":
		var args compareRefsArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.ChangedFilesBetweenRefs(args.RepoPath, args.BaseRef, args.TargetRef)
	case "path_status":
		var args pathStatusArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.PathStatus(args.RepoPath, args.Paths, args.IncludeIgnoreSource)
	case "submodule_summary":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.SubmoduleSummary(args.RepoPath)
	case "repository_integrity_check":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.RepositoryIntegrityCheck(args.RepoPath)
	case "reflog_summary":
		var args reflogSummaryArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.ReflogSummary(args.RepoPath, args.Ref, args.Limit)
	case "self_check":
		var args repoPathArgs
		if err := decodeExact(params.Arguments, &args); err != nil {
			return refusalPayload(err.Error())
		}
		payload, err = policy.SelfCheck(args.RepoPath, ServerVersion, ProtocolVersion, ToolNames())
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
