package gitexec

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveGitBinaryRejectsUntrustedPathLookup(t *testing.T) {
	dir := t.TempDir()
	fakeGit := filepath.Join(dir, "git")
	if err := os.WriteFile(fakeGit, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("CODEX_SAFE_GIT_GIT_PATH", "")

	if _, err := ResolveGitBinary(); err == nil || !strings.Contains(err.Error(), "trusted location") {
		t.Fatalf("expected untrusted git refusal, got %v", err)
	}
}

func TestResolveGitBinaryAllowsExplicitAbsolutePath(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	absGit, err := filepath.Abs(realGit)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_SAFE_GIT_GIT_PATH", absGit)

	resolved, err := ResolveGitBinary()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(resolved) != "git" {
		t.Fatalf("unexpected git path: %s", resolved)
	}
}

func TestResolveGitBinaryRequiresAbsoluteExplicitPath(t *testing.T) {
	t.Setenv("CODEX_SAFE_GIT_GIT_PATH", "git")

	if _, err := ResolveGitBinary(); err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Fatalf("expected absolute path refusal, got %v", err)
	}
}

func TestTrustedGitPathRejectsTrustedDirSymlinkToUntrustedTarget(t *testing.T) {
	untrusted := filepath.Join(t.TempDir(), "git")
	if trustedGitPath("/usr/local/bin/git", untrusted) {
		t.Fatalf("trusted dir symlink to untrusted target was allowed")
	}
	if !trustedGitPath("/opt/homebrew/bin/git", "/opt/homebrew/Cellar/git/2.51.0/bin/git") {
		t.Fatalf("trusted Homebrew symlink target was refused")
	}
}

func TestGitEnvironmentDisablesOptionalWrites(t *testing.T) {
	env := strings.Join(gitEnv(true), "\n")
	for _, item := range []string{
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_LITERAL_PATHSPECS=1",
		"GIT_TERMINAL_PROMPT=0",
	} {
		if !strings.Contains(env, item) {
			t.Fatalf("missing %s in git env:\n%s", item, env)
		}
	}
}

func TestRunnerRefusesOversizedStdout(t *testing.T) {
	git := writeFakeGit(t, "#!/bin/sh\nprintf 'abcdefghijklmnopqrstuvwxyz'\n")
	runner := Runner{GitPath: git, Timeout: DefaultTimeout, OutputLimitBytes: 8}

	result, err := runner.Run(t.TempDir(), "status")
	if err == nil || !strings.Contains(err.Error(), "output exceeded capture limit") {
		t.Fatalf("expected output limit error, got %v", err)
	}
	if !result.StdoutTruncated || result.StderrTruncated {
		t.Fatalf("unexpected truncation flags: stdout=%v stderr=%v", result.StdoutTruncated, result.StderrTruncated)
	}
	if result.Stdout != "abcdefgh" {
		t.Fatalf("unexpected captured stdout %q", result.Stdout)
	}
	if result.OutputLimitBytes != 8 {
		t.Fatalf("unexpected output limit %d", result.OutputLimitBytes)
	}
}

func TestRunnerRefusesOversizedStderrEvenWhenFailureIsAllowed(t *testing.T) {
	git := writeFakeGit(t, "#!/bin/sh\nprintf 'abcdefghijklmnopqrstuvwxyz' >&2\nexit 1\n")
	runner := Runner{GitPath: git, Timeout: DefaultTimeout, OutputLimitBytes: 8}

	result, err := runner.RunAllowFailure(t.TempDir(), "status")
	if err == nil || !strings.Contains(err.Error(), "output exceeded capture limit") {
		t.Fatalf("expected output limit error, got %v", err)
	}
	if result.StdoutTruncated || !result.StderrTruncated {
		t.Fatalf("unexpected truncation flags: stdout=%v stderr=%v", result.StdoutTruncated, result.StderrTruncated)
	}
	if result.Stderr != "abcdefgh" {
		t.Fatalf("unexpected captured stderr %q", result.Stderr)
	}
	if result.ExitCode != 1 {
		t.Fatalf("unexpected exit code %d", result.ExitCode)
	}
}

func writeFakeGit(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	git := filepath.Join(dir, "git")
	if err := os.WriteFile(git, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return git
}
