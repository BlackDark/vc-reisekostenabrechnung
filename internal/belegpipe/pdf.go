package belegpipe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// RenderPDFPage asks typst to rasterise one PDF page. page is 1-based.
func RenderPDFPage(typstPath string, pdf []byte, page int) ([]byte, error) {
	if page < 1 {
		return nil, fmt.Errorf("pdf page")
	}
	dir, err := os.MkdirTemp("", "rk-pdf-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "beleg.pdf"), pdf, 0o600); err != nil {
		return nil, err
	}
	src := fmt.Sprintf("#set page(width: auto, height: auto, margin: 0pt)\n#image(\"beleg.pdf\", page: %d)\n", page)
	in := filepath.Join(dir, "page.typ")
	if err := os.WriteFile(in, []byte(src), 0o600); err != nil {
		return nil, err
	}
	out := filepath.Join(dir, "page.png")
	cmd := exec.Command(typstPath, "compile", "--format", "png", "--ppi", "300", "--ignore-system-fonts", in, out)
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	log, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("typst pdf: %w: %s", err, log)
	}
	return os.ReadFile(out)
}
