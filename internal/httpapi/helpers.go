package httpapi

import (
	"fmt"
	"path"
	"strings"
)

// defaultLLMModel mirrors the per-package defaults used by the job-backed
// features (write.defaultWriteModel, fuse.defaultFuseModel, ...). Handlers that
// call the LLM directly must apply it too, otherwise an omitted model reaches
// the provider as "" and the request always fails.
const defaultLLMModel = "gpt-4o-mini"

// resolveModel returns the first non-blank candidate, falling back to
// defaultLLMModel. Callers pass their preference order, e.g. request model
// first, then the model pinned on the chapter.
func resolveModel(candidates ...string) string {
	for _, c := range candidates {
		if c = strings.TrimSpace(c); c != "" {
			return c
		}
	}
	return defaultLLMModel
}

// headRunes returns the first n runes of s.
func headRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// tailRunes returns the last n runes of s. It mirrors write.lastRunes, which is
// unexported.
func tailRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// attrChars is the RFC 5987 attr-char set: everything else must be
// percent-encoded in an ext-value.
const attrChars = "!#$&+-.^_`|~"

func rfc5987Encode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case strings.IndexByte(attrChars, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// contentDispositionAttachment builds a Content-Disposition value that survives
// a project name containing quotes or non-ASCII runes. Project names allow any
// character (see handleCreateProject in projects.go), so interpolating one
// directly produces a header a client cannot parse. Emit both an ASCII-safe
// filename and an RFC 5987 filename* so every client gets a usable name.
func contentDispositionAttachment(filename string) string {
	var ascii strings.Builder
	for _, r := range filename {
		switch {
		case r < 0x20 || r == 0x7f: // control characters
			continue
		case r == '"' || r == '\\':
			continue
		case r > 0x7f:
			ascii.WriteByte('_')
		default:
			ascii.WriteRune(r)
		}
	}
	fallback := strings.Trim(strings.TrimSpace(ascii.String()), "_")
	if fallback == "" || strings.HasPrefix(fallback, ".") {
		fallback = "novel" + path.Ext(filename)
	}
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", fallback, rfc5987Encode(filename))
}
