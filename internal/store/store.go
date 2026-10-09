package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/BlackDark/vc-reisekostenabrechnung/db"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/audit"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/id"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store/sqlitedb"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("version conflict")
	ErrLastAdmin = errors.New("last admin")
	ErrInvalid   = errors.New("invalid")
)

// Store is the SQLite repository. Writes use a single connection.
type Store struct {
	write *sql.DB
	read  *sql.DB
}

// Open opens the database and applies migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	write, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)
	write.SetMaxIdleConns(1)
	read, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		_ = write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(8)
	s := &Store{write: write, read: read}
	if err := s.Migrate(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	if err := s.SeedSatztabellen(ctx); err != nil {
		_ = s.Close()
		return nil, err
	}
	if _, err := write.ExecContext(ctx, "PRAGMA optimize"); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func dsn(path string) string {
	return "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
}

// Close closes both pools.
func (s *Store) Close() error {
	err1 := s.write.Close()
	err2 := s.read.Close()
	return errors.Join(err1, err2)
}

// Ping checks the write pool.
func (s *Store) Ping(ctx context.Context) error {
	return s.write.PingContext(ctx)
}

// Migrate applies embedded goose migrations.
func (s *Store) Migrate(ctx context.Context) error {
	sub, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, s.write, sub)
	if err != nil {
		return err
	}
	_, err = p.Up(ctx)
	return err
}

// DBVersion returns the current goose version.
func (s *Store) DBVersion(ctx context.Context) (int64, error) {
	sub, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return 0, err
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, s.write, sub)
	if err != nil {
		return 0, err
	}
	return p.GetDBVersion(ctx)
}

func (s *Store) readQ() *sqlitedb.Queries { return sqlitedb.New(s.read) }

func (s *Store) tx(ctx context.Context, fn func(q *sqlitedb.Queries) error) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(sqlitedb.New(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Actor is who caused an audit event.
type Actor struct {
	NutzerID *string
	Art      string
	IP       string
}

// NewNutzer is the input for creating a Nutzer.
type NewNutzer struct {
	Anzeigename      string
	Email            *string
	Benutzername     string
	PasswortHash     *string
	IstAdminLokal    bool
	AdminUeberGruppe bool
	Sprache          string
	Personalnummer   *string
	KIErlaubt        bool
	Aktiv            bool
}

func (s *Store) CountNutzer(ctx context.Context) (int64, error) {
	return s.readQ().CountNutzer(ctx)
}

func (s *Store) CountAktiveAdmins(ctx context.Context) (int64, error) {
	return s.readQ().CountAktiveAdmins(ctx)
}

func (s *Store) GetNutzer(ctx context.Context, id string) (sqlitedb.Nutzer, error) {
	n, err := s.readQ().GetNutzerByID(ctx, id)
	return n, mapErr(err)
}

func (s *Store) GetByBenutzername(ctx context.Context, name string) (sqlitedb.Nutzer, error) {
	n, err := s.readQ().GetNutzerByBenutzername(ctx, name)
	return n, mapErr(err)
}

func (s *Store) GetByEmail(ctx context.Context, email string) (sqlitedb.Nutzer, error) {
	n, err := s.readQ().GetNutzerByEmail(ctx, &email)
	return n, mapErr(err)
}

func (s *Store) ListNutzer(ctx context.Context, cursor *string, limit int64) ([]sqlitedb.Nutzer, error) {
	var c any
	if cursor != nil {
		c = *cursor
	}
	return s.readQ().ListNutzer(ctx, sqlitedb.ListNutzerParams{Cursor: c, LimitN: limit})
}

func (s *Store) CreateNutzer(ctx context.Context, in NewNutzer, actor Actor, aktion string) (sqlitedb.Nutzer, error) {
	var out sqlitedb.Nutzer
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		n, err := q.CreateNutzer(ctx, sqlitedb.CreateNutzerParams{
			ID:               id.Must(),
			Anzeigename:      in.Anzeigename,
			Email:            in.Email,
			Benutzername:     in.Benutzername,
			PasswortHash:     in.PasswortHash,
			IstAdminLokal:    in.IstAdminLokal,
			AdminUeberGruppe: in.AdminUeberGruppe,
			Sprache:          in.Sprache,
			Personalnummer:   in.Personalnummer,
			KiErlaubt:        in.KIErlaubt,
			Aktiv:            in.Aktiv,
			ErstelltAm:       now,
			GeaendertAm:      now,
		})
		if err != nil {
			return err
		}
		out = n
		return appendAudit(ctx, q, actor, aktion, "nutzer", n.ID, nil, publicJSON(n), nil)
	})
	return out, err
}

