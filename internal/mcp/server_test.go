package mcp_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/K1-R1/codex-safe-git/internal/audit"
	"github.com/K1-R1/codex-safe-git/internal/config"
	"github.com/K1-R1/codex-safe-git/internal/gitexec"
	"github.com/K1-R1/codex-safe-git/internal/gitpolicy"
	"github.com/K1-R1/codex-safe-git/internal/mcp"
	"github.com/K1-R1/codex-safe-git/internal/testrepo"
)

func TestToolSurfaceMatchesGoldenNames(t *testing.T) {
	resp := mcp.Server{}.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if resp == nil {
		t.Fatal("expected response")
	}

	var raw struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	marshalRoundTrip(t, resp, &raw)

	var got []string
	for _, tool := range raw.Result.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)

	goldenPath := filepath.Join("..", "..", "testdata", "golden", "tool-names.json")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	sort.Strings(want)

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("tool names mismatch: got %#v want %#v", got, want)
	}
}

func TestToolDefinitionsExposeAnnotationsAndOutputSchemas(t *testing.T) {
	resp := mcp.Server{}.Handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if resp == nil {
		t.Fatal("expected response")
	}

	var raw struct {
		Result struct {
			Tools []struct {
				Name         string         `json:"name"`
				InputSchema  map[string]any `json:"inputSchema"`
				Annotations  map[string]any `json:"annotations"`
				OutputSchema map[string]any `json:"outputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	marshalRoundTrip(t, resp, &raw)

	expectedRequired := map[string][]string{
		"git_status":                 {"repo_path"},
		"git_diff_summary":           {"repo_path"},
		"list_local_branches":        {"repo_path"},
		"compare_refs":               {"repo_path", "base_ref", "target_ref"},
		"commit_log_summary":         {"repo_path"},
		"show_commit_summary":        {"repo_path", "commit_ref"},
		"list_local_refs":            {"repo_path"},
		"merge_base":                 {"repo_path", "left_ref", "right_ref"},
		"changed_files_between_refs": {"repo_path", "base_ref", "target_ref"},
		"path_status":                {"repo_path", "paths"},
		"submodule_summary":          {"repo_path"},
		"repository_integrity_check": {"repo_path"},
		"reflog_summary":             {"repo_path"},
		"self_check":                 {"repo_path"},
		"commit_files":               {"repo_path", "files", "message"},
		"ensure_commit_branch":       {"repo_path", "branch_name"},
		"create_commit_branch":       {"repo_path", "branch_name"},
		"merge_branch":               {"repo_path", "source_branch"},
		"list_worktrees":             {"repo_path"},
		"create_worktree":            {"repo_path", "worktree_path", "branch_name"},
		"safe_checkout":              {"repo_path", "branch_name"},
	}
	readOnlyTools := map[string]struct{}{
		"git_status": {}, "git_diff_summary": {}, "list_local_branches": {}, "compare_refs": {},
		"commit_log_summary": {}, "show_commit_summary": {}, "list_local_refs": {}, "merge_base": {},
		"changed_files_between_refs": {}, "path_status": {}, "submodule_summary": {},
		"repository_integrity_check": {}, "reflog_summary": {}, "self_check": {}, "list_worktrees": {},
	}

	seen := map[string]struct{}{}
	for _, tool := range raw.Result.Tools {
		seen[tool.Name] = struct{}{}
		if tool.InputSchema["type"] != "object" || tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("%s has unsafe input schema: %#v", tool.Name, tool.InputSchema)
		}
		required, ok := tool.InputSchema["required"].([]any)
		if !ok {
			t.Fatalf("%s missing required inputs: %#v", tool.Name, tool.InputSchema)
		}
		if !sameStringMembers(anyStrings(required), expectedRequired[tool.Name]) {
			t.Fatalf("%s required inputs mismatch: got %#v want %#v", tool.Name, anyStrings(required), expectedRequired[tool.Name])
		}
		if tool.OutputSchema["type"] != "object" {
			t.Fatalf("%s missing object outputSchema: %#v", tool.Name, tool.OutputSchema)
		}
		if _, ok := tool.OutputSchema["required"].([]any); !ok {
			t.Fatalf("%s missing outputSchema required fields: %#v", tool.Name, tool.OutputSchema)
		}
		if tool.Annotations["openWorldHint"] != false || tool.Annotations["destructiveHint"] != false {
			t.Fatalf("%s has unsafe annotations: %#v", tool.Name, tool.Annotations)
		}
		_, readOnly := readOnlyTools[tool.Name]
		if tool.Annotations["readOnlyHint"] != readOnly {
			t.Fatalf("%s readOnlyHint mismatch: %#v", tool.Name, tool.Annotations)
		}
		if tool.Name == "commit_files" {
			if tool.Annotations["readOnlyHint"] != false {
				t.Fatalf("commit_files should be mutating: %#v", tool.Annotations)
			}
			properties, _ := tool.InputSchema["properties"].(map[string]any)
			files, _ := properties["files"].(map[string]any)
			if int(files["maxItems"].(float64)) != gitpolicy.CommitFileLimit {
				t.Fatalf("commit_files maxItems does not match policy limit: %#v", files)
			}
			message, _ := properties["message"].(map[string]any)
			if int(message["maxLength"].(float64)) != gitpolicy.CommitMessageSubjectLimit {
				t.Fatalf("commit_files message maxLength does not match policy limit: %#v", message)
			}
		}
	}
	if len(seen) != len(expectedRequired) {
		t.Fatalf("tool count mismatch: got %d want %d", len(seen), len(expectedRequired))
	}
	for name := range expectedRequired {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing tool %s", name)
		}
	}
}

func TestToolCallReturnsStructuredContent(t *testing.T) {
	repo := testrepo.New(t)
	server := mcp.Server{Policy: &repo.Policy}

	req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":` + quote(repo.Path) + `}}}`
	resp := server.Handle([]byte(req))
	var raw toolCallResponse
	marshalRoundTrip(t, resp, &raw)

	if raw.Result.IsError {
		t.Fatalf("unexpected error payload: %#v", raw.Result.StructuredContent)
	}
	if got := raw.Result.StructuredContent["result"]; got != "ok" {
		t.Fatalf("unexpected result: %#v", raw.Result.StructuredContent)
	}
	if got := raw.Result.StructuredContent["branch"]; got != "work" {
		t.Fatalf("unexpected branch: %#v", raw.Result.StructuredContent)
	}
	if len(raw.Result.Content) != 1 || raw.Result.Content[0].Type != "text" {
		t.Fatalf("missing text content block: %#v", raw.Result.Content)
	}
}

func TestToolCallRejectsUnexpectedArguments(t *testing.T) {
	repo := testrepo.New(t)
	server := mcp.Server{Policy: &repo.Policy}

	req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":` + quote(repo.Path) + `,"command":"status"}}}`
	resp := server.Handle([]byte(req))
	var raw toolCallResponse
	marshalRoundTrip(t, resp, &raw)

	if !raw.Result.IsError {
		t.Fatalf("expected refusal payload: %#v", raw.Result)
	}
	reason, _ := raw.Result.StructuredContent["reason"].(string)
	if !strings.Contains(reason, "unexpected tool arguments") {
		t.Fatalf("unexpected refusal reason: %q", reason)
	}
}

func TestWorktreeToolCallsReturnStructuredContent(t *testing.T) {
	repo := testrepo.New(t)
	policy := rootAllowedPolicy(t, repo)
	server := mcp.Server{Policy: &policy}
	target := filepath.Join(repo.Root, "mcp-worktree")

	createReq := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"create_worktree","arguments":{"repo_path":` + quote(repo.Path) + `,"worktree_path":` + quote(target) + `,"branch_name":"codex/mcp-worktree"}}}`
	createResp := server.Handle([]byte(createReq))
	var createRaw toolCallResponse
	marshalRoundTrip(t, createResp, &createRaw)
	if createRaw.Result.IsError {
		t.Fatalf("unexpected create_worktree refusal: %#v", createRaw.Result.StructuredContent)
	}
	if got := createRaw.Result.StructuredContent["worktree_path"]; got != canonicalPath(t, target) {
		t.Fatalf("unexpected worktree path: %#v", got)
	}

	listReq := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"list_worktrees","arguments":{"repo_path":` + quote(repo.Path) + `}}}`
	listResp := server.Handle([]byte(listReq))
	var listRaw toolCallResponse
	marshalRoundTrip(t, listResp, &listRaw)
	if listRaw.Result.IsError {
		t.Fatalf("unexpected list_worktrees refusal: %#v", listRaw.Result.StructuredContent)
	}
	if got := listRaw.Result.StructuredContent["result"]; got != "ok" {
		t.Fatalf("unexpected list_worktrees result: %#v", listRaw.Result.StructuredContent)
	}
}

func TestRefusalPayloadMatchesGolden(t *testing.T) {
	repo := testrepo.New(t)
	server := mcp.Server{Policy: &repo.Policy}

	req := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":` + quote(filepath.Join(repo.Root, "outside")) + `}}}`
	resp := server.Handle([]byte(req))
	var raw toolCallResponse
	marshalRoundTrip(t, resp, &raw)

	goldenPath := filepath.Join("..", "..", "testdata", "golden", "refusal.json")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !raw.Result.IsError {
		t.Fatalf("expected refusal payload: %#v", raw.Result)
	}
	if got, wantText := raw.Result.StructuredContent["reason"], want["reason"]; got != wantText {
		t.Fatalf("unexpected refusal: got %#v want %#v", got, wantText)
	}
}

func TestToolResultSchemaCoversCurrentResultShapes(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "docs", "schemas", "tool-results.schema.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		OneOf []struct {
			Required []string `json:"required"`
		} `json:"oneOf"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, required := range [][]string{
		{"result", "repo", "branch", "is_detached", "ambiguous_reasons", "has_staged_changes", "clean", "entries", "entry_count", "entries_truncated", "entry_limit", "redacted_secret_path_count"},
		{"result", "repo", "unstaged", "staged", "untracked"},
		{"result", "repo", "branches", "branch_count", "branches_truncated", "branch_limit"},
		{"result", "repo", "base_ref", "target_ref", "base_commit", "target_commit", "merge_base", "ahead_count", "behind_count", "changed_file_count", "redacted_secret_path_count"},
		{"result", "repo", "ref", "resolved_commit", "commits", "commit_count", "commits_truncated", "commit_limit"},
		{"result", "repo", "commit_ref", "commit"},
		{"result", "repo", "refs", "ref_count", "refs_truncated", "ref_limit"},
		{"result", "repo", "left_ref", "right_ref", "left_commit", "right_commit", "merge_base", "is_ancestor"},
		{"result", "repo", "base_ref", "target_ref", "base_commit", "target_commit", "changed"},
		{"result", "repo", "entries", "entry_count", "entries_truncated", "entry_limit", "redacted_secret_path_count"},
		{"result", "repo", "submodules", "submodule_count", "submodules_truncated", "submodule_limit", "redacted_secret_path_count"},
		{"result", "repo", "integrity_ok", "exit_code", "issues", "issue_count", "issues_truncated", "issue_limit"},
		{"result", "repo", "ref", "entries", "entry_count", "entries_truncated", "entry_limit", "redacted_sensitive_summary_count"},
		{"result", "repo", "server_version", "protocol_version", "git_version", "tool_names", "tool_count", "binary_path_hash", "binary_checksum", "checksum_status", "audit_log_configured", "audit_log_writable", "allowed_repo_count", "allowed_repo_root_count", "allowlist_fingerprints", "protected_branch_count", "redacted_config_path_count"},
		{"result", "repo", "commit_hash", "files", "audit_summary"},
		{"result", "repo", "branch", "action", "head_commit"},
		{"result", "repo", "source_branch", "target_branch", "action", "source_head", "target_head_before", "target_head_after"},
		{"result", "repo", "worktrees", "worktree_count", "worktrees_truncated", "worktree_limit", "redacted_unallowlisted_count"},
		{"result", "repo", "worktree_path", "branch", "base_branch", "base_head", "head_commit", "action"},
		{"result", "reason"},
	} {
		if !schemaHasRequired(schema.OneOf, required) {
			t.Fatalf("schema missing required shape: %#v", required)
		}
	}
}

func TestStdioServerRoundTrip(t *testing.T) {
	repo := testrepo.New(t)
	t.Setenv("CODEX_SAFE_GIT_ALLOWED_REPOS", repo.Path)
	t.Setenv("CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS", "")
	t.Setenv("CODEX_SAFE_GIT_AUDIT_LOG", repo.AuditLog)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":` + quote(repo.Path) + `}}}`,
	}, "\n") + "\n"

	var output bytes.Buffer
	if code := mcp.Main(strings.NewReader(input), &output); code != 0 {
		t.Fatalf("server returned non-zero exit code %d", code)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two responses, got %d: %s", len(lines), output.String())
	}
	var status toolCallResponse
	if err := json.Unmarshal([]byte(lines[1]), &status); err != nil {
		t.Fatal(err)
	}
	if got := status.Result.StructuredContent["result"]; got != "ok" {
		t.Fatalf("unexpected status response: %#v", status.Result.StructuredContent)
	}
}

