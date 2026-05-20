package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	AllowedRepos      map[string]struct{}
	AllowedRepoRoots  map[string]struct{}
	ProtectedBranches map[string]struct{}
	AuditLog          string
}

func FromEnv(env map[string]string) (Config, error) {
	if env == nil {
		env = environ()
	}
	allowedReposRaw := strings.TrimSpace(env["CODEX_SAFE_GIT_ALLOWED_REPOS"])
	allowedRootsRaw := strings.TrimSpace(env["CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS"])
	protectedRaw := strings.TrimSpace(env["CODEX_SAFE_GIT_PROTECTED_BRANCHES"])
	auditRaw := strings.TrimSpace(env["CODEX_SAFE_GIT_AUDIT_LOG"])
	if allowedReposRaw == "" && allowedRootsRaw == "" {
		return Config{}, errors.New("CODEX_SAFE_GIT_ALLOWED_REPOS or CODEX_SAFE_GIT_ALLOWED_REPO_ROOTS is required")
	}
	if auditRaw == "" {
		return Config{}, errors.New("CODEX_SAFE_GIT_AUDIT_LOG is required")
	}

	allowedRepos, err := pathSetFromEnv(allowedReposRaw, false)
	if err != nil {
		return Config{}, err
	}
	allowedRoots, err := pathSetFromEnv(allowedRootsRaw, true)
	if err != nil {
		return Config{}, err
	}
	if len(allowedRepos) == 0 && len(allowedRoots) == 0 {
		return Config{}, errors.New("allowed repo configuration has no usable entries")
	}
	auditLog, err := normalisePath(auditRaw)
	if err != nil {
		return Config{}, err
	}
	return Config{
		AllowedRepos:      allowedRepos,
		AllowedRepoRoots:  allowedRoots,
		ProtectedBranches: branchSetFromEnv(protectedRaw),
		AuditLog:          auditLog,
	}, nil
}

func environ() map[string]string {
	result := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			result[key] = value
		}
	}
	return result
}

func pathSetFromEnv(raw string, requireDirRoot bool) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	for _, item := range filepath.SplitList(raw) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		path, err := normalisePath(item)
		if err != nil {
			return nil, err
		}
		if requireDirRoot {
			if filepath.Dir(path) == path {
				return nil, errors.New("allowed repo root must not be the filesystem root")
			}
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				return nil, errors.New("allowed repo root does not exist or is not a directory: " + path)
			}
		}
		result[path] = struct{}{}
	}
	return result, nil
}

func branchSetFromEnv(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		branch := strings.TrimSpace(item)
		if branch != "" {
			result[branch] = struct{}{}
		}
	}
	return result
}

func normalisePath(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("path must not be empty")
	}
	if strings.HasPrefix(raw, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if raw == "~" {
			raw = home
		} else if strings.HasPrefix(raw, "~/") {
			raw = filepath.Join(home, raw[2:])
		}
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", err
	}
	cleaned := filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		return filepath.Clean(resolved), nil
	}
	return cleaned, nil
}