func (s *Store) UpdateProfil(ctx context.Context, id string, version int64, anzeigename, sprache string, personalnummer *string, ki bool, actor Actor) (sqlitedb.Nutzer, error) {
	var out sqlitedb.Nutzer
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetNutzerByID(ctx, id)
		if err != nil {
			return mapErr(err)
		}
		n, err := q.UpdateNutzerProfil(ctx, sqlitedb.UpdateNutzerProfilParams{
			Anzeigename:    anzeigename,
			Sprache:        sprache,
			Personalnummer: personalnummer,
			KiErlaubt:      ki,
			GeaendertAm:    time.Now().UTC(),
			ID:             id,
			Version:        version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		out = n
		return appendAudit(ctx, q, actor, "nutzer.profil", "nutzer", id, publicJSON(prev), publicJSON(n), nil)
	})
	return out, err
}

func (s *Store) UpdatePasswort(ctx context.Context, id string, version int64, hash string, actor Actor) (sqlitedb.Nutzer, error) {
	var out sqlitedb.Nutzer
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		n, err := q.UpdateNutzerPasswort(ctx, sqlitedb.UpdateNutzerPasswortParams{
			PasswortHash: &hash,
			GeaendertAm:  time.Now().UTC(),
			ID:           id,
			Version:      version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		out = n
		return appendAudit(ctx, q, actor, "nutzer.passwort", "nutzer", id, nil, nil, nil)
	})
	return out, err
}

func (s *Store) UpdateAdmin(ctx context.Context, id string, version int64, aktiv, adminLokal bool, newHash *string, actor Actor) (sqlitedb.Nutzer, error) {
	var out sqlitedb.Nutzer
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetNutzerByID(ctx, id)
		if err != nil {
			return mapErr(err)
		}
		if wasAdmin(prev) && (!aktiv || (!adminLokal && !prev.AdminUeberGruppe)) {
			n, err := q.CountAktiveAdmins(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastAdmin
			}
		}
		updated, err := q.UpdateNutzerAdmin(ctx, sqlitedb.UpdateNutzerAdminParams{
			Aktiv:         aktiv,
			IstAdminLokal: adminLokal,
			PasswortHash:  newHash,
			GeaendertAm:   time.Now().UTC(),
			ID:            id,
			Version:       version,
		})
		if err != nil {
			return mapUpdateErr(err)
		}
		if !updated.Aktiv {
			if err := q.DeleteSessionsForNutzer(ctx, &id); err != nil {
				return err
			}
		}
		out = updated
		return appendAudit(ctx, q, actor, "nutzer.admin", "nutzer", id, publicJSON(prev), publicJSON(updated), nil)
	})
	return out, err
}

func (s *Store) TouchLogin(ctx context.Context, nutzerID string, actor Actor, ok bool) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		aktion := "anmeldung"
		if !ok {
			aktion = "anmeldung.fehlgeschlagen"
		} else if err := q.TouchAnmeldung(ctx, sqlitedb.TouchAnmeldungParams{
			LetzteAnmeldung: ptrTime(time.Now().UTC()),
			ID:              nutzerID,
		}); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, aktion, "nutzer", nutzerID, nil, nil, nil)
	})
}

func (s *Store) RecordFailure(ctx context.Context, benutzername, ip string) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		return appendAudit(ctx, q, Actor{Art: "system", IP: ip}, "anmeldung.fehlgeschlagen", "nutzer", benutzername, nil, nil, nil)
	})
}

func (s *Store) SetGroupAdmin(ctx context.Context, id string, admin bool) error {
	return s.SyncGroupAdmin(ctx, id, admin, Actor{Art: "system"})
}

// SyncGroupAdmin updates admin_ueber_gruppe and refuses to drop the last active Admin.
func (s *Store) SyncGroupAdmin(ctx context.Context, id string, admin bool, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		prev, err := q.GetNutzerByID(ctx, id)
		if err != nil {
			return mapErr(err)
		}
		if prev.AdminUeberGruppe == admin {
			return nil
		}
		if prev.AdminUeberGruppe && !admin && prev.Aktiv && !prev.IstAdminLokal {
			n, err := q.CountAktiveAdmins(ctx)
			if err != nil {
				return err
			}
			if n <= 1 {
				return ErrLastAdmin
			}
		}
		if err := q.SetAdminUeberGruppe(ctx, sqlitedb.SetAdminUeberGruppeParams{
			AdminUeberGruppe: admin,
			GeaendertAm:      time.Now().UTC(),
			ID:               id,
		}); err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "nutzer.admin", "nutzer", id, publicJSON(prev), mustJSON(map[string]any{
			"admin_ueber_gruppe": admin,
		}), nil)
	})
}

