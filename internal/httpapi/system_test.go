package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProtokollOwn(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()

	if code := doJSON(t, h, http.MethodGet, "/api/v1/protokoll", "", nil).Code; code != http.StatusUnauthorized {
		t.Fatalf("anon %d", code)
	}

	login := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if login.Code != http.StatusOK {
		t.Fatal(login.Body)
	}
	cookie := login.Result().Cookies()[0]

	patch := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"sprache":"en"}`))
	patch.RemoteAddr = "192.0.2.10:1"
	patch.Header.Set("Content-Type", "application/json")
	patch.Header.Set("If-Match", login.Header().Get("ETag"))
	patch.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, patch)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch me %d %s", rec.Code, rec.Body)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/protokoll", nil)
	get.RemoteAddr = "192.0.2.10:1"
	get.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, get)
	if rec.Code != http.StatusOK {
		t.Fatalf("protokoll %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Items []struct {
			ID        string    `json:"id"`
			Zeitpunkt time.Time `json:"zeitpunkt"`
			Aktion    string    `json:"aktion"`
			ObjektTyp string    `json:"objekt_typ"`
			ObjektID  string    `json:"objekt_id"`
			AkteurArt string    `json:"akteur_art"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	saw := false
	for _, it := range out.Items {
		if it.ID == "" {
			t.Fatalf("event without an id: %s", rec.Body)
		}
		if it.AkteurArt != "nutzer" {
			t.Fatalf("unexpected actor %q in own protokoll: %s", it.AkteurArt, rec.Body)
		}
		if it.Zeitpunkt.Location() != time.UTC {
			t.Fatalf("event %s reported in %s, want UTC", it.Aktion, it.Zeitpunkt.Location())
		}
		if it.Aktion == "nutzer.profil" {
			saw = true
			if it.ObjektTyp != "nutzer" || it.ObjektID != me.ID {
				t.Fatalf("profile event names %s/%s, want nutzer/%s", it.ObjektTyp, it.ObjektID, me.ID)
			}
		}
	}
	if !saw {
		t.Fatalf("own protokoll missing nutzer.profil: %s", rec.Body)
	}
}

func TestAdminSystem(t *testing.T) {
	app := newTestApp(t, nil)
	app.Version = "1.2.3"
	seedUser(t, app, "root", "correct-horse-1", true)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()

	if code := doJSON(t, h, http.MethodGet, "/api/v1/admin/system", "", nil).Code; code != http.StatusUnauthorized {
		t.Fatalf("anon %d", code)
	}

	ada := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	forbidden := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system", nil)
	forbidden.RemoteAddr = "192.0.2.10:1"
	forbidden.AddCookie(ada.Result().Cookies()[0])
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, forbidden)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin %d %s", rec.Code, rec.Body)
	}

	root := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"root","passwort":"correct-horse-1"}`, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system", nil)
	req.RemoteAddr = "192.0.2.10:1"
	req.AddCookie(root.Result().Cookies()[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin %d %s", rec.Code, rec.Body)
	}
	var s struct {
		Version        string `json:"version"`
		StorageBackend string `json:"storage_backend"`
		DbSizeBytes    int64  `json:"db_size_bytes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.Version != "1.2.3" || s.StorageBackend != "local" || s.DbSizeBytes < 0 {
		t.Fatalf("system status %+v", s)
	}
}
