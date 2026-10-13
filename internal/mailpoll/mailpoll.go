package mailpoll

import (
	"strconv"
	"strings"
)

// NormalizePort turns a textual port into an integer, defaulting to 993 (implicit
// TLS) when it is empty. A non-numeric port yields 0, which the caller rejects.
func NormalizePort(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 993
	}
	p, err := strconv.Atoi(s)
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

// ProcessedMailbox resolves the target mailbox of a move:<name> processed action,
// reporting ok=false for any other action.
func ProcessedMailbox(action string) (string, bool) {
	if !strings.HasPrefix(action, "move:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(action, "move:")), true
}