func TestStdioServerRejectsOversizedAndMalformedRequestsThenRecovers(t *testing.T) {
	repo := testrepo.New(t)
	t.Setenv("CODEX_SAFE_GIT_ALLOWED_REPOS", repo.Path)
	t.Setenv("CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS", "")
	t.Setenv("CODEX_SAFE_GIT_AUDIT_LOG", repo.AuditLog)

	valid := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"git_status","arguments":{"repo_path":` + quote(repo.Path) + `}}}`
	input := strings.Repeat("x", 2*1024*1024) + "\n" + "{bad json}\n" + valid + "\n"

	var output bytes.Buffer
	if code := mcp.Main(strings.NewReader(input), &output); code != 0 {
		t.Fatalf("server returned non-zero exit code %d", code)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected three responses, got %d: %s", len(lines), output.String())
	}
	var oversized, malformed responseEnvelope
	if err := json.Unmarshal([]byte(lines[0]), &oversized); err != nil {
		t.Fatal(err)
	}
	if oversized.Error == nil || !strings.Contains(oversized.Error.Message, "maximum size") {
		t.Fatalf("expected oversized parse error, got %#v", oversized)
	}
	if err := json.Unmarshal([]byte(lines[1]), &malformed); err != nil {
		t.Fatal(err)
	}
	if malformed.Error == nil || malformed.Error.Code != -32700 {
		t.Fatalf("expected malformed parse error, got %#v", malformed)
	}
	var status toolCallResponse
	if err := json.Unmarshal([]byte(lines[2]), &status); err != nil {
		t.Fatal(err)
	}
	if status.Result.IsError || status.Result.StructuredContent["result"] != "ok" {
		t.Fatalf("expected recovery status response, got %#v", status.Result)
	}
}

func TestInstallerRefusesCanonicalParentInstallPath(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), ".codex")
	if err := os.MkdirAll(filepath.Join(codexHome, "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join("..", "..", "scripts", "install-local.sh")
	cmd := exec.Command("bash", script, "--dry-run")
	cmd.Env = append(os.Environ(),
		"CODEX_HOME="+codexHome,
		"CODEX_SAFE_GIT_INSTALL_DIR="+filepath.Join(codexHome, "tools", ".."),
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected installer refusal, got success:\n%s", output)
	}
	if !strings.Contains(string(output), "Refusing unsafe install directory") {
		t.Fatalf("unexpected installer output:\n%s", output)
	}
}

func TestInstallerPrintConfigEscapesTomlStrings(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), `codex "home\dir`)
	script := filepath.Join("..", "..", "scripts", "install-local.sh")
	cmd := exec.Command("bash", script, "--print-config")
	cmd.Env = append(os.Environ(),
		"CODEX_HOME="+codexHome,
		`CODEX_SAFE_GIT_PROTECTED_BRANCHES=release/"quoted"`+"\tbranch",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("print config failed: %v\n%s", err, output)
	}
	text := string(output)
	for _, want := range []string{`codex \"home\\dir`, `release/\"quoted\"\tbranch`} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected escaped TOML fragment %q in:\n%s", want, text)
		}
	}
}

