package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/config"
	"github.com/BlackDark/vc-reisekostenabrechnung/internal/storage"
)

func TestBelegPhotoLifecycle(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.BelegFormat = "avif"
		cfg.BelegAVIFQuality = 40
		cfg.BelegAVIFSpeed = 10
		cfg.BelegJPEGQuality = 70
	})
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	cookie := loginCookie(t, h)

	body := jpegBytes(t, 16, 8)
	res := uploadBeleg(t, h, cookie, body, "seite.jpg", "image/jpeg", false)
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload %d %s", res.Code, res.Body)
	}
	var first map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	id, _ := first["id"].(string)
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := getBeleg(t, h, cookie, id)
	if got["status"] != "zur_bestaetigung" || got["pipeline_version"] != "2026.2" {
		t.Fatalf("%v", got)
	}
	param := ""
	if row, err := app.store.GetBelegByID(t.Context(), id); err == nil {
		param = row.PipelineParameter
	}
	if !strings.Contains(param, `"avif_quality":40`) || !strings.Contains(param, `"avif_speed":10`) {
		t.Fatalf("settings not from config: %s", param)
	}
	prev := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/vorschau", nil, "")
	if prev.Code != http.StatusOK || !bytes.HasPrefix(prev.Body.Bytes(), []byte("RIFF")) {
		t.Fatalf("preview %d %x", prev.Code, prev.Body.Bytes())
	}
	bild := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/bild", nil, "")
	if bild.Code != http.StatusOK || !bytes.HasPrefix(bild.Body.Bytes(), []byte("RIFF")) {
		t.Fatalf("bild %d %x", bild.Code, bild.Body.Bytes())
	}
	orig := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/original", nil, "")
	if orig.Code != http.StatusOK || !bytes.HasPrefix(orig.Body.Bytes(), []byte{0xFF, 0xD8}) {
		t.Fatalf("original %d", orig.Code)
	}
	dup := uploadBeleg(t, h, cookie, body, "seite.jpg", "image/jpeg", false)
	if dup.Code != http.StatusConflict || !strings.Contains(dup.Body.String(), "duplikat") || !strings.Contains(dup.Body.String(), id) {
		t.Fatalf("dup %d %s", dup.Code, dup.Body)
	}
	again := uploadBeleg(t, h, cookie, body, "seite.jpg", "image/jpeg", true)
	if again.Code != http.StatusAccepted {
		t.Fatalf("dup accept %d %s", again.Code, again.Body)
	}
	ok := authed(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/bestaetigen", nil, etagOf(got))
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "belegnummer") {
		t.Fatalf("confirm %d %s", ok.Code, ok.Body)
	}
	del := authed(t, h, cookie, http.MethodDelete, "/api/v1/belege/"+id, nil, ok.Header().Get("ETag"))
	if del.Code != http.StatusConflict {
		t.Fatalf("delete confirmed %d %s", del.Code, del.Body)
	}
	files, err := app.store.ListBelegdateien(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	var key string
	for _, f := range files {
		if f.Variante == "archiv" && f.Unveraenderbar {
			key = f.SpeicherSchluessel
		}
	}
	if key == "" {
		t.Fatal("missing archiv")
	}
	if err := app.blobs.PutIfAbsent(t.Context(), key, bytes.NewReader([]byte("nope")), ""); err == nil || !errorsIsStorage(err) {
		t.Fatalf("overwrite %v", err)
	}
	warn := authed(t, h, cookie, http.MethodGet, "/api/v1/warnungen", nil, "")
	if !strings.Contains(warn.Body.String(), "W04") {
		t.Fatalf("warn %s", warn.Body)
	}
}

