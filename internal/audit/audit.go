package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Entry struct {
	Timestamp               string   `json:"timestamp"`
	Action                  string   `json:"action"`
	Result                  string   `json:"result"`
	Repo                    string   `json:"repo,omitempty"`
	Files                   []string `json:"files,omitempty"`
	CommitHash              string   `json:"commit_hash,omitempty"`
	Branch                  string   `json:"branch,omitempty"`
	Reason                  string   `json:"reason,omitempty"`
	FileCount               *int     `json:"file_count,omitempty"`
	RedactedSecretPathCount *int     `json:"redacted_secret_path_count,omitempty"`
	SourceBranch            string   `json:"source_branch,omitempty"`
	TargetBranch            string   `json:"target_branch,omitempty"`
}

type Logger struct {
	Path string
}

func (l Logger) Write(entry Entry) {
	if l.Path == "" {
		return
	}
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	encoded, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_, _ = file.Write(append(encoded, '\n'))
}

func IntPtr(value int) *int {
	return &value
}
