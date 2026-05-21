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
