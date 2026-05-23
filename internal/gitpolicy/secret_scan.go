package gitpolicy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/K1-R1/codex-safe-git/internal/secretcheck"
)

func (p Policy) rejectLikelySecretMaterial(repo string, rels []string) error {
	for _, rel := range rels {
		path := filepath.Join(repo, filepath.FromSlash(rel))
		tracked, err := p.tracked(repo, rel)
		if err != nil {
			return err
		}
		if _, statErr := os.Stat(path); statErr == nil && !tracked {
			if err := rejectLikelySecretMaterialInFile(path, rel); err != nil {
				return err
			}
		} else {
			diff, err := p.Git.Run(repo, "diff", "--no-ext-diff", "--unified=0", "--", rel)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(diff.Stdout, "\n") {
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					if secretcheck.ContainsLikelySecret(strings.TrimPrefix(line, "+")) {
						return refuse("requested diff appears to contain secret material: " + rel)
					}
				}
			}
		}
	}
	return nil
}

func (p Policy) rejectLikelySecretMaterialInStagedDiff(repo string, rels []string) error {
	for _, rel := range rels {
		diff, err := p.Git.Run(repo, "diff", "--cached", "--no-ext-diff", "--unified=0", "--", rel)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(diff.Stdout, "\n") {
			if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
				continue
			}
			if secretcheck.ContainsLikelySecret(strings.TrimPrefix(line, "+")) {
				return refuse("requested staged diff appears to contain secret material: " + rel)
			}
		}
	}
	return nil
}

func rejectLikelySecretMaterialInFile(path, rel string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	total := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), CommitLineScanLimit)
	for scanner.Scan() {
		line := scanner.Text()
		total += len(line) + 1
		if total > CommitContentScanLimit {
			return refuse(fmt.Sprintf("requested file exceeds secret scan limit of %d bytes: %s", CommitContentScanLimit, rel))
		}
		if secretcheck.ContainsLikelySecret(line) {
			return refuse("requested diff appears to contain secret material: " + rel)
		}
	}
	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return refuse(fmt.Sprintf("requested file contains a line exceeding secret scan limit of %d bytes: %s", CommitLineScanLimit, rel))
		}
		return err
	}
	return nil
}
