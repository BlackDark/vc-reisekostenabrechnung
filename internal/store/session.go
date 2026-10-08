package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

type metaKey struct{}

// SessionMeta is attached to the request context so the session row can record the Nutzer.
type SessionMeta struct {
	NutzerID  string
	UserAgent string
	IP        string
}

// WithSessionMeta stores meta on ctx.
func WithSessionMeta(ctx context.Context, m *SessionMeta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// SessionMetaFrom returns the meta pointer, or nil.
func SessionMetaFrom(ctx context.Context) *SessionMeta {
	m, _ := ctx.Value(metaKey{}).(*SessionMeta)
	return m
}

// SessionStore is an scs store backed by the session table.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore uses the write pool so commits stay ordered.
func NewSessionStore(s *Store) *SessionStore {
	return &SessionStore{db: s.write}
}

func (s *SessionStore) q() *sqlitedb.Queries { return sqlitedb.New(s.db) }

func (s *SessionStore) Delete(token string) error {
	return s.DeleteCtx(context.Background(), token)
}

func (s *SessionStore) Find(token string) ([]byte, bool, error) {
	return s.FindCtx(context.Background(), token)
}

func (s *SessionStore) Commit(token string, b []byte, expiry time.Time) error {
	return s.CommitCtx(context.Background(), token, b, expiry)
}

func (s *SessionStore) DeleteCtx(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.q().DeleteSession(ctx, token)
}

func (s *SessionStore) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	row, err := s.q().GetSession(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !row.Expiry.After(time.Now()) {
		_ = s.DeleteCtx(ctx, token)
		return nil, false, nil
	}
	raw, err := base64.StdEncoding.DecodeString(row.Data)
	if err != nil {
		return nil, false, nil
	}
	return raw, true, nil
}

func (s *SessionStore) CommitCtx(ctx context.Context, token string, b []byte, expiry time.Time) error {
	now := time.Now().UTC()
	publicID := id.Must()
	created := now
	if existing, err := s.q().GetSession(ctx, token); err == nil {
		publicID = existing.OeffentlichID
		created = existing.ErstelltAm
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var nutzerID, ua, ip *string
	if meta := SessionMetaFrom(ctx); meta != nil {
		if meta.NutzerID != "" {
			nutzerID = &meta.NutzerID
		}
		if meta.UserAgent != "" {
			ua = &meta.UserAgent
		}
		if meta.IP != "" {
			ip = &meta.IP
		}
	}
	return s.q().UpsertSession(ctx, sqlitedb.UpsertSessionParams{
		Token:         token,
		Data:          base64.StdEncoding.EncodeToString(b),
		Expiry:        expiry.UTC(),
		NutzerID:      nutzerID,
		UserAgent:     ua,
		Ip:            ip,
		OeffentlichID: publicID,
		ErstelltAm:    created,
		LetzteNutzung: now,
	})
}
