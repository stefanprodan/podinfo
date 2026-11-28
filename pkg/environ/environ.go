package environ

import (
	"os"
	"regexp"
	"slices"
	"strings"
)

const redacted = "[REDACTED]"

var (
	sensitiveNames = []string{
		"PASS", "SECRET", "TOKEN", "KEY", "CREDENTIAL", "AUTH", "PRIVATE",
		"COOKIE", "JWT", "DSN", "SIGNATURE", "ASSERTION",
	}
	// sensitiveSuffixes match the end of a name only: MYSQL_PWD is a password,
	// PWD and OLDPWD are directories.
	sensitiveSuffixes = []string{"_HEADERS", "_PWD"}
	// sensitiveParams are the URL query parameters masked on top of the names above.
	sensitiveParams = []string{"PWD", "SIG"}
)

const sensitiveHeaderPrefix = "X-AMZN-OIDC-"

var (
	// urlScheme matches the scheme of a URL, up to and including "://".
	urlScheme = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://`)
	// queryParam matches a URL query parameter with its leading '?' or '&'.
	queryParam = regexp.MustCompile(`[?&][^=&#\s]+=[^&#\s]*`)
	// passwordPair matches the password of a key=value connection string;
	// the value may be quoted.
	passwordPair = regexp.MustCompile(`(?i)((?:password|passwd|pwd)=)(?:'[^']*'|"[^"]*"|[^\s;&]*)`)
	// portEnd matches what follows the colon of a host:port, up to the end of the port.
	portEnd = regexp.MustCompile(`^\d*[/?#,;\s]`)
)

// Redacted returns the process environment with secret values masked.
func Redacted() []string {
	return redact(os.Environ())
}

// RedactedMap returns the process environment by name, with secret values masked.
func RedactedMap() map[string]string {
	return toMap(Redacted())
}

// RedactHeaders returns a copy of the HTTP headers or gRPC metadata with the
// values of credential headers and cookies masked, and the secrets of URLs
// and connection strings hidden.
func RedactHeaders(headers map[string][]string) map[string][]string {
	out := make(map[string][]string, len(headers))
	for name, values := range headers {
		pseudo := strings.HasPrefix(name, ":")
		if !pseudo && isSensitiveHeader(name) {
			out[name] = []string{redacted}
			continue
		}
		masked := make([]string, len(values))
		for i, v := range values {
			masked[i] = redactValue(v)
		}
		out[name] = masked
	}
	return out
}

func toMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		out[name] = value
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
			out[i] = name + "=" + redactValue(value)
		}
	}
	return out
}

func isSensitive(name string) bool {
	name = strings.ToUpper(name)
	for _, s := range sensitiveSuffixes {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	for _, s := range sensitiveNames {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}

func isSensitiveHeader(name string) bool {
	return isSensitive(name) || strings.HasPrefix(strings.ToUpper(name), sensitiveHeaderPrefix)
}

func isSensitiveParam(name string) bool {
	return isSensitive(name) || slices.Contains(sensitiveParams, strings.ToUpper(name))
}

// redactValue masks the secrets inside a value: the userinfo of URLs, the
// sensitive query parameters and the passwords of key=value pairs.
func redactValue(value string) string {
	value = redactURLs(value)
	value = queryParam.ReplaceAllStringFunc(value, func(param string) string {
		eq := strings.Index(param, "=")
		if !isSensitiveParam(param[1:eq]) {
			return param
		}
		return param[:eq+1] + redacted
	})
	return passwordPair.ReplaceAllString(value, "${1}"+redacted)
}

// redactURLs masks the userinfo secret of every URL in the value, so lists
// and embedded URLs are covered.
func redactURLs(value string) string {
	var b strings.Builder
	for {
		loc := urlScheme.FindStringIndex(value)
		if loc == nil {
			break
		}
		masked, n := redactUserinfo(value[loc[1]:])
		b.WriteString(value[:loc[1]])
		b.WriteString(masked)
		value = value[loc[1]+n:]
	}
	b.WriteString(value)
	return b.String()
}

// redactUserinfo returns the masked userinfo that starts the rest of a URL
// and the length of the userinfo it replaces. A username without a password
// is masked as a whole, since it is a token in that shape.
func redactUserinfo(rest string) (string, int) {
	if colon := strings.IndexAny(rest, ":/ \t"); colon >= 0 && rest[colon] == ':' {
		if at := passwordEnd(rest, colon); at >= 0 {
			return rest[:colon+1] + redacted, at
		}
	}
	authority := rest
	if i := strings.IndexAny(rest, "/?# \t"); i >= 0 {
		authority = rest[:i]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		return redacted, at
	}
	return "", 0
}

// passwordEnd returns the index of the '@' that ends the password starting
// after the colon, or -1 without one. The password runs to the last '@' ahead
// of the next whitespace or URL, so it may hold '@', '/' and commas. It holds
// whitespace or "://" only when the colon is not followed by a port, which
// tells a password from the next entry of a URL list.
func passwordEnd(rest string, colon int) int {
	first := strings.Index(rest[colon:], "@")
	if first < 0 {
		return -1
	}
	first += colon
	gap := rest[colon+1 : first]
	if strings.ContainsAny(gap, " \t") || urlScheme.MatchString(gap) {
		if portEnd.MatchString(gap) || strings.HasPrefix(rest, "[") {
			return -1
		}
	}
	end := len(rest)
	if i := strings.IndexAny(rest[first:], " \t"); i >= 0 {
		end = first + i
	}
	if loc := urlScheme.FindStringIndex(rest[first:end]); loc != nil {
		end = first + loc[0]
	}
	return strings.LastIndex(rest[:end], "@")
}