func TestBelegPDFAndXMLUnchanged(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	cookie := loginCookie(t, h)
	pdf := []byte("%PDF-1.1\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 3 3]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")
	res := uploadBeleg(t, h, cookie, pdf, "rechnung.pdf", "application/pdf", false)
	if res.Code != http.StatusAccepted {
		t.Fatalf("pdf %d %s", res.Code, res.Body)
	}
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &doc)
	id, _ := doc["id"].(string)
	got := getBeleg(t, h, cookie, id)
	if got["status"] != "zur_bestaetigung" || got["typ"] != "pdf" {
		t.Fatalf("%v", got)
	}
	orig := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/original", nil, "")
	if !bytes.Equal(orig.Body.Bytes(), pdf) {
		t.Fatalf("pdf changed %d", orig.Code)
	}
	xml := []byte(`<?xml version="1.0"?><Invoice xmlns="urn:example"></Invoice>`)
	xres := uploadBeleg(t, h, cookie, xml, "rechnung.xml", "application/xml", false)
	if xres.Code != http.StatusAccepted || !strings.Contains(xres.Body.String(), "e_rechnung_xml") {
		t.Fatalf("xml %d %s", xres.Code, xres.Body)
	}
	var xdoc map[string]any
	_ = json.Unmarshal(xres.Body.Bytes(), &xdoc)
	xid, _ := xdoc["id"].(string)
	if xdoc["status"] != "zur_bestaetigung" {
		t.Fatalf("%v", xdoc)
	}
	xorig := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+xid+"/original", nil, "")
	if !bytes.Equal(xorig.Body.Bytes(), xml) {
		t.Fatal("xml changed")
	}
	bad := uploadBeleg(t, h, cookie, []byte("hello"), "note.txt", "text/plain", false)
	if bad.Code != http.StatusUnprocessableEntity {
		t.Fatalf("text %d %s", bad.Code, bad.Body)
	}
}

func TestBelegBildFallbackOhneBilddatei(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.BelegFormat = "avif"
		cfg.BelegAVIFSpeed = 10
	})
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	cookie := loginCookie(t, h)
	res := uploadBeleg(t, h, cookie, jpegBytes(t, 16, 8), "seite.jpg", "image/jpeg", false)
	if res.Code != http.StatusAccepted {
		t.Fatalf("upload %d %s", res.Code, res.Body)
	}
	var doc map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &doc)
	id, _ := doc["id"].(string)
	if _, err := app.ProcessNext(t.Context()); err != nil {
		t.Fatal(err)
	}
	// Belege archived with pipeline 2026.1 have no display rendition.
	db, err := sql.Open("sqlite", app.cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(t.Context(),
		"DELETE FROM belegdatei WHERE variante = 'bild'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(),
		"UPDATE beleg SET pipeline_version = '2026.1'"); err != nil {
		t.Fatal(err)
	}
	bild := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/bild", nil, "")
	if bild.Code != http.StatusOK {
		t.Fatalf("bild %d %s", bild.Code, bild.Body)
	}
	if bild.Header().Get("Content-Type") != "image/avif" || !bytes.Contains(bild.Body.Bytes(), []byte("ftyp")) {
		t.Fatalf("no archiv fallback: %s %x", bild.Header().Get("Content-Type"), bild.Body.Bytes()[:8])
	}
	seite := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/seiten/9", nil, "")
	if seite.Code != http.StatusNotFound {
		t.Fatalf("unknown page %d", seite.Code)
	}
}

func TestBelegBildOhneAbleitung404(t *testing.T) {
	app := newTestApp(t, nil)
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	cookie := loginCookie(t, h)
	res := uploadBeleg(t, h, cookie, jpegBytes(t, 16, 8), "seite.jpg", "image/jpeg", false)
	var doc map[string]any
	_ = json.Unmarshal(res.Body.Bytes(), &doc)
	id, _ := doc["id"].(string)
	bild := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id+"/bild", nil, "")
	if bild.Code != http.StatusNotFound {
		t.Fatalf("bild without derivatives %d %s", bild.Code, bild.Body)
	}
}

