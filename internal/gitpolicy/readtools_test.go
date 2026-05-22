package gitpolicy

import "testing"

func TestIntegrityIssueCountsAggregateRepeatedCodes(t *testing.T) {
	issues := parseIntegrityIssues("error: missing blob one\nerror: missing blob two\nwarning: dangling commit three\n")
	if total := totalIntegrityIssueCount(issues); total != 3 {
		t.Fatalf("unexpected total issue count: %d from %#v", total, issues)
	}
	if len(issues) != 2 {
		t.Fatalf("expected grouped issue codes, got %#v", issues)
	}
}

func TestParseSubmoduleStatusLineSupportsSHA256ObjectIDs(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	entry, ok := parseSubmoduleStatusLine(" "+hash+" vendor/child (heads/main)", nil)
	if !ok {
		t.Fatal("expected SHA-256 submodule status to parse")
	}
	if entry.Head != hash || entry.Path != "vendor/child" || entry.Status != "clean" {
		t.Fatalf("unexpected submodule entry: %#v", entry)
	}
}

func TestParseSubmoduleStatusLineSupportsConfiguredPathWithSpaces(t *testing.T) {
	hash := "0123456789abcdef0123456789abcdef01234567"
	configured := map[string]struct{}{"vendor/child module": {}}
	entry, ok := parseSubmoduleStatusLine(" "+hash+" vendor/child module (heads/main)", configured)
	if !ok {
		t.Fatal("expected submodule status to parse")
	}
	if entry.Path != "vendor/child module" {
		t.Fatalf("unexpected submodule path: %#v", entry)
	}
}
