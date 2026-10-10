package belegpipe

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalizeLiftsShadow(t *testing.T) {
	const w, h = 100, 40
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := src.PixOffset(x, y)
			v := byte(220)
			if x < 20 {
				v = 80
			}
			if x == 60 {
				v = 0
			}
			src.Pix[i] = v
			src.Pix[i+1] = v
			src.Pix[i+2] = v
			src.Pix[i+3] = 255
		}
	}
	out := Normalize(src)
	again := Normalize(src)
	if !bytes.Equal(out.Pix, again.Pix) {
		t.Fatal("normalize is not deterministic")
	}
	if out.Pix[out.PixOffset(10, 20)] < 180 {
		t.Fatalf("shadow stayed dark: %d", out.Pix[out.PixOffset(10, 20)])
	}
	if out.Pix[out.PixOffset(60, 20)] > 80 {
		t.Fatalf("ink was erased: %d", out.Pix[out.PixOffset(60, 20)])
	}
}

func TestProcessTinyAVIF(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 16, 8))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i] = 240
		src.Pix[i+1] = 240
		src.Pix[i+2] = 230
		src.Pix[i+3] = 255
	}
	photo, err := Process(src, Settings{Format: "avif", AVIFQuality: 40, AVIFSpeed: 10, JPEGQuality: 70, PreviewWebP: 55})
	if err != nil {
		t.Fatal(err)
	}
	if photo.ArchivExt != "avif" || !bytes.Contains(photo.Archiv[:12], []byte("ftyp")) {
		t.Fatalf("archiv %q %x", photo.ArchivExt, photo.Archiv[:12])
	}
	if len(photo.Archiv) > 100*1024 {
		t.Fatalf("archiv %d bytes", len(photo.Archiv))
	}
	if !bytes.HasPrefix(photo.Preview, []byte("RIFF")) || !bytes.HasPrefix(photo.ExportJPEG, []byte{0xFF, 0xD8}) {
		t.Fatal("derivatives")
	}
	if !bytes.HasPrefix(photo.Bild, []byte("RIFF")) || bytes.Equal(photo.Bild, photo.Preview) {
		t.Fatalf("bild %d bytes", len(photo.Bild))
	}
	again, err := Process(src, Settings{Format: "avif", AVIFQuality: 40, AVIFSpeed: 10, JPEGQuality: 70, PreviewWebP: 55})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(photo.Archiv, again.Archiv) || !bytes.Equal(photo.Bild, again.Bild) {
		t.Fatal("encode is not deterministic")
	}
}

func TestNormalizeSettingsDefaults(t *testing.T) {
	s := NormalizeSettings(Settings{})
	if s.AVIFQuality != 70 || s.AVIFSpeed != 8 || s.Format != "avif" {
		t.Fatalf("defaults %+v", s)
	}
	if s.JPEGQuality != 70 || s.PreviewWebP != 55 || s.WebPQuality != 55 {
		t.Fatalf("derivatives %+v", s)
	}
}

func TestEncodePreviewJPEGUsesConfigQuality(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 200, 300))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i] = 250
		src.Pix[i+1] = 248
		src.Pix[i+2] = 240
		src.Pix[i+3] = 255
	}
	preview, bild, export, err := EncodePreviewJPEG(src, 30, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(preview, []byte("RIFF")) || !bytes.HasPrefix(bild, []byte("RIFF")) {
		t.Fatal("webp derivatives")
	}
	if !bytes.HasPrefix(export, []byte{0xFF, 0xD8}) {
		t.Fatal("jpeg export")
	}
	// BELEG_JPEG_QUALITY must win over the default 70 in the PDF path too.
	_, _, lowJPG, err := EncodePreviewJPEG(src, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(lowJPG) >= len(export) {
		t.Fatalf("jpeg quality ignored %d >= %d", len(lowJPG), len(export))
	}
	lowerPreview, _, _, err := EncodePreviewJPEG(src, 30, 20)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(lowerPreview, preview) {
		t.Fatal("preview quality ignored")
	}
}

func TestNormalizeDeterministic(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			i := src.PixOffset(x, y)
			src.Pix[i] = byte(150 + x%37 + y%11)
			src.Pix[i+1] = byte(140 + y%23)
			src.Pix[i+2] = byte(120 + (x*y)%19)
			src.Pix[i+3] = 255
		}
	}
	first := Normalize(src)
	second := Normalize(src)
	if !bytes.Equal(first.Pix, second.Pix) {
		t.Fatal("normalize is not deterministic")
	}
}

func TestProcessWebPFallback(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = 255
	}
	photo, err := Process(src, Settings{Format: "webp", WebPQuality: 55, AVIFSpeed: 10})
	if err != nil {
		t.Fatal(err)
	}
	if photo.ArchivMIME != "image/webp" || !bytes.HasPrefix(photo.Archiv, []byte("RIFF")) {
		t.Fatalf("%s %x", photo.ArchivMIME, photo.Archiv[:8])
	}
}

func TestPixelBombRejected(t *testing.T) {
	png := craftPNG(100000, 100000)
	_, _, err := DecodeLimited(bytes.NewReader(png), MaxPixels)
	if err == nil {
		t.Fatal("bomb decoded")
	}
}

func TestPDFAndXML(t *testing.T) {
	pdf := []byte("%PDF-1.1\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Count 1/Kids[3 0 R]>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 3 3]>>endobj\ntrailer<</Root 1 0 R>>\n%%EOF\n")
	n, err := PDFPages(pdf)
	if err != nil || n != 1 {
		t.Fatalf("pages %d %v", n, err)
	}
	if !PDFHybrid([]byte("%PDF-1.1 factur-x %%EOF")) {
		t.Fatal("hybrid")
	}
	many := append([]byte("%PDF-1.1\n"), bytes.Repeat([]byte("/Type /Page\n"), 51)...)
	many = append(many, []byte("%%EOF")...)
	if _, err := PDFPages(many); err == nil {
		t.Fatal("page limit")
	}
	root, err := XMLRoot([]byte(`<?xml version="1.0"?><Invoice xmlns="urn:example"></Invoice>`))
	if err != nil || root != "Invoice" {
		t.Fatalf("%s %v", root, err)
	}
	if _, err := XMLRoot([]byte(`<?xml version="1.0"?><note></note>`)); err == nil {
		t.Fatal("root")
	}
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()
	xxe := `<?xml version="1.0"?><!DOCTYPE Invoice [<!ENTITY xxe SYSTEM "` + srv.URL + `">]><Invoice>&xxe;</Invoice>`
	if _, err := XMLRoot([]byte(xxe)); err == nil {
		t.Fatal("xxe accepted")
	}
	if hits != 0 {
		t.Fatalf("external entity fetched %d", hits)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if Sniff(buf.Bytes()) != "png" {
		t.Fatal(Sniff(buf.Bytes()))
	}
}

func craftPNG(w, h uint32) []byte {
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], w)
	binary.BigEndian.PutUint32(data[4:8], h)
	data[8] = 8
	data[9] = 2
	sig := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	out := append(sig, pngChunk("IHDR", data)...)
	return append(out, pngChunk("IEND", nil)...)
}

func pngChunk(typ string, data []byte) []byte {
	buf := make([]byte, 8+len(data)+4)
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(data)))
	copy(buf[4:8], typ)
	copy(buf[8:], data)
	sum := crc32.ChecksumIEEE(buf[4 : 8+len(data)])
	binary.BigEndian.PutUint32(buf[8+len(data):], sum)
	return buf
}
