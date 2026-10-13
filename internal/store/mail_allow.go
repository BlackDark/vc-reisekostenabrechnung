package store

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"strings"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

const maxMailAllow = 200

// NormalizeMailAllow normalizes an allowlist entry to a lowercased email address
// or an "@domain" entry, reporting ok=false for anything else. Callers store the
// returned wert; the mail poller matches a sender against it.
func NormalizeMailAllow(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, "@") {
		dom := s[1:]
		if !validDomain(dom) {
			return "", false
		}
		return "@" + dom, true
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return "", false
	}
	return strings.ToLower(addr.Address), true
}

func validDomain(d string) bool {
	if len(d) < 3 || d[0] == '-' || d[len(d)-1] == '-' {
		return false
	}
	for _, r := range d {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if r != '.' && r != '-' && !isLetter && !isDigit {
			return false
		}
	}
	return strings.Contains(d, ".") && !strings.Contains(d, "..")
}

// AddMailErlaubt lets a Nutzer receive mail attachments from an address or a
// whole domain, and audits it.
func (s *Store) AddMailErlaubt(ctx context.Context, nutzerID, wert, erstelltAm string, actor Actor) error {
	q := sqlitedb.New(s.write)
	if err := q.InsertMailErlaubt(ctx, sqlitedb.InsertMailErlaubtParams{
		NutzerID: nutzerID, Wert: wert, ErstelltAm: erstelltAm,
	}); err != nil {
		return err
	}
	return appendAudit(ctx, q, actor, "nutzer.mail.erlaubt", "nutzer", nutzerID, nil,
		mustJSON(map[string]string{"wert": wert}), nil)
}

// ListMailErlaubt returns the Nutzer's allowlist entries.
func (s *Store) ListMailErlaubt(ctx context.Context, nutzerID string) ([]sqlitedb.MailErlaubt, error) {
	return s.readQ().ListMailErlaubt(ctx, sqlitedb.ListMailErlaubtParams{NutzerID: nutzerID, LimitN: maxMailAllow})
}

// RemoveMailErlaubt removes one entry and reports whether it existed.
func (s *Store) RemoveMailErlaubt(ctx context.Context, nutzerID, wert string, actor Actor) (bool, error) {
	q := sqlitedb.New(s.write)
	n, err := q.DeleteMailErlaubt(ctx, sqlitedb.DeleteMailErlaubtParams{NutzerID: nutzerID, Wert: wert})
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if err := appendAudit(ctx, q, actor, "nutzer.mail.widerrufen", "nutzer", nutzerID, nil,
		mustJSON(map[string]string{"wert": wert}), nil); err != nil {
		return false, err
	}
	return true, nil
}

// MailOwnerForAbsender resolves a sender address to a Nutzer: the exact address
// wins over a domain entry, so a user who allowlisted one sender is not shadowed
// by another user allowlisting that domain.
func (s *Store) MailOwnerForAbsender(ctx context.Context, absender string) (string, bool, error) {
	absender = strings.ToLower(strings.TrimSpace(absender))
	if id, ok, err := s.mailOwnerByWert(ctx, absender); err != nil || ok {
		return id, ok, err
	}
	domainWert := ""
	if i := strings.LastIndex(absender, "@"); i >= 0 {
		domainWert = "@" + absender[i+1:]
	}
	return s.mailOwnerByWert(ctx, domainWert)
}

func (s *Store) mailOwnerByWert(ctx context.Context, wert string) (string, bool, error) {
	if wert == "" {
		return "", false, nil
	}
	w, err := s.readQ().FindMailErlaubtByWert(ctx, wert)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return w, true, nil
}
