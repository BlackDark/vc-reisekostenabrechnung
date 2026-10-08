package httpapi

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
)

func TestOIDCLoginPKCE(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var gotChallenge, gotVerifier, gotNonce string
	var issuer string
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                issuer,
				"authorization_endpoint":                issuer + "/authorize",
				"token_endpoint":                        issuer + "/token",
				"jwks_uri":                              issuer + "/jwks",
				"response_types_supported":              []string{"code"},
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
				"code_challenge_methods_supported":      []string{"S256"},
			})
		case "/jwks":
			n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
			e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]string{{
					"kty": "RSA", "kid": "test", "alg": "RS256", "use": "sig", "n": n, "e": e,
				}},
			})
		case "/authorize":
			q := r.URL.Query()
			mu.Lock()
			gotChallenge = q.Get("code_challenge")
			gotNonce = q.Get("nonce")
			mu.Unlock()
			if q.Get("code_challenge_method") != "S256" {
				t.Errorf("challenge method %s", q.Get("code_challenge_method"))
			}
			dest, _ := url.Parse(q.Get("redirect_uri"))
			dq := dest.Query()
			dq.Set("code", "authcode")
			dq.Set("state", q.Get("state"))
			dest.RawQuery = dq.Encode()
			http.Redirect(w, r, dest.String(), http.StatusFound)
		case "/token":
			_ = r.ParseForm()
			mu.Lock()
			gotVerifier = r.Form.Get("code_verifier")
			nonce := gotNonce
			mu.Unlock()
			claims := jwt.MapClaims{
				"iss": issuer, "sub": "user-1", "aud": "reisekosten",
				"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
				"nonce": nonce, "email": "ada@example.com", "email_verified": true,
				"name": "Ada Lovelace", "preferred_username": "ada",
				"groups": []string{"admins"},
			}
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			tok.Header["kid"] = "test"
			signed, err := tok.SignedString(key)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "atk", "token_type": "Bearer", "id_token": signed,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer idp.Close()
	issuer = idp.URL

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	appBase := "http://" + ln.Addr().String()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.AppBaseURL = appBase
		cfg.OIDC.Enabled = true
		cfg.OIDC.IssuerURL = issuer
		cfg.OIDC.ClientID = "reisekosten"
		cfg.OIDC.Scopes = []string{"openid", "profile", "email", "groups"}
		cfg.OIDC.GroupsClaim = "groups"
		cfg.OIDC.AdminGroup = "admins"
		cfg.OIDC.AllowedGroup = "users"
	})
	srv := &http.Server{Handler: app.Handler()}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	res, err := client.Get(appBase + "/api/v1/auth/oidc/start")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("final %d", res.StatusCode)
	}
	me, err := client.Get(appBase + "/api/v1/auth/me")
	if err != nil {
		t.Fatal(err)
	}
	defer me.Body.Close()
	var body struct {
		Benutzername string `json:"benutzername"`
		IstAdmin     bool   `json:"ist_admin"`
	}
	if err := json.NewDecoder(me.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if me.StatusCode != http.StatusOK || body.Benutzername != "ada" || !body.IstAdmin {
		t.Fatalf("me %d %+v", me.StatusCode, body)
	}
	mu.Lock()
	defer mu.Unlock()
	sum := sha256.Sum256([]byte(gotVerifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if gotChallenge == "" || gotChallenge != want {
		t.Fatalf("pkce challenge %q verifier %q", gotChallenge, gotVerifier)
	}
	if !strings.Contains(gotNonce, "") || gotNonce == "" {
		t.Fatal("missing nonce")
	}
}
