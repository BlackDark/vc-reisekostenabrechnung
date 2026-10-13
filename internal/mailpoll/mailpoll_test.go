package mailpoll

import "testing"

func TestNormalizePort(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 993},
		{"993", 993},
		{"143", 143},
		{"abc", 0},
		{"0", 0},
		{"70000", 0},
		{" 993 ", 993},
	}
	for _, c := range cases {
		if got := NormalizePort(c.in); got != c.want {
			t.Errorf("NormalizePort(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestProcessedMailbox(t *testing.T) {
	if m, ok := ProcessedMailbox("move:Processed"); !ok || m != "Processed" {
		t.Fatalf("move:got %q %v", m, ok)
	}
	if m, ok := ProcessedMailbox("move: Archive "); !ok || m != "Archive" {
		t.Fatalf("trimmed move: %q %v", m, ok)
	}
	if _, ok := ProcessedMailbox("flag"); ok {
		t.Fatal("flag should not resolve a mailbox")
	}
	if _, ok := ProcessedMailbox("delete"); ok {
		t.Fatal("delete should not resolve a mailbox")
	}
	if _, ok := ProcessedMailbox(""); ok {
		t.Fatal("empty action should not resolve a mailbox")
	}
}
