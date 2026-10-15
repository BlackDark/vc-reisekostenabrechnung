// Package mailauth reads the sender authentication verdicts a receiving mail
// server recorded for an email, so that mail intake can file or refuse the
// message on that basis.
//
// The verdict belongs to the mail server, not to this app. A server that
// receives a message checks SPF and DKIM itself and stamps the outcome into an
// Authentication-Results header before delivering the message to a mailbox.
// This package therefore never performs DNS lookups; it reads the header the
// mailbox's own server added and reports what that header claims. A message
// carrying no such header has no verdict, which is reported as unverified and
// not as a failure, because most mail that never traversed an authenticated
// relay still arrives legitimately.
package mailauth

import (
	"net/mail"
	"net/textproto"
	"strings"
)

// arHeader is the canonical MIME key mail.ReadMessage stores the verdicts under.
const arHeader = "Authentication-Results"

// Result holds the verdicts found in an email's topmost Authentication-Results
// header. A blank field means the mechanism carried no verdict at all, which is
// different from a verdict of "none" (the mechanism evaluated the message and
// found nothing to report).
type Result struct {
	SPF   string
	DKIM  string
	DMARC string
}

// Parse reads the verdicts from msg. When several Authentication-Results
// headers are present the topmost wins: a mail server prepends its own result
// while the message is in flight, so the first header in the message comes from
// the last hop the mail made before it reached the mailbox, which is the server
// this installation has to trust.
func Parse(msg *mail.Message) Result {
	if msg == nil {
		return Result{}
	}
	for _, raw := range msg.Header[textproto.CanonicalMIMEHeaderKey(arHeader)] {
		if res, ok := parseHeader(raw); ok {
			return res
		}
	}
	return Result{}
}

// Verified reports whether at least one mechanism reported a verdict. A false
// result means the message carries no usable evidence either way.
func (r Result) Verified() bool {
	return r.SPF != "" || r.DKIM != "" || r.DMARC != ""
}

// Rejected reports whether policy refuses a message carrying these verdicts.
//
// "fail" means the mechanism evaluated the message and found it not authentic.
// "softfail" means the sender's domain publishes no strict policy and so
// declines to vouch for the message. "permerror" means a permanent problem,
// typically a missing or malformed DNS record, prevented evaluation altogether.
// All three leave the sender unproven, and a receipts mailbox accepts mail only
// from a sender it can prove.
//
// Nothing else is a failure: "pass" is genuine, "neutral" and "none" report no
// policy either way, "temperror" is transient (a lookup timed out) so the
// message may well be genuine, and "bestguesspass" and "nopolicy" are DMARC's
// own non-failing outcomes.
func (r Result) Rejected() bool {
	for _, verdict := range [3]string{r.SPF, r.DKIM, r.DMARC} {
		if rejectedVerdicts[verdict] {
			return true
		}
	}
	return false
}

// Summary renders the verdicts for storage and display, for example
// "spf=pass dkim=fail dmarc=none". It is empty when no mechanism reported a
// verdict, so a caller can treat an empty summary as unverified.
func (r Result) Summary() string {
	parts := make([]string, 0, 3)
	for _, m := range [3][2]string{{"spf", r.SPF}, {"dkim", r.DKIM}, {"dmarc", r.DMARC}} {
		if m[1] != "" {
			parts = append(parts, m[0]+"="+m[1])
		}
	}
	return strings.Join(parts, " ")
}

// rejectedVerdicts are the outcomes that leave a sender unproven. Every other
// outcome documented for SPF, DKIM and DMARC is either genuine or inconclusive
// without being damning, so it stays acceptable to a receipts mailbox.
var rejectedVerdicts = map[string]bool{
	"fail":      true,
	"softfail":  true,
	"permerror": true,
}

// parseHeader reads one Authentication-Results header value. It reports false
// when the header held no verdict for the three mechanisms this package
// follows, which lets Parse fall through to the next header in the message.
func parseHeader(raw string) (Result, bool) {
	var res Result
	found := false
	// Go's MIME reader joins folded continuation lines with one space, so the
	// value arrives as a single line even when the sender split it.
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := splitResult(stripComment(part))
		if !ok {
			continue
		}
		switch key {
		case "spf":
			res.SPF, found = value, true
		case "dkim":
			res.DKIM, found = value, true
		case "dmarc":
			res.DMARC, found = value, true
		}
	}
	return res, found
}

// splitResult reads one "method=result ptype.property=value" clause. The first
// clause of the header is the authserv-id and carries no result, so ok is false
// for it and Parse skips it.
func splitResult(part string) (key, value string, ok bool) {
	key, value, found := strings.Cut(part, "=")
	if !found {
		return "", "", false
	}
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.ToLower(firstField(value))
	return key, value, key != "" && value != ""
}

// firstField returns the leading whitespace-delimited token, dropping the
// trailing ptype.property clauses of a result such as
// "dkim=pass header.d=example.net".
func firstField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// stripComment removes the RFC 8601 comment parentheticals a server may append
// to a result, so "spf=pass (Google: domain of ... designates ...)" reads as
// "spf=pass". Comments never nest.
func stripComment(s string) string {
	var out strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				out.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(out.String())
}
