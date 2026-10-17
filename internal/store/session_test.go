package store

import (
	"context"
	"testing"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
)

// A copy of the database must not hand out live sessions, so the table stores
// the hash of the session token and every lookup hashes what it is given.
func TestSessionStoreKeepsTokensHashed(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	sessions := NewSessionStore(s)

	const token = "session-token-as-the-cookie-carries-it"
	if err := sessions.CommitCtx(ctx, token, []byte("data"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := s.write.QueryRowContext(ctx, "SELECT token_hash FROM session").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == token {
		t.Fatalf("session table holds the token itself")
	}
	if stored != auth.HashBearer(token) {
		t.Fatalf("stored %q, want the hash of the token", stored)
	}
	if _, ok, err := sessions.FindCtx(ctx, token); err != nil || !ok {
		t.Fatalf("plaintext lookup: ok=%v err=%v", ok, err)
	}
	// The stored value is not a credential: presenting it must not authenticate.
	if _, ok, err := sessions.FindCtx(ctx, stored); err != nil || ok {
		t.Fatalf("hash lookup: ok=%v err=%v", ok, err)
	}
	if err := sessions.DeleteCtx(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := sessions.FindCtx(ctx, token); err != nil || ok {
		t.Fatalf("after delete: ok=%v err=%v", ok, err)
	}
}
