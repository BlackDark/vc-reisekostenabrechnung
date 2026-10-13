package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
)

func createTokenScope(t *testing.T, h http.Handler, cookie *http.Cookie, name, scope string) string {
	t.Helper()
	body := `{"name":"` + name + `","scope":"` + scope + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/tokens", strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %s token %d %s", scope, rec.Code, rec.Body)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Token
}

func buildMail(t *testing.T, jpg []byte) []byte {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.SetBoundary("RKMAILBOUNDARY")

	textHeader := make(textproto.MIMEHeader)
	textHeader.Set("Content-Type", "text/plain")
	pw, err := mw.CreatePart(textHeader)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pw.Write([]byte("Receipt attached."))

	attachHeader := make(textproto.MIMEHeader)
	attachHeader.Set("Content-Disposition", `attachment; filename="receipt.jpg"`)
	attachHeader.Set("Content-Type", "image/jpeg")
	pa, err := mw.CreatePart(attachHeader)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = pa.Write(jpg)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	head := "From: sender@example.com\r\n" +
		"To: belege+token@example.com\r\n" +
		"Subject: Your receipt\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: multipart/mixed; boundary=RKMAILBOUNDARY\r\n\r\n"
	return append([]byte(head), body.Bytes()...)
}

func postMailBearer(t *testing.T, h http.Handler, token string, raw []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/eingang/mail", bytes.NewReader(raw))
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("Content-Type", "message/rfc822")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMailIntakeFilesIntoBelegeingang(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", false)
	h := app.Handler()
	cookie := loginCookie(t, h)
	jpg := jpegBytesMarked(t, 7)
	raw := buildMail(t, jpg)

	if r := postMailBearer(t, h, "", raw); r.Code != http.StatusUnauthorized {
		t.Fatalf("no auth %d", r.Code)
	}

	ingestToken := createTokenScope(t, h, cookie, "inbox", "mail:ingest")
	rec := postMailBearer(t, h, ingestToken, raw)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("ingest %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Items []struct {
			Id string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 1 {
		t.Fatalf("expected one receipt from the attachment, got %d: %s", len(out.Items), rec.Body)
	}
	createdID := out.Items[0].Id

	list := httptest.NewRequest(http.MethodGet, "/api/v1/belege", nil)
	list.RemoteAddr = "192.0.2.10:1234"
	list.AddCookie(cookie)
	lrec := httptest.NewRecorder()
	h.ServeHTTP(lrec, list)
	if !strings.Contains(lrec.Body.String(), createdID) {
		t.Fatalf("created receipt not in owner's list: %s", lrec.Body)
	}

	// Resend: the same attachment is a duplicate and is not filed again.
	again := postMailBearer(t, h, ingestToken, raw)
	if again.Code != http.StatusAccepted {
		t.Fatalf("resend %d %s", again.Code, again.Body)
	}
	_ = json.Unmarshal(again.Body.Bytes(), &out)
	if len(out.Items) != 0 {
		t.Fatalf("duplicate mail should file nothing, got %d", len(out.Items))
	}

	// Scope isolation: a plain upload token cannot file mail, and vice versa.
	uploadToken := createTokenScope(t, h, cookie, "uploader", "belege:upload")
	if r := postMailBearer(t, h, uploadToken, raw); r.Code != http.StatusUnauthorized {
		t.Fatalf("upload token on ingest endpoint %d", r.Code)
	}
}
