package environ

import (
	"net/url"
	"os"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveNames = []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "KEY", "CREDENTIAL", "AUTH", "PRIVATE"}

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
			out[i] = name + "=" + redactURL(value)
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

func redactURL(value string) string {
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User == nil {
		return value
	}
	if _, ok := u.User.Password(); !ok {
		return value
	}
	u.User = url.User(u.User.Username())
	prefix := u.Scheme + "://" + u.User.String()
	return prefix + ":" + redacted + strings.TrimPrefix(u.String(), prefix)
}
