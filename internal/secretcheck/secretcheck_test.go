package secretcheck_test

import (
	"strings"
	"testing"

	"github.com/K1-R1/codex-safe-git/internal/secretcheck"
)

func TestSecretPaths(t *testing.T) {
	for _, path := range []string{".env", ".env.local", "private.pem", ".ssh/id_rsa", "wallet/data.json"} {
		if !secretcheck.IsSecretPath(path) {
			t.Fatalf("expected %q to be secret path", path)
		}
	}
	if secretcheck.IsSecretPath("src/configuration.txt") {
		t.Fatal("normal source path should not be secret")
	}
}

func TestSecretMaterial(t *testing.T) {
	if !secretcheck.ContainsLikelySecret(strings.Join([]string{"api", "_key = abcdefghijklmnop"}, "")) {
		t.Fatal("expected api_key assignment to be detected")
	}
	if secretcheck.ContainsLikelySecret("LIKELY_SECRET = re.compile('placeholder')") {
		t.Fatal("identifier-contained keyword should not be detected")
	}
}
