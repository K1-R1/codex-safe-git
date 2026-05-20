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
