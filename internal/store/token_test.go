package store

import (
	"context"
	"testing"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
)

func TestApiTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	s := openTest(t)
	n, err := s.CreateNutzer(ctx, NewNutzer{
		Anzeigename: "Ada", Benutzername: "ada", Sprache: "de", Aktiv: true,
	}, Actor{Art: "system"}, "nutzer.angelegt")
	if err != nil {
		t.Fatal(err)
	}
	actor := Actor{Art: "nutzer", NutzerID: &n.ID}
	plain, hash, err := auth.NewAPIToken()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tok, err := s.CreateToken(ctx, n.ID, "scanner", hash, now, actor)
	if err != nil {
		t.Fatal(err)
	}
	if tok.TokenHash != hash || tok.TokenHash == plain {
		t.Fatalf("token hash must be the stored hash, not the plaintext")
	}
	if _, err := s.FindActiveToken(ctx, auth.HashAPIToken(plain)); err != nil {
		t.Fatalf("active lookup by real hash: %v", err)
	}
	if _, err := s.FindActiveToken(ctx, "deadbeef"); !errorsIs(err, ErrNotFound) {
		t.Fatalf("unknown hash should be not found, got %v", err)
	}
	rows, err := s.ListTokens(ctx, n.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list %d %v", len(rows), err)
	}
	ok, err := s.RevokeToken(ctx, n.ID, tok.ID, now, actor)
	if err != nil || !ok {
		t.Fatalf("revoke ok=%v err=%v", ok, err)
	}
	if again, err := s.RevokeToken(ctx, n.ID, tok.ID, now, actor); err != nil || again {
		t.Fatalf("second revoke should be a no-op, got ok=%v err=%v", again, err)
	}
	if _, err := s.FindActiveToken(ctx, hash); !errorsIs(err, ErrNotFound) {
		t.Fatalf("revoked token should be gone, got %v", err)
	}
	if err := s.VerifyAudit(ctx); err != nil {
		t.Fatal(err)
	}
}
