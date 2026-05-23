package gitpolicy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/K1-R1/codex-safe-git/internal/secretcheck"
)

func (p Policy) normaliseFiles(repo string, files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, refuse("at least one file must be listed")
	}
	if len(files) > CommitFileLimit {
		return nil, refuse(fmt.Sprintf("too many requested files: limit is %d", CommitFileLimit))
	}
	repo = filepath.Clean(repo)
	seen := map[string]struct{}{}
	result := make([]string, 0, len(files))
	for _, item := range files {
		rel, filePath, err := normaliseRepoPath(repo, item, "requested file", "files")
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(item) == "" {
			return nil, refuse("requested files must be non-empty strings")
		}
		if _, ok := seen[rel]; ok {
			return nil, refuse("duplicate requested file: " + rel)
		}
		seen[rel] = struct{}{}
		if secretcheck.IsSecretPath(rel) {
			return nil, refuse("refusing secret-bearing path: " + rel)
		}
		hasSymlinkPrefix, err := pathPrefixContainsSymlink(repo, rel)
		if err != nil {
			return nil, err
		}
		if hasSymlinkPrefix {
			return nil, refuse("requested path must not contain symlink components: " + rel)
		}
		info, lstatErr := os.Lstat(filePath)
		if lstatErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return nil, refuse("requested path must not be a symlink: " + rel)
			}
			if !info.Mode().IsRegular() {
				return nil, refuse("requested path must be a regular file: " + rel)
			}
		} else if errors.Is(lstatErr, os.ErrNotExist) {
			tracked, err := p.tracked(repo, rel)
			if err != nil {
				return nil, err
			}
			if !tracked {
				return nil, refuse("requested file does not exist and is not tracked: " + rel)
			}
		} else {
			return nil, lstatErr
		}
		result = append(result, rel)
	}
	return result, nil
}

func normaliseRepoPath(repo, input, label, emptyPlural string) (string, string, error) {
	if strings.TrimSpace(input) == "" {
		return "", "", refuse("requested " + emptyPlural + " must be non-empty strings")
	}
	if explicitPathHasUnsafeSyntax(input) {
		return "", "", refuse(label + " has unsafe syntax")
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	clean := filepath.Clean(abs)
	rel, err := filepath.Rel(repo, clean)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		if resolved, resolveErr := filepath.EvalSymlinks(clean); resolveErr == nil {
			clean = filepath.Clean(resolved)
			rel, err = filepath.Rel(repo, clean)
		}
	}
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", refuse(label + " is outside repo")
	}
	rel = filepath.ToSlash(rel)
	return rel, clean, nil
}

func explicitPathHasUnsafeSyntax(input string) bool {
	if strings.TrimSpace(input) != input || containsControlRune(input) {
		return true
	}
	return hasParentPathSegment(input)
}

func containsControlRune(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

func hasParentPathSegment(value string) bool {
	normalised := filepath.ToSlash(value)
	for _, part := range strings.Split(normalised, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
