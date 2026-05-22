package gitexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultTimeout          = 15 * time.Second
	DefaultOutputLimitBytes = 4 * 1024 * 1024
)

type Result struct {
	Stdout           string
	Stderr           string
	ExitCode         int
	StdoutTruncated  bool
	StderrTruncated  bool
	OutputLimitBytes int
}

type Runner struct {
	GitPath          string
	Timeout          time.Duration
	OutputLimitBytes int
}

func NewRunner() (Runner, error) {
	path, err := ResolveGitBinary()
	if err != nil {
		return Runner{}, err
	}
	return Runner{GitPath: path, Timeout: DefaultTimeout}, nil
}

func ResolveGitBinary() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CODEX_SAFE_GIT_GIT_PATH")); configured != "" {
		return validateGitBinaryPath(configured, true)
	}
	path, err := exec.LookPath("git")
	if err != nil {
		return "", errors.New("git executable not found in PATH")
	}
	return validateGitBinaryPath(path, false)
}

func validateGitBinaryPath(path string, explicitlyConfigured bool) (string, error) {
	if strings.TrimSpace(path) != path || strings.ContainsRune(path, '\x00') {
		return "", errors.New("git executable path has unsafe syntax")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if explicitlyConfigured && !filepath.IsAbs(path) {
		return "", errors.New("CODEX_SAFE_GIT_GIT_PATH must be absolute")
	}
	candidate := filepath.Clean(abs)
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}
	if filepath.Base(abs) != "git" {
		return "", errors.New("resolved git executable has unexpected name: " + abs)
	}
	if err := requireExecutable(abs); err != nil {
		return "", err
	}
	if !explicitlyConfigured && !trustedGitPath(candidate, abs) {
		return "", fmt.Errorf("git executable is not in a trusted location: %s", candidate)
	}
	return filepath.Clean(abs), nil
}

func requireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("git executable is not executable: " + path)
	}
	return nil
}

func trustedGitPath(candidate, resolved string) bool {
	cleanedCandidate := filepath.Clean(candidate)
	cleanedResolved := filepath.Clean(resolved)
	if cleanedCandidate != cleanedResolved {
		return pathIsTrustedGit(cleanedResolved)
	}
	return pathIsTrustedGit(cleanedCandidate)
}

func pathIsTrustedGit(path string) bool {
	if filepath.Base(path) != "git" {
		return false
	}
	for _, trusted := range trustedGitExactDirs() {
		if path == filepath.Join(trusted, "git") {
			return true
		}
	}
	for _, trusted := range trustedGitTreePrefixes() {
		if strings.HasPrefix(path, trusted+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func trustedGitExactDirs() []string {
	return []string{
		"/usr/bin",
		"/bin",
		"/usr/local/bin",
		"/usr/local/git/bin",
		"/opt/homebrew/bin",
		"/opt/local/bin",
	}
}

func trustedGitTreePrefixes() []string {
	return []string{
		"/opt/homebrew/Cellar/git",
		"/usr/local/Cellar/git",
		"/nix/store",
	}
}

func (r Runner) Run(repo string, args ...string) (Result, error) {
	return r.run(repo, args, true, nil, true)
}

func (r Runner) RunAllowFailure(repo string, args ...string) (Result, error) {
	return r.run(repo, args, false, nil, true)
}

func (r Runner) RunNoLiteralPathspecWithInputAllowFailure(repo, stdin string, args ...string) (Result, error) {
	return r.run(repo, args, false, &stdin, false)
}

func (r Runner) run(repo string, args []string, checked bool, stdin *string, literalPathspecs bool) (Result, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	outputLimit := r.OutputLimitBytes
	if outputLimit <= 0 {
		outputLimit = DefaultOutputLimitBytes
	}
	hooksDir, err := os.MkdirTemp("", "codex-safe-git-hooks-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(hooksDir)

	commandArgs := []string{
		"-c", "commit.gpgsign=false",
		"-c", "tag.gpgsign=false",
		"-c", "core.hooksPath=" + hooksDir,
		"-c", "credential.helper=",
		"-c", "core.pager=cat",
		"-C", repo,
	}
	commandArgs = append(commandArgs, args...)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.GitPath, commandArgs...)
	cmd.Env = gitEnv(literalPathspecs)
	stdout := newCappedBuffer(outputLimit)
	stderr := newCappedBuffer(outputLimit)
	if stdin != nil {
		cmd.Stdin = strings.NewReader(*stdin)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	result := Result{
		Stdout:           stdout.String(),
		Stderr:           stderr.String(),
		StdoutTruncated:  stdout.Truncated(),
		StderrTruncated:  stderr.Truncated(),
		OutputLimitBytes: outputLimit,
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	} else if ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = -1
		return result, errors.New("git command timed out")
	}
	if result.StdoutTruncated || result.StderrTruncated {
		return result, outputLimitError(args, result)
	}
	if checked && result.ExitCode != 0 {
		return result, errors.New("git " + safeArgSummary(args) + " failed: " + strings.TrimSpace(result.Stderr+result.Stdout))
	}
	if err != nil && result.ExitCode == 0 {
		return result, err
	}
	return result, nil
}

type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.truncated = true
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining > 0 {
		if len(p) <= remaining {
			_, _ = b.buf.Write(p)
			return len(p), nil
		}
		_, _ = b.buf.Write(p[:remaining])
	}
	b.truncated = true
	return len(p), nil
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}

func (b *cappedBuffer) Truncated() bool {
	return b.truncated
}

func outputLimitError(args []string, result Result) error {
	streams := make([]string, 0, 2)
	if result.StdoutTruncated {
		streams = append(streams, "stdout")
	}
	if result.StderrTruncated {
		streams = append(streams, "stderr")
	}
	return fmt.Errorf("git %s output exceeded capture limit: streams=%s limit_bytes=%d", safeArgSummary(args), strings.Join(streams, ","), result.OutputLimitBytes)
}

func gitEnv(literalPathspecs bool) []string {
	names := []string{"PATH", "HOME", "LANG", "LC_ALL", "TMPDIR"}
	env := make([]string, 0, len(names)+6)
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			env = append(env, name+"="+value)
		}
	}
	falsePath := "/usr/bin/false"
	if _, err := os.Stat(falsePath); err != nil {
		falsePath = "false"
	}
	env = append(env,
		"GIT_ASKPASS="+falsePath,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_EDITOR=:",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_TERMINAL_PROMPT=0",
		"SSH_ASKPASS="+falsePath,
	)
	if literalPathspecs {
		env = append(env, "GIT_LITERAL_PATHSPECS=1")
	}
	return env
}

func safeArgSummary(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return args[0]
	}
	return args[0] + " " + args[1]
}
