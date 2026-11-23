package accesstoken

import (
	"strings"
	"unicode"
)

// Scopes splits scp at whitespace outside double quotes; a backslash escapes the next character.
// Each scope is returned as written, quotes and backslashes included.
func (p *Payload) Scopes() []string {
	var scopes []string
	var current strings.Builder
	inQuotes, escaped := false, false
	for _, r := range p.Scp {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
		case unicode.IsSpace(r) && !inQuotes:
			scopes = appendScope(scopes, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	return appendScope(scopes, current.String())
}

func appendScope(scopes []string, scope string) []string {
	if UnquoteScope(scope) == "" {
		return scopes
	}
	return append(scopes, scope)
}

// UnquoteScope removes the quotes of a scope and resolves its backslash escapes; a trailing backslash is kept.
func UnquoteScope(scope string) string {
	var unquoted strings.Builder
	escaped := false
	for _, r := range scope {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
			continue
		case r == '"':
			continue
		}
		unquoted.WriteRune(r)
	}
	if escaped {
		unquoted.WriteRune('\\')
	}
	return unquoted.String()
}
