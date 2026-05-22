package gitpolicy

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (p Policy) submoduleDirtyPaths(repo string) (map[string]bool, error) {
	entries, err := p.porcelainEntries(repo)
	if err != nil {
		return nil, err
	}
	dirty := map[string]bool{}
	for _, entry := range entries {
		if entry.Path != "" {
			dirty[entry.Path] = true
		}
	}
	return dirty, nil
}

func (p Policy) configuredSubmodulePaths(repo string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	config := filepath.Join(repo, ".gitmodules")
	if _, err := os.Stat(config); errors.Is(err, os.ErrNotExist) {
		return result, nil
	} else if err != nil {
		return nil, err
	}
	raw, err := p.Git.RunAllowFailure(repo, "config", "--file", ".gitmodules", "--get-regexp", `^submodule\..*\.path$`)
	if err != nil {
		return nil, err
	}
	if raw.ExitCode == 1 {
		return result, nil
	}
	if raw.ExitCode != 0 {
		return nil, refuse("submodule path inspection failed: " + strings.TrimSpace(raw.Stderr+raw.Stdout))
	}
	for _, line := range strings.Split(raw.Stdout, "\n") {
		key, value, ok := strings.Cut(strings.TrimRight(line, "\r"), " ")
		if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result[filepath.ToSlash(strings.TrimSpace(value))] = struct{}{}
	}
	return result, nil
}

func parseSubmoduleStatusLine(line string, configuredPaths map[string]struct{}) (SubmoduleEntry, bool) {
	if len(line) < 3 {
		return SubmoduleEntry{}, false
	}
	code := strings.TrimSpace(line[:1])
	rest := strings.TrimSpace(line[1:])
	hash, tail, ok := strings.Cut(rest, " ")
	if !ok || !fullObjectIDPattern.MatchString(hash) {
		return SubmoduleEntry{}, false
	}
	path := submodulePathFromStatusTail(strings.TrimSpace(tail), configuredPaths)
	status := "clean"
	entry := SubmoduleEntry{Path: path, Head: hash, Status: status, StatusCode: code}
	switch code {
	case "-":
		entry.Status = "uninitialised"
		entry.Uninitialised = true
	case "+":
		entry.Status = "out_of_sync"
		entry.OutOfSync = true
	case "U":
		entry.Status = "conflicted"
		entry.Conflicted = true
	case "":
		entry.Status = "clean"
	default:
		entry.Status = "unknown"
	}
	return entry, path != ""
}

func submodulePathFromStatusTail(tail string, configuredPaths map[string]struct{}) string {
	if len(configuredPaths) > 0 {
		paths := make([]string, 0, len(configuredPaths))
		for path := range configuredPaths {
			paths = append(paths, path)
		}
		sort.SliceStable(paths, func(i, j int) bool { return len(paths[i]) > len(paths[j]) })
		for _, path := range paths {
			if tail == path || strings.HasPrefix(tail, path+" ") {
				return path
			}
		}
	}
	if index := strings.LastIndex(tail, " ("); index > 0 && strings.HasSuffix(tail, ")") {
		return strings.TrimSpace(tail[:index])
	}
	fields := strings.Fields(tail)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
