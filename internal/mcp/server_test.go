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

	"local/codex-safe-git/internal/audit"
	"local/codex-safe-git/internal/config"
	"local/codex-safe-git/internal/gitexec"
	"local/codex-safe-git/internal/gitpolicy"
	"local/codex-safe-git/internal/mcp"
	"local/codex-safe-git/internal/testrepo"
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

	seen := map[string]struct{}{}
	for _, tool := range raw.Result.Tools {
		seen[tool.Name] = struct{}{}
		if tool.OutputSchema["type"] != "object" {
			t.Fatalf("%s missing object outputSchema: %#v", tool.Name, tool.OutputSchema)
		}
		if tool.Annotations["openWorldHint"] != false || tool.Annotations["destructiveHint"] != false {
			t.Fatalf("%s has unsafe annotations: %#v", tool.Name, tool.Annotations)
		}
		if tool.Name == "git_status" && tool.Annotations["readOnlyHint"] != true {
			t.Fatalf("git_status should be read-only: %#v", tool.Annotations)
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
		}
	}
	for _, name := range []string{"git_status", "git_diff_summary", "commit_files", "list_worktrees"} {
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
