package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/auth"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/store"
)

// Bootstrap creates the initial Admin or logs a setup token while no Nutzer exists.
// The token is logged on every start until the first Nutzer exists so operators can
// still read it after a restart (`docker compose logs | grep -i setup`).
func Bootstrap(ctx context.Context, st *store.Store, cfg config.Config, pw *auth.Passwords, log *slog.Logger) error {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	n, err := st.CountNutzer(ctx)
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.DataDir, "setup.token")
	if n > 0 {
		_ = os.Remove(path)
		return nil
	}
	if cfg.InitialAdminUser != "" && cfg.InitialAdminPass != "" {
		hash, err := auth.Hash(cfg.InitialAdminPass, pw.Params)
		if err != nil {
			return err
		}
		var email *string
		if cfg.InitialAdminMail != "" {
			e := auth.NormEmail(cfg.InitialAdminMail)
			email = &e
		}
		name := auth.NormName(cfg.InitialAdminUser)
		_, err = st.CreateNutzer(ctx, store.NewNutzer{
			Anzeigename:   name,
			Email:         email,
			Benutzername:  name,
			PasswortHash:  &hash,
			IstAdminLokal: true,
			Sprache:       cfg.DefaultLocale,
			Aktiv:         true,
		}, store.Actor{Art: "system"}, "nutzer.angelegt")
		if err != nil {
			return err
		}
		log.Info("initial admin created", "benutzername", name)
		_ = os.Remove(path)
		return nil
	}
	token, err := readOrCreateToken(path)
	if err != nil {
		return err
	}
	log.Info("setup token pending; open /setup", "setup", token)
	return nil
}

func readOrCreateToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		s := string(b)
		if s != "" {
			return s, nil
		}
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		return "", err
	}
	return token, nil
}

func readSetupToken(dataDir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dataDir, "setup.token"))
	if err != nil {
		return "", err
	}
	return string(b), nil
}