// SessionByToken loads one session row.
func (s *Store) SessionByToken(ctx context.Context, token string) (sqlitedb.Session, error) {
	row, err := sqlitedb.New(s.write).GetSession(ctx, token)
	return row, mapErr(err)
}

// RecordAudit appends one audit event.
func (s *Store) RecordAudit(ctx context.Context, actor Actor, aktion, objektTyp, objektID string) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		return appendAudit(ctx, q, actor, aktion, objektTyp, objektID, nil, nil, nil)
	})
}

func (s *Store) GetIdentitaet(ctx context.Context, art, aussteller, subjekt string) (sqlitedb.NutzerIdentitaet, error) {
	row, err := s.readQ().GetIdentitaet(ctx, sqlitedb.GetIdentitaetParams{Art: art, Aussteller: aussteller, Subjekt: subjekt})
	return row, mapErr(err)
}

func (s *Store) LinkIdentitaet(ctx context.Context, nutzerID, art, aussteller, subjekt string, actor Actor) error {
	return s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		row, err := q.CreateIdentitaet(ctx, sqlitedb.CreateIdentitaetParams{
			ID:             id.Must(),
			NutzerID:       nutzerID,
			Art:            art,
			Aussteller:     aussteller,
			Subjekt:        subjekt,
			ZuletztGesehen: &now,
			ErstelltAm:     now,
			GeaendertAm:    now,
		})
		if err != nil {
			return err
		}
		return appendAudit(ctx, q, actor, "identitaet.verknuepft", "nutzer_identitaet", row.ID, nil, mustJSON(map[string]string{
			"art": art, "aussteller": aussteller, "subjekt": subjekt, "nutzer_id": nutzerID,
		}), nil)
	})
}

