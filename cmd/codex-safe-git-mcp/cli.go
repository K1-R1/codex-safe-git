package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/K1-R1/codex-safe-git/internal/mcp"
)

type homeDirFunc func() (string, error)

func handleArgs(args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "could not determine executable path: %v\n", err)
		return 1, true
	}
	return handleArgsWithEnv(args, stdout, stderr, exe, environMap(), os.UserHomeDir)
}

func handleArgsWithEnv(args []string, stdout, stderr io.Writer, executable string, env map[string]string, homeDir homeDirFunc) (int, bool) {
	if len(args) == 0 {
		return 0, false
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "expected at most one option")
		printUsage(stderr)
		return 2, true
	}
	switch args[0] {
	case "--version":
		fmt.Fprintf(stdout, "codex-safe-git %s\n", mcp.ServerVersion)
		return 0, true
	case "--print-config":
		if err := printConfig(stdout, executable, env, homeDir); err != nil {
			fmt.Fprintf(stderr, "could not print config: %v\n", err)
			return 1, true
		}
		return 0, true
	case "--help", "-h":
		printUsage(stdout)
		return 0, true
	default:
		fmt.Fprintf(stderr, "unknown option: %s\n", args[0])
		printUsage(stderr)
		return 2, true
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: codex-safe-git-mcp [--print-config] [--version] [--help]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Without options, starts the stdio MCP server.")
}

func printConfig(w io.Writer, executable string, env map[string]string, homeDir homeDirFunc) error {
	command, err := cleanExecutablePath(executable)
	if err != nil {
		return err
	}
	codexHome, err := codexHome(env, homeDir)
	if err != nil {
		return err
	}
	allowedRepos := strings.TrimSpace(env["CODEX_SAFE_GIT_ALLOWED_REPOS"])
	allowedRoots := strings.TrimSpace(env["CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS"])
	if allowedRepos == "" && allowedRoots == "" {
		allowedRoots = filepath.Join(codexHome, "worktrees")
	}
	auditLog := strings.TrimSpace(env["CODEX_SAFE_GIT_AUDIT_LOG"])
	if auditLog == "" {
		auditLog = filepath.Join(codexHome, "log", "codex-safe-git-audit.jsonl")
	}

	fmt.Fprintln(w, "[mcp_servers.codex_safe_git]")
	fmt.Fprintf(w, "command = %s\n", tomlString(command))
	fmt.Fprintf(w, "enabled_tools = %s\n", tomlArray(mcp.ToolNames()))
	fmt.Fprintln(w, `default_tools_approval_mode = "approve"`)
	fmt.Fprintln(w, "enabled = true")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "[mcp_servers.codex_safe_git.env]")
	if allowedRepos != "" {
		fmt.Fprintf(w, "CODEX_SAFE_GIT_ALLOWED_REPOS = %s\n", tomlString(allowedRepos))
	}
	if allowedRoots != "" {
		fmt.Fprintf(w, "CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS = %s\n", tomlString(allowedRoots))
	}
	fmt.Fprintf(w, "CODEX_SAFE_GIT_AUDIT_LOG = %s\n", tomlString(auditLog))
	if protected := strings.TrimSpace(env["CODEX_SAFE_GIT_PROTECTED_BRANCHES"]); protected != "" {
		fmt.Fprintf(w, "CODEX_SAFE_GIT_PROTECTED_BRANCHES = %s\n", tomlString(protected))
	}
	if gitPath := strings.TrimSpace(env["CODEX_SAFE_GIT_GIT_PATH"]); gitPath != "" {
		fmt.Fprintf(w, "CODEX_SAFE_GIT_GIT_PATH = %s\n", tomlString(gitPath))
	}
	return nil
}

func cleanExecutablePath(executable string) (string, error) {
	abs, err := filepath.Abs(executable)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs), nil
}

func codexHome(env map[string]string, homeDir homeDirFunc) (string, error) {
	raw := strings.TrimSpace(env["CODEX_HOME"])
	if raw == "" {
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		raw = filepath.Join(home, ".codex")
	}
	expanded, err := expandHome(raw, homeDir)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func expandHome(path string, homeDir homeDirFunc) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := homeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func tomlString(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func tomlArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, tomlString(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func environMap() map[string]string {
	result := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}
