package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"local/codex-safe-git/internal/config"
)

func TestFromEnvRequiresAllowlistAndAuditLog(t *testing.T) {
	if _, err := config.FromEnv(map[string]string{}); err == nil {
		t.Fatal("expected missing allowlist refusal")
	}
	root := t.TempDir()
	if _, err := config.FromEnv(map[string]string{"CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS": root}); err == nil {
		t.Fatal("expected missing audit log refusal")
	}
}

func TestFromEnvSupportsAllowedRepoRoots(t *testing.T) {
	root := t.TempDir()
	audit := filepath.Join(root, "audit.jsonl")
	cfg, err := config.FromEnv(map[string]string{
		"CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS": root,
		"CODEX_SAFE_GIT_AUDIT_LOG":          audit,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedRoot := canonicalPath(t, root)
	if _, ok := cfg.AllowedRepoRoots[expectedRoot]; !ok {
		t.Fatalf("expected allowed root %q in %#v", expectedRoot, cfg.AllowedRepoRoots)
	}
}

func TestFromEnvRejectsFilesystemRoot(t *testing.T) {
	_, err := config.FromEnv(map[string]string{
		"CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS": string(os.PathSeparator),
		"CODEX_SAFE_GIT_AUDIT_LOG":          filepath.Join(t.TempDir(), "audit.jsonl"),
	})
	if err == nil {
		t.Fatal("expected filesystem root refusal")
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