type toolCallResponse struct {
	Result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StructuredContent map[string]any `json:"structuredContent"`
		IsError           bool           `json:"isError"`
	} `json:"result"`
}

type responseEnvelope struct {
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func anyStrings(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func sameStringMembers(left, right []string) bool {
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	return strings.Join(leftCopy, "\x00") == strings.Join(rightCopy, "\x00")
}

func marshalRoundTrip(t *testing.T, in any, out any) {
	t.Helper()
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}

func quote(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func rootAllowedPolicy(t *testing.T, repo testrepo.Repo) gitpolicy.Policy {
	t.Helper()
	runner, err := gitexec.NewRunner()
	if err != nil {
		t.Fatal(err)
	}
	return gitpolicy.Policy{
		Config: config.Config{
			AllowedRepos:     map[string]struct{}{},
			AllowedRepoRoots: map[string]struct{}{canonicalPath(t, repo.Root): {}},
			AuditLog:         repo.AuditLog,
		},
		Git:   runner,
		Audit: audit.Logger{Path: repo.AuditLog},
	}
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(abs)
}

func schemaHasRequired(shapes []struct {
	Required []string `json:"required"`
}, required []string) bool {
	want := append([]string(nil), required...)
	sort.Strings(want)
	for _, shape := range shapes {
		got := append([]string(nil), shape.Required...)
		sort.Strings(got)
		if strings.Join(got, "\x00") == strings.Join(want, "\x00") {
			return true
		}
	}
	return false
}