func TestBelegNumbersGapless(t *testing.T) {
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.BelegFormat = "webp"
		cfg.BelegWebPQuality = 40
	})
	seedUser(t, app, "ada", "correct-horse-1", true)
	h := app.Handler()
	cookie := loginCookie(t, h)
	var ids []string
	var etags []string
	for i := range 2 {
		res := uploadBeleg(t, h, cookie, jpegBytesMarked(t, i), "a.jpg", "image/jpeg", false)
		if res.Code != http.StatusAccepted {
			t.Fatalf("upload %d %s", res.Code, res.Body)
		}
		var doc map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &doc)
		id, _ := doc["id"].(string)
		ids = append(ids, id)
		if _, err := app.ProcessNext(t.Context()); err != nil {
			t.Fatal(err)
		}
		got := getBeleg(t, h, cookie, id)
		etags = append(etags, etagOf(got))
	}
	var numbers []string
	for i, id := range ids {
		res := authed(t, h, cookie, http.MethodPost, "/api/v1/belege/"+id+"/bestaetigen", nil, etags[i])
		if res.Code != http.StatusOK {
			t.Fatalf("confirm %d %s", res.Code, res.Body)
		}
		var doc map[string]any
		_ = json.Unmarshal(res.Body.Bytes(), &doc)
		num, _ := doc["belegnummer"].(string)
		numbers = append(numbers, num)
	}
	if len(numbers) != 2 || numbers[0] == numbers[1] || !strings.HasSuffix(numbers[0], "-0001") || !strings.HasSuffix(numbers[1], "-0002") {
		t.Fatalf("numbers %v", numbers)
	}
}

func loginCookie(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	res := doJSON(t, h, http.MethodPost, "/api/v1/auth/login", `{"benutzername":"ada","passwort":"correct-horse-1"}`, nil)
	if res.Code != http.StatusOK || len(res.Result().Cookies()) == 0 {
		t.Fatalf("login %d %s", res.Code, res.Body)
	}
	return res.Result().Cookies()[0]
}

func jpegBytesMarked(t *testing.T, mark int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = byte(200 + mark)
		img.Pix[i+1] = 250
		img.Pix[i+2] = 245
		img.Pix[i+3] = 255
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, hgt int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, hgt))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = 250
		img.Pix[i+1] = 250
		img.Pix[i+2] = 245
		img.Pix[i+3] = 255
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func uploadBeleg(t *testing.T, h http.Handler, cookie *http.Cookie, body []byte, name, mime string, dup bool) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("datei", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := mw.WriteField("profil", "bon"); err != nil {
		t.Fatal(err)
	}
	if dup {
		if err := mw.WriteField("duplikat_bestaetigt", "true"); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/belege", &buf)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func authed(t *testing.T, h http.Handler, cookie *http.Cookie, method, path string, body []byte, etag string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.RemoteAddr = "192.0.2.10:1234"
	req.AddCookie(cookie)
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	if method == http.MethodPost && body == nil && strings.Contains(path, "stornieren") {
		req.Header.Set("Content-Type", "application/json")
		req.Body = ioNopCloser(strings.NewReader(`{"grund":"falsch"}`))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func getBeleg(t *testing.T, h http.Handler, cookie *http.Cookie, id string) map[string]any {
	t.Helper()
	res := authed(t, h, cookie, http.MethodGet, "/api/v1/belege/"+id, nil, "")
	if res.Code != http.StatusOK {
		t.Fatalf("get %d %s", res.Code, res.Body)
	}
	var doc map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	doc["_etag"] = res.Header().Get("ETag")
	return doc
}

func etagOf(doc map[string]any) string {
	s, _ := doc["_etag"].(string)
	return s
}

func errorsIsStorage(err error) bool {
	return err != nil && (err == storage.ErrExists || strings.Contains(err.Error(), "exists"))
}

type readerNop struct{ r *strings.Reader }

func (r readerNop) Read(p []byte) (int, error) { return r.r.Read(p) }
func (r readerNop) Close() error               { return nil }

func ioNopCloser(r *strings.Reader) readerNop { return readerNop{r: r} }
