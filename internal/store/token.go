package store

import (
	"context"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

// TokenScopeUpload lets an API token upload receipts into the Belegeingang and
// nothing else. Every other endpoint still requires the session cookie.
const TokenScopeUpload = "belege:upload"

// CreateToken stores a hashed token for the user and audits the creation. The
// timestamp is RFC-3339 UTC, matching the api_token text columns.
func (s *Store) CreateToken(ctx context.Context, nutzerID, name, tokenHash, erstelltAm string, actor Actor) (sqlitedb.ApiToken, error) {
	tok := sqlitedb.ApiToken{
		ID: id.Must(), NutzerID: nutzerID, Name: name,
		Scope: TokenScopeUpload, TokenHash: tokenHash, ErstelltAm: erstelltAm,
	}
	q := sqlitedb.New(s.write)
	if err := q.InsertToken(ctx, sqlitedb.InsertTokenParams{
		ID: tok.ID, NutzerID: nutzerID, Name: name, Scope: TokenScopeUpload,
		TokenHash: tokenHash, ErstelltAm: erstelltAm,
	}); err != nil {
		return sqlitedb.ApiToken{}, err
	}
	if err := appendAudit(ctx, q, actor, "nutzer.token.angelegt", "api_token", tok.ID, nil,
		mustJSON(map[string]string{"name": name, "scope": TokenScopeUpload}), nil); err != nil {
		return sqlitedb.ApiToken{}, err
	}
	return tok, nil
}

// ListTokens returns the user's tokens. The rows carry the hash; the HTTP layer
// drops it before responding.
func (s *Store) ListTokens(ctx context.Context, nutzerID string) ([]sqlitedb.ApiToken, error) {
	return s.readQ().ListTokensByNutzer(ctx, sqlitedb.ListTokensByNutzerParams{NutzerID: nutzerID, LimitN: 200})
}

// FindActiveToken returns a non-revoked token by its stored hash, or ErrNotFound.
func (s *Store) FindActiveToken(ctx context.Context, tokenHash string) (sqlitedb.ApiToken, error) {
	tok, err := s.readQ().FindTokenByHash(ctx, tokenHash)
	if err != nil {
		return sqlitedb.ApiToken{}, mapErr(err)
	}
	if tok.WiderrufenAm != nil {
		return sqlitedb.ApiToken{}, ErrNotFound
	}
	return tok, nil
}

// RevokeToken revokes a token the user owns. It reports whether a live token was
// revoked, so the caller can tell 204 from 404.
func (s *Store) RevokeToken(ctx context.Context, nutzerID, tokenID, am string, actor Actor) (bool, error) {
	q := sqlitedb.New(s.write)
	n, err := q.RevokeToken(ctx, sqlitedb.RevokeTokenParams{Am: &am, ID: tokenID, NutzerID: nutzerID})
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if err := appendAudit(ctx, q, actor, "nutzer.token.widerrufen", "api_token", tokenID, nil, nil, nil); err != nil {
		return false, err
	}
	return true, nil
}

// TouchToken records the last use of a token. Best-effort; callers ignore the
// error so a failed touch cannot fail an otherwise valid upload.
func (s *Store) TouchToken(ctx context.Context, tokenID, am string) error {
	return sqlitedb.New(s.write).TouchToken(ctx, sqlitedb.TouchTokenParams{Am: &am, ID: tokenID})
}
