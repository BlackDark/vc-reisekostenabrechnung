package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type oidcPending struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Exp      int64  `json:"e"`
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func signOIDC(key []byte, p oidcPending) (string, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	body := base64.RawURLEncoding.EncodeToString(raw)
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

func openOIDC(key []byte, token string) (oidcPending, error) {
	body, sig, ok := splitDot(token)
	if !ok {
		return oidcPending{}, errors.New("malformed")
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return oidcPending{}, err
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return oidcPending{}, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return oidcPending{}, errors.New("bad signature")
	}
	var p oidcPending
	if err := json.Unmarshal(raw, &p); err != nil {
		return oidcPending{}, err
	}
	if time.Now().Unix() > p.Exp {
		return oidcPending{}, errors.New("expired")
	}
	return p, nil
}

func splitDot(s string) (string, string, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

func (a *App) setOIDCCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.CookieName("rk_oidc"),
		Value:    value,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) clearOIDCCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.CookieName("rk_oidc"),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}
