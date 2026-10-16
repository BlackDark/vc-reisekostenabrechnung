package belegpipe

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// typstPath mirrors the skip convention of the export tests: CI downloads the
// pinned Typst binary and exports TYPST_PATH for it.
func typstPath(t *testing.T) string {
	t.Helper()
	path := os.Getenv("TYPST_PATH")
	if path == "" {
		path = "/usr/local/bin/typst"
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("typst not installed")
	}
	return path
}

// pageBoxPDF builds a PDF whose pages have different media boxes, in the order
// given. The rendered shape of each page therefore identifies which page it came
// from, which is how the caller's page order is checked.
func pageBoxPDF(boxes ...string) []byte {
	var b strings.Builder
	b.WriteString("%PDF-1.1\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	fmt.Fprintf(&b, "2 0 obj<</Type/Pages/Count %d/Kids[", len(boxes))
	for i := range boxes {
		fmt.Fprintf(&b, "%d 0 R ", 3+i)
	}
	b.WriteString("]>>endobj\n")
	for i, box := range boxes {
		fmt.Fprintf(&b, "%d 0 obj<</Type/Page/Parent 2 0 R/MediaBox[%s]>>endobj\n", 3+i, box)
	}
	b.WriteString("trailer<</Root 1 0 R>>\n%%EOF\n")
	return []byte(b.String())
}

// Rasterising at 300 ppi scales the media boxes, so each page is described by its
// shape rather than by its exact pixel size.
func assertShape(t *testing.T, page int, body []byte) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("page %d: %v", page, err)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	want := map[int]string{0: "square", 1: "wide", 2: "tall"}[(page-1)%3]
	ok := map[string]bool{
		"square": w == h,
		"wide":   w > 5*h,
		"tall":   h > 5*w,
	}[want]
	if !ok {
		t.Fatalf("page %d rendered %dx%d, which is not the %s page: the pages came back out of order", page, w, h, want)
	}
}

// The loader renders a batch of pages per typst run, and a receipt may carry more
// pages than one batch. Ten pages cross that boundary with a full batch and a
// short one, and each page must still arrive once, in order, under its own number.
func TestRenderPDFPagesKeepsPageOrderAcrossBatches(t *testing.T) {
	boxes := make([]string, 10)
	for i := range boxes {
		boxes[i] = []string{"0 0 30 30", "0 0 90 10", "0 0 10 90"}[i%3]
	}
	var seen []int
	err := RenderPDFPages(context.Background(), typstPath(t), pageBoxPDF(boxes...), len(boxes), func(page int, png []byte) error {
		seen = append(seen, page)
		assertShape(t, page, png)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != len(boxes) {
		t.Fatalf("loaded %d pages, want %d", len(seen), len(boxes))
	}
	for i, page := range seen {
		if page != i+1 {
			t.Fatalf("page %d arrived as number %d", i+1, page)
		}
	}
}

func TestRenderPDFPagesRejectsAnEmptyDocument(t *testing.T) {
	err := RenderPDFPages(context.Background(), typstPath(t), pageBoxPDF("0 0 30 30"), 0, func(int, []byte) error { return nil })
	if err == nil {
		t.Fatal("want an error for a document without pages")
	}
}

func TestRenderPDFPagesReportsATypstFailure(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no shell")
	}
	broken := t.TempDir() + "/typst"
	if err := os.WriteFile(broken, []byte("#!/bin/sh\necho 'broken' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := RenderPDFPages(context.Background(), broken, pageBoxPDF("0 0 30 30"), 1, func(int, []byte) error { return nil })
	if err == nil {
		t.Fatal("want the typst output reported as an error")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("error %q does not carry typst's own message", err)
	}
}

func TestRenderPDFPagesStopsAtTheFirstCallbackError(t *testing.T) {
	boxes := make([]string, 10)
	for i := range boxes {
		boxes[i] = "0 0 30 30"
	}
	want := fmt.Errorf("caller stopped")
	got := 0
	err := RenderPDFPages(context.Background(), typstPath(t), pageBoxPDF(boxes...), len(boxes), func(page int, png []byte) error {
		got++
		if page == 3 {
			return want
		}
		return nil
	})
	if err != want {
		t.Fatalf("error %v, want the callback's own error", err)
	}
	if got != 3 {
		t.Fatalf("callback ran %d times, want 3", got)
	}
}
