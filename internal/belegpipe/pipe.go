// Package belegpipe normalises receipt photos and encodes the archive copy.
// Version is stored on every Beleg. Changing the algorithm requires a new version.
package belegpipe

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

// Version is the pipeline revision recorded in Beleg metadata.
const Version = "2026.1"

// MaxPixels rejects decompression bombs (SPEC 10.8).
const MaxPixels = 40_000_000

// PreviewLongest is the thumbnail's longest side in pixels.
const PreviewLongest = 320

// Settings are the encoder knobs. Zero values are filled from the product defaults.
type Settings struct {
	Format      string
	AVIFQuality int
	AVIFSpeed   int
	WebPQuality int
	JPEGQuality int
	PreviewWebP int
}

// Photo is one processed page.
type Photo struct {
	Archiv     []byte
	ArchivMIME string
	ArchivExt  string
	Preview    []byte
	ExportJPEG []byte
}

// NormalizeSettings fills empty fields with the ADR 0005 defaults.
func NormalizeSettings(s Settings) Settings {
	if s.Format == "" {
		s.Format = "avif"
	}
	if s.AVIFQuality == 0 {
		s.AVIFQuality = 40
	}
	if s.AVIFSpeed == 0 {
		s.AVIFSpeed = 6
	}
	if s.WebPQuality == 0 {
		s.WebPQuality = 55
	}
	if s.JPEGQuality == 0 {
		s.JPEGQuality = 70
	}
	if s.PreviewWebP == 0 {
		s.PreviewWebP = 55
	}
	return s
}

// DecodeLimited decodes a JPEG, PNG or WebP and refuses images over maxPixels.
func DecodeLimited(r io.Reader, maxPixels int) (image.Image, string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}
	kind := Sniff(data)
	if kind != "jpeg" && kind != "png" && kind != "webp" {
		return nil, kind, errors.New("not an image")
	}
	if maxPixels <= 0 {
		maxPixels = MaxPixels
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, kind, err
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 20000 || cfg.Height > 20000 {
		return nil, kind, errors.New("pixel limit")
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, kind, errors.New("pixel limit")
	}
	var img image.Image
	switch kind {
	case "webp":
		img, err = webp.Decode(bytes.NewReader(data))
	default:
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return nil, kind, err
	}
	return img, kind, nil
}

// Process normalises src and encodes the archive copy, the WebP preview and the JPEG export.
func Process(src image.Image, s Settings) (Photo, error) {
	s = NormalizeSettings(s)
	norm := Normalize(src)
	var photo Photo
	var archiv bytes.Buffer
	switch s.Format {
	case "webp":
		if err := webp.Encode(&archiv, norm, webp.Options{Quality: s.WebPQuality, Method: 4}); err != nil {
			return Photo{}, err
		}
		photo.ArchivMIME = "image/webp"
		photo.ArchivExt = "webp"
	default:
		if err := avif.Encode(&archiv, norm, avif.Options{Quality: s.AVIFQuality, Speed: s.AVIFSpeed}); err != nil {
			return Photo{}, err
		}
		photo.ArchivMIME = "image/avif"
		photo.ArchivExt = "avif"
	}
	photo.Archiv = archiv.Bytes()
	prev := fitLongest(norm, PreviewLongest)
	var preview bytes.Buffer
	if err := webp.Encode(&preview, prev, webp.Options{Quality: s.PreviewWebP, Method: 4}); err != nil {
		return Photo{}, err
	}
	photo.Preview = preview.Bytes()
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, norm, &jpeg.Options{Quality: s.JPEGQuality}); err != nil {
		return Photo{}, err
	}
	photo.ExportJPEG = jpg.Bytes()
	return photo, nil
}

// EncodePreviewJPEG encodes a raster (a PDF page) as a WebP thumbnail and a JPEG export copy.
func EncodePreviewJPEG(src image.Image, jpegQuality, previewQuality int) (preview, exportJPEG []byte, err error) {
	if jpegQuality == 0 {
		jpegQuality = 70
	}
	if previewQuality == 0 {
		previewQuality = 55
	}
	prev := fitLongest(src, PreviewLongest)
	var pb, jb bytes.Buffer
	if err = webp.Encode(&pb, prev, webp.Options{Quality: previewQuality, Method: 4}); err != nil {
		return nil, nil, err
	}
	if err = jpeg.Encode(&jb, src, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, nil, err
	}
	return pb.Bytes(), jb.Bytes(), nil
}

func fitLongest(src image.Image, longest int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	if w <= longest && h <= longest {
		return src
	}
	var nw, nh int
	if w >= h {
		nw = longest
		nh = h * longest / w
	} else {
		nh = longest
		nw = w * longest / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}
