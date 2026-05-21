package gitexec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const DefaultTimeout = 15 * time.Second

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Runner struct {
	GitPath string
	Timeout time.Duration
}

func NewRunner() (Runner, error) {
	path, err := ResolveGitBinary()
	if err != nil {
		return Runner{}, err
	}
	return Runner{GitPath: path, Timeout: DefaultTimeout}, nil
}

func ResolveGitBinary() (string, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", errors.New("git executable not found in PATH")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		abs = resolved
	}
	if filepath.Base(abs) != "git" {
		return "", errors.New("resolved git executable has unexpected name: " + abs)
	}
	return abs, nil
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
	var stdout, stderr bytes.Buffer
	if stdin != nil {
		cmd.Stdin = strings.NewReader(*stdin)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	} else if ctx.Err() == context.DeadlineExceeded {
		result.ExitCode = -1
		return result, errors.New("git command timed out")
	}
	if checked && result.ExitCode != 0 {
		return result, errors.New("git " + safeArgSummary(args) + " failed: " + strings.TrimSpace(result.Stderr+result.Stdout))
	}
	if err != nil && result.ExitCode == 0 {
		return result, err
	}
	return result, nil
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
