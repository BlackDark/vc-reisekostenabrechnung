package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func createTokenFor(t *testing.T, h http.Handler, cookie *http.Cookie, name string) (token, id string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", strings.NewReader(`{"name":"`+name+`"}`))
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Token string `json:"token"`
		Id    string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Token, out.Id
}

func postBelegeBearer(t *testing.T, h http.Handler, token string, jpg []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("datei", "s.jpg")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(jpg)
	_ = mw.WriteField("profil", "a4")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/belege", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = "192.0.2.10:1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPITokenUpload(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()

	if code := doJSON(t, h, http.MethodGet, "/api/v1/me/tokens", "", nil).Code; code != http.StatusUnauthorized {
		t.Fatalf("anon list %d", code)
	}
	cookie := loginCookie(t, h)
	token, id := createTokenFor(t, h, cookie, "scanner")
	if token == "" || id == "" {
		t.Fatal("missing token or id")
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/me/tokens", nil)
	list.RemoteAddr = "192.0.2.10:1234"
	list.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, list)
	if rec.Code != http.StatusOK {
		t.Fatalf("list %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), token) {
		t.Fatalf("list leaks the token: %s", rec.Body)
	}

	jpg := jpegBytesMarked(t, 1)
	if r := postBelegeBearer(t, h, token, jpg); r.Code != http.StatusAccepted {
		t.Fatalf("bearer upload %d %s", r.Code, r.Body)
	}
	if r := postBelegeBearer(t, h, "not-a-real-token", jpg); r.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token should be 401, got %d", r.Code)
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/v1/me/tokens/"+id, nil)
	del.RemoteAddr = "192.0.2.10:1234"
	del.AddCookie(cookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, del)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke %d %s", rec.Code, rec.Body)
	}
	if r := postBelegeBearer(t, h, token, jpg); r.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token should be 401, got %d", r.Code)
	}
}
