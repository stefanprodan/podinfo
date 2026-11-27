package environ

import (
	"os"
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveNames = []string{"PASS", "SECRET", "TOKEN", "KEY", "CREDENTIAL", "AUTH", "PRIVATE"}

var urlScheme = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://`)

// Redacted returns the process environment with secret values masked.
func Redacted() []string {
	return redact(os.Environ())
}

// RedactHeaders returns a copy of the HTTP headers or gRPC metadata with the
// values of credential headers and cookies masked, and URL passwords hidden.
func RedactHeaders(headers map[string][]string) map[string][]string {
	out := make(map[string][]string, len(headers))
	for name, values := range headers {
		pseudo := strings.HasPrefix(name, ":")
		if !pseudo && (isSensitive(name) || strings.Contains(strings.ToUpper(name), "COOKIE")) {
			out[name] = []string{redacted}
			continue
		}
		masked := make([]string, len(values))
		for i, v := range values {
			masked[i] = redactURLs(v)
		}
		out[name] = masked
	}
	return out
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
			out[i] = name + "=" + redactURLs(value)
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

// redactURLs masks the userinfo password of every URL in the value. Each URL
// runs until the next scheme, so lists, embedded URLs and passwords holding
// commas or '@' are all covered.
func redactURLs(value string) string {
	starts := urlScheme.FindAllStringIndex(value, -1)
	if starts == nil {
		return value
	}
	var b strings.Builder
	b.WriteString(value[:starts[0][0]])
	for i, loc := range starts {
		end := len(value)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		b.WriteString(value[loc[0]:loc[1]])
		b.WriteString(redactUserinfo(value[loc[1]:end]))
	}
	return b.String()
}

func redactUserinfo(rest string) string {
	token, tail := rest, ""
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		token, tail = rest[:i], rest[i:]
	}
	at := strings.LastIndex(token, "@")
	if at < 0 {
		return rest
	}
	user, _, ok := strings.Cut(token[:at], ":")
	if !ok || strings.Contains(user, "/") {
		return rest
	}
	return user + ":" + redacted + token[at:] + tail
}
