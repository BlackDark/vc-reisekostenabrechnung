package belegpipe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// pdfBatchPages bounds how many pages one typst run rasterises. A dense A4 page
// is about 1.8MB at 300 ppi, so a receipt at MaxPDFPages would leave some 90MB of
// page images behind if every page were rendered at once, against roughly one
// page before. Eight pages keep the batch near the footprint the pipeline already
// accepts for a single decoded image.
const pdfBatchPages = 8

// RenderPDFPages rasterises every page of a PDF and hands each page image to fn
// in page order, page being 1-based. It stops at the first error fn returns and
// returns that error untouched.
//
// Rendering page by page repeated three things per page: a temp directory, a full
// copy of the PDF and a process spawn. One run now covers a batch of pages, which
// measured at 175ms per page against 306ms on a ten-page receipt. The command
// stops when ctx is cancelled, and after 60 seconds per page of the batch either
// way, the allowance a page had when it was rendered on its own.
func RenderPDFPages(ctx context.Context, typstPath string, pdf []byte, pages int, fn func(page int, png []byte) error) error {
	if pages < 1 {
		return fmt.Errorf("pdf pages")
	}
	dir, err := os.MkdirTemp("", "rk-pdf-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "beleg.pdf"), pdf, 0o600); err != nil {
		return err
	}
	for first := 1; first <= pages; first += pdfBatchPages {
		last := min(first+pdfBatchPages-1, pages)
		if err := rasteriseBatch(ctx, typstPath, dir, first, last, fn); err != nil {
			return err
		}
	}
	return nil
}

// rasteriseBatch renders the pages from first to last in one typst run and hands
// them over one at a time. Each page image is removed once fn has seen it, so the
// batch never leaves more than one batch of images on disk.
func rasteriseBatch(ctx context.Context, typstPath, dir string, first, last int, fn func(page int, png []byte) error) error {
	count := last - first + 1
	var src strings.Builder
	src.WriteString("#set page(width: auto, height: auto, margin: 0pt)\n")
	fmt.Fprintf(&src, "#for p in range(%d) {\n  image(\"beleg.pdf\", page: p + %d)\n  pagebreak(weak: true)\n}\n", count, first)
	in := filepath.Join(dir, "seiten.typ")
	if err := os.WriteFile(in, []byte(src.String()), 0o600); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(count)*60*time.Second)
	defer cancel()
	// The output path needs a page template, because one run writes one image per
	// page. Numbers restart at one for every batch, which is why the page number
	// is derived from the position in the batch.
	cmd := exec.CommandContext(ctx, typstPath, "compile", "--format", "png", "--ppi", "300",
		"--ignore-system-fonts", in, filepath.Join(dir, "seite-{p}.png"))
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	log, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("typst pdf: %w: %s", err, log)
	}
	for i := range count {
		name := filepath.Join(dir, fmt.Sprintf("seite-%d.png", i+1))
		png, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("typst pdf page %d: %w: %s", first+i, err, log)
		}
		_ = os.Remove(name)
		if err := fn(first+i, png); err != nil {
			return err
		}
	}
	return nil
}