func (s *Store) CreateWithIdentitaet(ctx context.Context, in NewNutzer, art, aussteller, subjekt string, actor Actor) (sqlitedb.Nutzer, error) {
	var out sqlitedb.Nutzer
	err := s.tx(ctx, func(q *sqlitedb.Queries) error {
		now := time.Now().UTC()
		n, err := q.CreateNutzer(ctx, sqlitedb.CreateNutzerParams{
			ID: id.Must(), Anzeigename: in.Anzeigename, Email: in.Email, Benutzername: in.Benutzername,
			PasswortHash: in.PasswortHash, IstAdminLokal: in.IstAdminLokal, AdminUeberGruppe: in.AdminUeberGruppe,
			Sprache: in.Sprache, Personalnummer: in.Personalnummer, KiErlaubt: in.KIErlaubt, Aktiv: true,
			ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		ident, err := q.CreateIdentitaet(ctx, sqlitedb.CreateIdentitaetParams{
			ID: id.Must(), NutzerID: n.ID, Art: art, Aussteller: aussteller, Subjekt: subjekt,
			ZuletztGesehen: &now, ErstelltAm: now, GeaendertAm: now,
		})
		if err != nil {
			return err
		}
		if err := appendAudit(ctx, q, actor, "nutzer.angelegt", "nutzer", n.ID, nil, publicJSON(n), nil); err != nil {
			return err
		}
		out = n
		return appendAudit(ctx, q, actor, "identitaet.verknuepft", "nutzer_identitaet", ident.ID, nil, mustJSON(map[string]string{
			"art": art, "subjekt": subjekt,
		}), nil)
	})
	return out, err
}

func (s *Store) TouchIdentitaet(ctx context.Context, identID string) error {
	now := time.Now().UTC()
	return sqlitedb.New(s.write).TouchIdentitaet(ctx, sqlitedb.TouchIdentitaetParams{
		ZuletztGesehen: &now, GeaendertAm: now, ID: identID,
	})
}

func (s *Store) ListSessions(ctx context.Context, nutzerID string) ([]sqlitedb.Session, error) {
	return s.readQ().ListSessionsByNutzer(ctx, sqlitedb.ListSessionsByNutzerParams{NutzerID: &nutzerID, Now: time.Now().UTC()})
}

func (s *Store) DeleteSessionFor(ctx context.Context, nutzerID, publicID string) error {
	return sqlitedb.New(s.write).DeleteSessionByPublicID(ctx, sqlitedb.DeleteSessionByPublicIDParams{
		OeffentlichID: publicID, NutzerID: &nutzerID,
	})
}

func (s *Store) DeleteSessions(ctx context.Context, nutzerID string) error {
	return sqlitedb.New(s.write).DeleteSessionsForNutzer(ctx, &nutzerID)
}

func (s *Store) AuditAll(ctx context.Context) ([]audit.Event, error) {
	rows, err := s.readQ().ListAudit(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]audit.Event, 0, len(rows))
	for _, r := range rows {
		ev := audit.Event{
			ID: r.ID, Zeitpunkt: r.Zeitpunkt.UTC(), AkteurNutzerID: r.AkteurNutzerID, AkteurArt: r.AkteurArt,
			Aktion: r.Aktion, ObjektTyp: r.ObjektTyp, ObjektID: r.ObjektID, Grund: r.Grund,
			VorgaengerHash: r.VorgaengerHash, Hash: r.Hash,
		}
		if r.Vorher != nil {
			ev.Vorher = json.RawMessage(*r.Vorher)
		}
		if r.Nachher != nil {
			ev.Nachher = json.RawMessage(*r.Nachher)
		}
		if r.Ip != nil {
			ev.IP = *r.Ip
		}
		out = append(out, ev)
	}
	return out, nil
}

func (s *Store) VerifyAudit(ctx context.Context) error {
	ev, err := s.AuditAll(ctx)
	if err != nil {
		return err
	}
	return audit.Verify(ev)
}

func appendAudit(ctx context.Context, q *sqlitedb.Queries, actor Actor, aktion, objektTyp, objektID string, vorher, nachher json.RawMessage, grund *string) error {
	prev := ""
	h, err := q.LastAuditHash(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		prev = h
	}
	if actor.Art == "" {
		actor.Art = "system"
	}
	ev := audit.Event{
		ID: id.Must(), Zeitpunkt: time.Now().UTC(), AkteurNutzerID: actor.NutzerID, AkteurArt: actor.Art,
		Aktion: aktion, ObjektTyp: objektTyp, ObjektID: objektID, Vorher: vorher, Nachher: nachher,
		Grund: grund, IP: actor.IP,
	}
	if err := audit.Seal(prev, &ev); err != nil {
		return err
	}
	var ip *string
	if ev.IP != "" {
		ip = &ev.IP
	}
	return q.InsertAudit(ctx, sqlitedb.InsertAuditParams{
		ID: ev.ID, Zeitpunkt: ev.Zeitpunkt, AkteurNutzerID: ev.AkteurNutzerID, AkteurArt: ev.AkteurArt,
		Aktion: ev.Aktion, ObjektTyp: ev.ObjektTyp, ObjektID: ev.ObjektID,
		Vorher: rawPtr(ev.Vorher), Nachher: rawPtr(ev.Nachher), Grund: ev.Grund, Ip: ip,
		VorgaengerHash: ev.VorgaengerHash, Hash: ev.Hash,
	})
}

func rawPtr(r json.RawMessage) *string {
	if len(r) == 0 {
		return nil
	}
	s := string(r)
	return &s
}

func publicJSON(n sqlitedb.Nutzer) json.RawMessage {
	return mustJSON(map[string]any{
		"id": n.ID, "anzeigename": n.Anzeigename, "benutzername": n.Benutzername,
		"email": n.Email, "ist_admin_lokal": n.IstAdminLokal, "admin_ueber_gruppe": n.AdminUeberGruppe,
		"aktiv": n.Aktiv, "sprache": n.Sprache,
	})
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

func wasAdmin(n sqlitedb.Nutzer) bool {
	return n.Aktiv && (n.IstAdminLokal || n.AdminUeberGruppe)
}

func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func mapUpdateErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConflict
	}
	return err
}

func ptrTime(t time.Time) *time.Time { return &t }

// UniqueName finds a free benutzername starting at base.
func (s *Store) UniqueName(ctx context.Context, base string) (string, error) {
	name := base
	for i := 0; i < 50; i++ {
		if i > 0 {
			name = fmt.Sprintf("%s-%d", base, i+1)
		}
		_, err := s.GetByBenutzername(ctx, name)
		if errors.Is(err, ErrNotFound) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
	return base + "-" + id.Must()[:8], nil
}
