package mailauth

import (
	"net/mail"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Result
		ok   bool
	}{
		{
			name: "all three mechanisms",
			raw: "mx.example.com; spf=pass smtp.mailfrom=example.net; " +
				"dkim=pass header.d=example.net; dmarc=pass header.from=example.net",
			want: Result{SPF: "pass", DKIM: "pass", DMARC: "pass"},
			ok:   true,
		},
		{
			name: "the first clause is the authserv id and carries no result",
			raw:  "mx.example.com; spf=fail smtp.mailfrom=evil.example",
			want: Result{SPF: "fail"},
			ok:   true,
		},
		{
			name: "verdicts are case insensitive",
			raw:  "mx; SPF=SoftFail; DKIM=Pass",
			want: Result{SPF: "softfail", DKIM: "pass"},
			ok:   true,
		},
		{
			name: "trailing property clauses are dropped",
			raw:  "mx; dkim=pass header.d=example.net header.s=sel1 (32-byte body)",
			want: Result{DKIM: "pass"},
			ok:   true,
		},
		{
			name: "folded continuation lines arrive as one space",
			raw:  "mx;\r\n spf=pass smtp.mailfrom=example.net;\r\n dkim=none",
			want: Result{SPF: "pass", DKIM: "none"},
			ok:   true,
		},
		{
			name: "a header for other mechanisms alone is not a verdict",
			raw:  "mx; arc=none; iprev=pass",
			want: Result{},
			ok:   false,
		},
		{
			name: "a spf none is a verdict, not a missing one",
			raw:  "mx; spf=none",
			want: Result{SPF: "none"},
			ok:   true,
		},
		{
			name: "an empty header is not a verdict",
			raw:  "",
			want: Result{},
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseHeader(tt.raw)
			if ok != tt.ok {
				t.Fatalf("parseHeader(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("parseHeader(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParsePrefersTheTopmostHeader(t *testing.T) {
	msg := readMail(t, "Authentication-Results: mx.mailbox.example; spf=fail\r\n"+
		"Authentication-Results: mx.first-hop.example; spf=pass\r\n"+
		"\r\nbody")
	// The mailbox's own server stamped its verdict last, so it is the first
	// header in the message and the one this installation has to trust.
	if got := Parse(msg).SPF; got != "fail" {
		t.Fatalf("Parse(msg).SPF = %q, want %q", got, "fail")
	}
}

func TestParseFallsBackWhenTheTopmostHeaderHasNoVerdict(t *testing.T) {
	msg := readMail(t, "Authentication-Results: mx.mailbox.example; iprev=fail\r\n"+
		"Authentication-Results: mx.first-hop.example; dkim=pass\r\n"+
		"\r\nbody")
	if got := Parse(msg).DKIM; got != "pass" {
		t.Fatalf("Parse(msg).DKIM = %q, want %q", got, "pass")
	}
}

func TestRejected(t *testing.T) {
	tests := []struct {
		verdict Result
		reject  bool
	}{
		{Result{SPF: "pass", DKIM: "pass", DMARC: "pass"}, false},
		{Result{SPF: "fail"}, true},
		{Result{SPF: "softfail"}, true},
		{Result{SPF: "permerror"}, true},
		{Result{SPF: "pass", DKIM: "fail", DMARC: "pass"}, true},
		{Result{DMARC: "fail"}, true},
		// A transient or inconclusive outcome never blocks a receipt.
		{Result{SPF: "temperror"}, false},
		{Result{SPF: "neutral"}, false},
		{Result{SPF: "none"}, false},
		{Result{DMARC: "bestguesspass"}, false},
		{Result{DMARC: "nopolicy"}, false},
		// No verdict at all is unverified, not a failure.
		{Result{}, false},
	}
	for _, tt := range tests {
		if got := tt.verdict.Rejected(); got != tt.reject {
			t.Fatalf("%+v.Rejected() = %v, want %v", tt.verdict, got, tt.reject)
		}
	}
}

func TestSummary(t *testing.T) {
	tests := []struct {
		verdict Result
		want    string
	}{
		{Result{SPF: "pass", DKIM: "fail", DMARC: "none"}, "spf=pass dkim=fail dmarc=none"},
		{Result{SPF: "pass"}, "spf=pass"},
		{Result{}, ""},
	}
	for _, tt := range tests {
		if got := tt.verdict.Summary(); got != tt.want {
			t.Fatalf("%+v.Summary() = %q, want %q", tt.verdict, got, tt.want)
		}
	}
}

// readMail builds the message a raw email would produce, so Parse is exercised
// through net/mail rather than against a hand-made Header map.
func readMail(t *testing.T, raw string) *mail.Message {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("mail.ReadMessage: %v", err)
	}
	return msg
}
