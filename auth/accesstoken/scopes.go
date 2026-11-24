package accesstoken

import (
	"strings"
	"unicode"
)

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
