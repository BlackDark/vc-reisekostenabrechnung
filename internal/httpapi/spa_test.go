package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	h := spaHandler(dist)
	cases := []struct {
		method, path string
		code         int
		body         string
	}{
		{http.MethodGet, "/", http.StatusOK, "app"},
		{http.MethodGet, "/assets/app.js", http.StatusOK, "console.log"},
		{http.MethodGet, "/reisen/123", http.StatusOK, "app"},
		{http.MethodGet, "/assets", http.StatusOK, "app"},
		{http.MethodPost, "/", http.StatusNotFound, ""},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) {
			t.Errorf("%s %s: %d %q", c.method, c.path, rec.Code, rec.Body)
		}
	}
}

func TestSPAHandlerWithoutBuild(t *testing.T) {
	rec := httptest.NewRecorder()
	spaHandler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}
