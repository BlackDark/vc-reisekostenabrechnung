package auth

import (
	"strings"
	"unicode"
)

// NormName lowercases a login name.
func NormName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NormEmail lowercases an email. Empty stays empty.
func NormEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// BenutzernameOK is the local login-name rule.
func BenutzernameOK(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case i > 0 && (r == '.' || r == '_' || r == '-'):
		default:
			return false
		}
	}
	return true
}

// SanitizeBenutzername builds a login name from an external claim.
func SanitizeBenutzername(s string) string {
	s = NormName(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), ".-_")
	if out == "" {
		return "nutzer"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}
