package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/K1-R1/codex-safe-git/internal/mcp"
)

func TestCLIPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, handled := handleArgsWithEnv([]string{"--version"}, &stdout, &stderr, "/tmp/codex-safe-git-mcp", nil, fakeHomeDir(t.TempDir()))
	if !handled || code != 0 {
		t.Fatalf("expected handled success, got handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "codex-safe-git "+mcp.ServerVersion {
		t.Fatalf("unexpected version output: %q", got)
	}
}

func TestCLIPrintConfigUsesExecutableAndDefaults(t *testing.T) {
	home := t.TempDir()
	exe := filepath.Join(home, "bin", "codex-safe-git-mcp")
	var stdout, stderr bytes.Buffer
	code, handled := handleArgsWithEnv([]string{"--print-config"}, &stdout, &stderr, exe, map[string]string{}, fakeHomeDir(home))
	if !handled || code != 0 {
		t.Fatalf("expected handled success, got handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{
		`[mcp_servers.codex_safe_git]`,
		`command = "` + filepath.ToSlash(exe),
		`"git_status"`,
		`CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "` + filepath.ToSlash(filepath.Join(home, ".codex", "worktrees")),
		`CODEX_SAFE_GIT_AUDIT_LOG = "` + filepath.ToSlash(filepath.Join(home, ".codex", "log", "codex-safe-git-audit.jsonl")),
	} {
		if !strings.Contains(filepath.ToSlash(text), want) {
			t.Fatalf("expected %q in config:\n%s", want, text)
		}
	}
}

func TestCLIPrintConfigHonoursOverrides(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{
		"CODEX_HOME":                        "~/custom-codex",
		"CODEX_SAFE_GIT_ALLOWED_REPOS":      "/repo/one:/repo/two",
		"CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS": "/roots",
		"CODEX_SAFE_GIT_AUDIT_LOG":          "/audit/log.jsonl",
		"CODEX_SAFE_GIT_PROTECTED_BRANCHES": `release/"quoted"`,
		"CODEX_SAFE_GIT_GIT_PATH":           "/usr/bin/git",
	}
	var stdout, stderr bytes.Buffer
	code, handled := handleArgsWithEnv([]string{"--print-config"}, &stdout, &stderr, "/tmp/codex-safe-git-mcp", env, fakeHomeDir(home))
	if !handled || code != 0 {
		t.Fatalf("expected handled success, got handled=%v code=%d stderr=%q", handled, code, stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{
		`CODEX_SAFE_GIT_ALLOWED_REPOS = "/repo/one:/repo/two"`,
		`CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = "/roots"`,
		`CODEX_SAFE_GIT_AUDIT_LOG = "/audit/log.jsonl"`,
		`CODEX_SAFE_GIT_PROTECTED_BRANCHES = "release/\"quoted\""`,
		`CODEX_SAFE_GIT_GIT_PATH = "/usr/bin/git"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in config:\n%s", want, text)
		}
	}
}

func TestCLIWithoutArgsStartsServer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, handled := handleArgsWithEnv(nil, &stdout, &stderr, "/tmp/codex-safe-git-mcp", nil, fakeHomeDir(t.TempDir()))
	if handled || code != 0 {
		t.Fatalf("expected unhandled no-arg invocation, got handled=%v code=%d", handled, code)
	}
}

func TestCLIRejectsUnknownOption(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, handled := handleArgsWithEnv([]string{"--wat"}, &stdout, &stderr, "/tmp/codex-safe-git-mcp", nil, fakeHomeDir(t.TempDir()))
	if !handled || code != 2 {
		t.Fatalf("expected usage error, got handled=%v code=%d", handled, code)
	}
	if !strings.Contains(stderr.String(), "unknown option") {
		t.Fatalf("expected unknown option error, got %q", stderr.String())
	}
}

func fakeHomeDir(home string) homeDirFunc {
	return func() (string, error) {
		return home, nil
	}
}
