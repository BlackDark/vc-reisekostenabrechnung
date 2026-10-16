package httpapi

import (
	"context"
	"errors"
	"strings"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

var (
	errKeinZugang = errors.New("kein zugang")
	errInaktiv    = errors.New("inaktiv")
)

type externalLogin struct {
	Art           string
	Issuer        string
	Subject       string
	Username      string
	Name          string
	Email         string
	EmailVerified bool
	Groups        []string
	AdminGroup    string
	AllowedGroup  string
	AllowEmail    bool
	Sprache       string
	IP            string
}

func (a *App) resolveExternal(ctx context.Context, in externalLogin) (sqlitedb.Nutzer, error) {
	actor := store.Actor{Art: "system", IP: in.IP}
	ident, err := a.store.GetIdentitaet(ctx, in.Art, in.Issuer, in.Subject)
	if err == nil {
		n, err := a.store.GetNutzer(ctx, ident.NutzerID)
		if err != nil {
			return sqlitedb.Nutzer{}, err
		}
		if !n.Aktiv {
			// A refused login still belongs in the audit trail, so a write that
			// fails is reported instead of leaving the attempt untraceable.
			if err := a.store.TouchLogin(ctx, n.ID, store.Actor{NutzerID: &n.ID, Art: "nutzer", IP: in.IP}, false); err != nil {
				return sqlitedb.Nutzer{}, err
			}
			return sqlitedb.Nutzer{}, errInaktiv
		}
		if err := a.store.TouchIdentitaet(ctx, ident.ID); err != nil {
			return sqlitedb.Nutzer{}, err
		}
		if err := a.syncAdmin(ctx, n, in); err != nil {
			return sqlitedb.Nutzer{}, err
		}
		// syncAdmin may have moved the admin flag, so the principal is read back
		// rather than returned as it was.
		n, err = a.store.GetNutzer(ctx, n.ID)
		if err != nil {
			return sqlitedb.Nutzer{}, err
		}
		if err := a.store.TouchLogin(ctx, n.ID, store.Actor{NutzerID: &n.ID, Art: "nutzer", IP: in.IP}, true); err != nil {
			return sqlitedb.Nutzer{}, err
		}
		return n, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return sqlitedb.Nutzer{}, err
	}

	email := auth.NormEmail(in.Email)
	if in.AllowEmail && in.EmailVerified && email != "" {
		n, err := a.store.GetByEmail(ctx, email)
		// A lookup that failed for any reason other than "no such address" must not
		// be read as absence, which would create a second account for the address.
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return sqlitedb.Nutzer{}, err
		}
		if err == nil {
			if !n.Aktiv {
				return sqlitedb.Nutzer{}, errInaktiv
			}
			if err := a.store.LinkIdentitaet(ctx, n.ID, in.Art, in.Issuer, in.Subject, actor); err != nil {
				return sqlitedb.Nutzer{}, err
			}
			if err := a.syncAdmin(ctx, n, in); err != nil {
				return sqlitedb.Nutzer{}, err
			}
			n, err = a.store.GetNutzer(ctx, n.ID)
			if err != nil {
				return sqlitedb.Nutzer{}, err
			}
			if err := a.store.TouchLogin(ctx, n.ID, store.Actor{NutzerID: &n.ID, Art: "nutzer", IP: in.IP}, true); err != nil {
				return sqlitedb.Nutzer{}, err
			}
			return n, nil
		}
	}

	inAdmin := auth.InGroup(in.Groups, in.AdminGroup)
	inAllowed := auth.InGroup(in.Groups, in.AllowedGroup)
	if auth.Access(false, false, in.EmailVerified, in.AllowEmail, inAdmin, inAllowed) != "create" {
		return sqlitedb.Nutzer{}, errKeinZugang
	}
	base := auth.SanitizeBenutzername(in.Username)
	if !auth.BenutzernameOK(base) {
		base = auth.SanitizeBenutzername(strings.Split(email, "@")[0])
	}
	if !auth.BenutzernameOK(base) {
		base = "nutzer"
	}
	name, err := a.store.UniqueName(ctx, base)
	if err != nil {
		return sqlitedb.Nutzer{}, err
	}
	anzeige := strings.TrimSpace(in.Name)
	if anzeige == "" {
		anzeige = name
	}
	var emailPtr *string
	if email != "" {
		emailPtr = &email
	}
	sprache := in.Sprache
	if sprache != "de" && sprache != "en" {
		sprache = "de"
	}
	n, err := a.store.CreateWithIdentitaet(ctx, store.NewNutzer{
		Anzeigename:      anzeige,
		Email:            emailPtr,
		Benutzername:     name,
		AdminUeberGruppe: inAdmin,
		Sprache:          sprache,
		Aktiv:            true,
	}, in.Art, in.Issuer, in.Subject, actor)
	if err != nil {
		return sqlitedb.Nutzer{}, err
	}
	if err := a.store.TouchLogin(ctx, n.ID, store.Actor{NutzerID: &n.ID, Art: "nutzer", IP: in.IP}, true); err != nil {
		return sqlitedb.Nutzer{}, err
	}
	return n, nil
}

// syncAdmin moves the Nutzer's admin flag onto what the groups in in say, and
// reports a write that failed, because the flag decides what the account may do.
func (a *App) syncAdmin(ctx context.Context, n sqlitedb.Nutzer, in externalLogin) error {
	want := auth.InGroup(in.Groups, in.AdminGroup)
	if n.AdminUeberGruppe == want {
		return nil
	}
	return a.store.SyncGroupAdmin(ctx, n.ID, want, store.Actor{NutzerID: &n.ID, Art: "nutzer", IP: in.IP})
}
