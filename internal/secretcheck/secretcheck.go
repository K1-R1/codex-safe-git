package secretcheck

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	privateKeyPattern = regexp.MustCompile(`BEGIN [A-Z ]*PRIVATE KEY`)
	assignmentPattern = regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password|credential)\s*[:=]\s*['"]?[A-Za-z0-9_./+=:-]{8,}`)
	awsSecretPattern  = regexp.MustCompile(`(?i)aws_secret_access_key\s*=`)
	openAIKeyPattern  = regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`)
	githubKeyPattern  = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{20,}`)
	slackKeyPattern   = regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{20,}`)
)

var secretFileNames = map[string]struct{}{
	".npmrc":           {},
	".pypirc":          {},
	".netrc":           {},
	"auth.json":        {},
	"credentials":      {},
	"credentials.json": {},
	"id_dsa":           {},
	"id_ecdsa":         {},
	"id_ed25519":       {},
	"id_rsa":           {},
	"secrets.json":     {},
	"tokens.json":      {},
}

var secretDirNames = map[string]struct{}{
	".aws":    {},
	".azure":  {},
	".docker": {},
	".gnupg":  {},
	".kube":   {},
	".ssh":    {},
	"wallet":  {},
	"wallets": {},
}

var secretSuffixes = []string{".key", ".p12", ".pem", ".pfx"}

func IsSecretPath(rel string) bool {
	clean := filepath.ToSlash(filepath.Clean(rel))
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		lower := strings.ToLower(part)
		if _, ok := secretDirNames[lower]; ok {
			return true
		}
		if strings.Contains(lower, "wallet") {
			return true
		}
	}
	name := strings.ToLower(parts[len(parts)-1])
	if name == ".env" || strings.HasPrefix(name, ".env.") {
		return true
	}
	if _, ok := secretFileNames[name]; ok {
		return true
	}
	for _, prefix := range []string{"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	for _, suffix := range secretSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func ContainsLikelySecret(line string) bool {
	if privateKeyPattern.MatchString(line) ||
		awsSecretPattern.MatchString(line) ||
		openAIKeyPattern.MatchString(line) ||
		githubKeyPattern.MatchString(line) ||
		slackKeyPattern.MatchString(line) {
		return true
	}
	matches := assignmentPattern.FindAllStringIndex(line, -1)
	for _, match := range matches {
		if match[0] > 0 {
			prev := rune(line[match[0]-1])
			if unicode.IsLetter(prev) || unicode.IsDigit(prev) || prev == '_' {
				continue
			}
		}
		return true
	}
	return false
}
