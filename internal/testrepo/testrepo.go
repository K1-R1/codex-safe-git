package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"local/codex-safe-git/internal/audit"
	"local/codex-safe-git/internal/config"
	"local/codex-safe-git/internal/gitexec"
	"local/codex-safe-git/internal/gitpolicy"
)

type Repo struct {
	T        *testing.T
	Root     string
	Path     string
	AuditLog string
	Policy   gitpolicy.Policy
}

func New(t *testing.T) Repo {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	run(t, "", "git", "init", "-b", "main", repo)
	run(t, repo, "git", "config", "user.name", "Codex Safe Git Test")
	run(t, repo, "git", "config", "user.email", "codex-safe-git@example.invalid")
	write(t, repo, "README.md", "initial\n")
	run(t, repo, "git", "add", "README.md")
	run(t, repo, "git", "commit", "-m", "Initial commit")
	run(t, repo, "git", "switch", "-c", "work")
	auditLog := filepath.Join(root, "audit.jsonl")
	runner, err := gitexec.NewRunner()
	if err != nil {
		t.Fatal(err)
	}
	policy := gitpolicy.Policy{
		Config: config.Config{
			AllowedRepos: map[string]struct{}{
				mustAbs(t, repo): {},
			},
			AllowedRepoRoots: map[string]struct{}{},
			AuditLog:         auditLog,
		},
		Git:   runner,
		Audit: audit.Logger{Path: auditLog},
	}
	return Repo{T: t, Root: root, Path: repo, AuditLog: auditLog, Policy: policy}
}

func (r Repo) Write(rel, contents string) {
	r.T.Helper()
	write(r.T, r.Path, rel, contents)
}

func (r Repo) Run(args ...string) string {
	r.T.Helper()
	return run(r.T, r.Path, "git", args...)
}

func (r Repo) Abs(rel string) string {
	return filepath.Join(r.Path, rel)
}

func Run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	return run(t, dir, name, args...)
}

func Write(t *testing.T, dir, rel, contents string) {
	t.Helper()
	write(t, dir, rel, contents)
}

func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, rel, contents string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
}
