package audit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp                  string   `json:"timestamp"`
	Action                     string   `json:"action"`
	Result                     string   `json:"result"`
	Repo                       string   `json:"repo,omitempty"`
	Files                      []string `json:"files,omitempty"`
	CommitHash                 string   `json:"commit_hash,omitempty"`
	Branch                     string   `json:"branch,omitempty"`
	WorktreePath               string   `json:"worktree_path,omitempty"`
	Reason                     string   `json:"reason,omitempty"`
	FileCount                  *int     `json:"file_count,omitempty"`
	RedactedSecretPathCount    *int     `json:"redacted_secret_path_count,omitempty"`
	RedactedUnallowlistedCount *int     `json:"redacted_unallowlisted_count,omitempty"`
	SourceBranch               string   `json:"source_branch,omitempty"`
	TargetBranch               string   `json:"target_branch,omitempty"`
}

type Logger struct {
	Path string
}

func (l Logger) Write(entry Entry) {
	_ = l.WriteChecked(entry)
}

func (l Logger) WriteChecked(entry Entry) error {
	if l.Path == "" {
		return errors.New("audit log path is required")
	}
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func (l Logger) EnsureWritable() error {
	return l.WriteChecked(Entry{Action: "audit_check", Result: "ok"})
}

func IntPtr(value int) *int {
	return &value
}
