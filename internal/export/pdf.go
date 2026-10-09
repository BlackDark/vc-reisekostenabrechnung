package export

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func compilePDF(ctx context.Context, typst, dir string, stamp int64) ([]byte, error) {
	if typst == "" {
		return nil, fmt.Errorf("typst missing")
	}
	args := []string{
		"compile", "--root", dir, "--ignore-system-fonts",
		"--pdf-standard", "a-3b",
	}
	if stamp > 0 {
		args = append(args, "--creation-timestamp", strconv.FormatInt(stamp, 10))
	}
	empty, err := os.MkdirTemp("", "rk-packages-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(empty) }()
	args = append(args, "--package-path", empty, "abrechnung.typ", "out.pdf")
	cmd := exec.CommandContext(ctx, typst, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("typst compile: %w: %s", err, out)
	}
	return os.ReadFile(filepath.Join(dir, "out.pdf"))
}

func rasterPDF(ctx context.Context, typst, dir, pdfName string) ([][]byte, error) {
	var pages [][]byte
	for page := 1; page <= 30; page++ {
		typ := fmt.Sprintf("#set page(width: auto, height: auto, margin: 0pt)\n#image(%q, page: %d)\n", pdfName, page)
		name := fmt.Sprintf("raster-%s-%d.typ", pdfName, page)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(typ), 0o644); err != nil {
			return nil, err
		}
		pngName := fmt.Sprintf("raster-%s-%d.png", pdfName, page)
		cmd := exec.CommandContext(ctx, typst, "compile", "--root", dir, "--ignore-system-fonts",
			"--format", "png", "--ppi", "300", name, pngName)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME=/tmp")
		if out, err := cmd.CombinedOutput(); err != nil {
			if page == 1 {
				return nil, fmt.Errorf("raster %s: %w: %s", pdfName, err, out)
			}
			break
		}
		raw, err := os.ReadFile(filepath.Join(dir, pngName))
		if err != nil {
			return nil, err
		}
		jpg, err := pngToJPEG(raw)
		if err != nil {
			return nil, err
		}
		pages = append(pages, jpg)
		_ = os.Remove(filepath.Join(dir, pngName))
		_ = os.Remove(filepath.Join(dir, name))
	}
	return pages, nil
}

func pngToJPEG(raw []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return encodeJPEG(img)
}

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 70}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}
