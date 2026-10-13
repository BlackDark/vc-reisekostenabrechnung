package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doCookie(t *testing.T, h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.RemoteAddr = "192.0.2.10:1234"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMailErlaubtAPI(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()

	if c := doCookie(t, h, http.MethodGet, "/api/v1/me/mail/erlaubt", "", nil); c.Code != http.StatusUnauthorized {
		t.Fatalf("anon %d", c.Code)
	}
	cookie := loginCookie(t, h)

	if c := doCookie(t, h, http.MethodPost, "/api/v1/me/mail/erlaubt", `{"wert":"nope"}`, cookie); c.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid wert %d %s", c.Code, c.Body)
	}
	if c := doCookie(t, h, http.MethodPost, "/api/v1/me/mail/erlaubt", `{"wert":"Hotel@Example.com"}`, cookie); c.Code != http.StatusCreated ||
		!strings.Contains(c.Body.String(), `"hotel@example.com"`) {
		t.Fatalf("add address %d %s", c.Code, c.Body)
	}
	if c := doCookie(t, h, http.MethodPost, "/api/v1/me/mail/erlaubt", `{"wert":"Hotel@Example.com"}`, cookie); c.Code != http.StatusConflict {
		t.Fatalf("duplicate %d", c.Code)
	}
	if c := doCookie(t, h, http.MethodPost, "/api/v1/me/mail/erlaubt", `{"wert":"@Booking.com"}`, cookie); c.Code != http.StatusCreated {
		t.Fatalf("add domain %d", c.Code)
	}
	list := doCookie(t, h, http.MethodGet, "/api/v1/me/mail/erlaubt", "", cookie)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "@booking.com") {
		t.Fatalf("list %d %s", list.Code, list.Body)
	}
	if c := doCookie(t, h, http.MethodDelete, "/api/v1/me/mail/erlaubt/%40booking.com", "", cookie); c.Code != http.StatusNoContent {
		t.Fatalf("delete %d", c.Code)
	}
	if c := doCookie(t, h, http.MethodDelete, "/api/v1/me/mail/erlaubt/%40booking.com", "", cookie); c.Code != http.StatusNotFound {
		t.Fatalf("delete again %d", c.Code)
	}
}
