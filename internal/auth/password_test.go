package auth

import "testing"

func TestHashAndVerify(t *testing.T) {
	p, err := NewPasswords(Params{Memory: 8 * 1024, Time: 1, Threads: 1})
	if err != nil {
		t.Fatal(err)
	}
	enc, err := Hash("correct-horse", p.Params)
	if err != nil {
		t.Fatal(err)
	}
	ok, rehash := p.Verify(enc, "correct-horse")
	if !ok || rehash {
		t.Fatalf("ok=%v rehash=%v", ok, rehash)
	}
	ok, _ = p.Verify(enc, "wrong-password")
	if ok {
		t.Fatal("accepted wrong password")
	}
	p.VerifyDummy("anything-at-all")
}

func TestAccess(t *testing.T) {
	if Access(true, false, false, false, false, false) != "existing" {
		t.Fatal()
	}
	if Access(false, true, true, true, false, false) != "link" {
		t.Fatal()
	}
	if Access(false, false, false, false, true, false) != "create" {
		t.Fatal("admin group must be allowed")
	}
	if Access(false, false, false, false, false, false) != "reject" {
		t.Fatal()
	}
}

func TestParseGroups(t *testing.T) {
	g, ok := ParseGroups([]byte(`["admins","users"]`))
	if !ok || len(g) != 2 || g[0] != "admins" {
		t.Fatal(g, ok)
	}
	g, ok = ParseGroups([]byte(`"a, b"`))
	if !ok || len(g) != 2 || g[1] != "b" {
		t.Fatal(g, ok)
	}
}

func TestBenutzername(t *testing.T) {
	if !BenutzernameOK("ada.lovelace") || BenutzernameOK("Ada") || BenutzernameOK(".ada") {
		t.Fatal()
	}
	if got := SanitizeBenutzername("Ada Lovelace"); got != "ada-lovelace" {
		t.Fatal(got)
	}
}
