package environ

import (
	"os"
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveNames = []string{"PASS", "SECRET", "TOKEN", "KEY", "CREDENTIAL", "AUTH", "PRIVATE"}

// urlPassword matches the password in the userinfo of every URL in a value,
// stopping at whitespace or a comma so lists and embedded URLs are covered.
var urlPassword = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:@/\s,]*):[^\s,]*@`)

// Redacted returns the process environment with secret values masked.
func Redacted() []string {
	return redact(os.Environ())
}

func redact(env []string) []string {
	out := make([]string, len(env))
	for i, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		switch {
		case !ok:
			out[i] = kv
		case isSensitive(name):
			out[i] = name + "=" + redacted
		default:
			out[i] = name + "=" + urlPassword.ReplaceAllString(value, "${1}:"+redacted+"@")
		}
	}
	return out
}

func isSensitive(name string) bool {
	name = strings.ToUpper(name)
	if strings.HasSuffix(name, "_HEADERS") {
		return true
	}
	for _, s := range sensitiveNames {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}
