// Command beleg-quality measures the receipt pipeline on a folder of photos:
// encoded size, encode time and SSIM against the normalised master. It is the
// measurement behind ADR 0005 and the BELEG_*_QUALITY defaults.
//
//	go run ./scripts/beleg-quality photos/
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gen2brain/avif"
	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
)

// candidate is one encoder setting to compare.
type candidate struct {
	name string
	enc  func(image.Image) ([]byte, error)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/beleg-quality <folder>")
		os.Exit(2)
	}
	cands := candidates()
	paths, err := images(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	type total struct {
		size int64
		ms   int64
		ssim float64
	}
	sums := make([]total, len(cands))
	fail := 0
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			fail = 1
			continue
		}
		img, _, err := belegpipe.DecodeLimited(f, belegpipe.MaxPixels)
		_ = f.Close()
		if err != nil {
			fmt.Printf("%s\tskip\t%s\n", filepath.Base(path), err)
			continue
		}
		// The client deskews and scales to the 300 dpi profile width before upload.
		img = scaleWidth(img, 945)
		ref := belegpipe.Normalize(img)
		fmt.Printf("\n== %s %v (reference = normalised, %dx%d)\n",
			filepath.Base(path), ref.Bounds(), ref.Bounds().Dx(), ref.Bounds().Dy())
		rawSSIM := compare(ref, img)
		fmt.Printf("   normalise costs %.4f SSIM\n", rawSSIM)
		for i, c := range cands {
			start := time.Now()
			bin, err := c.enc(ref)
			if err != nil {
				fmt.Printf("   %-14s ERROR %v\n", c.name, err)
				fail = 1
				continue
			}
			ms := time.Since(start).Milliseconds()
			dec, err := decodeBack(bin)
			if err != nil {
				fmt.Printf("   %-14s DECODE ERROR %v\n", c.name, err)
				fail = 1
				continue
			}
			s := compare(ref, dec)
			sums[i].size += int64(len(bin))
			sums[i].ms += ms
			sums[i].ssim += s
			fmt.Printf("   %-14s %6.1f KB %5d ms  SSIM %.4f\n",
				c.name, float64(len(bin))/1024, ms, s)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "no images")
		os.Exit(1)
	}
	fmt.Printf("\n== average over %d images\n", len(paths))
	type row struct {
		name string
		kb   float64
		ms   float64
		ssim float64
	}
	rows := make([]row, len(cands))
	for i, c := range cands {
		rows[i] = row{c.name,
			float64(sums[i].size) / 1024 / float64(len(paths)),
			float64(sums[i].ms) / float64(len(paths)),
			sums[i].ssim / float64(len(paths))}
	}
	sort.SliceStable(rows, func(a, b int) bool { return rows[a].kb < rows[b].kb })
	for _, r := range rows {
		fmt.Printf("   %-14s %6.1f KB %5.0f ms  SSIM %.4f\n", r.name, r.kb, r.ms, r.ssim)
	}
	os.Exit(fail)
}

func candidates() []candidate {
	out := []candidate{}
	for _, q := range []int{40, 55, 70} {
		q := q
		for _, speed := range []int{6, 8} {
			speed := speed
			out = append(out, candidate{
				name: fmt.Sprintf("avif q%d s%d", q, speed),
				enc: func(src image.Image) ([]byte, error) {
					var buf bytes.Buffer
					err := avif.Encode(&buf, src, avif.Options{Quality: q, Speed: speed})
					return buf.Bytes(), err
				},
			})
		}
	}
	for _, q := range []int{75, 88} {
		q := q
		out = append(out, candidate{
			name: fmt.Sprintf("jpeg q%d", q),
			enc: func(src image.Image) ([]byte, error) {
				var buf bytes.Buffer
				err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: q})
				return buf.Bytes(), err
			},
		})
	}
	for _, q := range []int{75, 90} {
		q := q
		out = append(out, candidate{
			name: fmt.Sprintf("webp q%d", q),
			enc: func(src image.Image) ([]byte, error) {
				var buf bytes.Buffer
				err := webp.Encode(&buf, src, webp.Options{Quality: q, Method: 4})
				return buf.Bytes(), err
			},
		})
	}
	return out
}

func decodeBack(bin []byte) (image.Image, error) {
	switch {
	case bytes.HasPrefix(bin, []byte{0xff, 0xd8}):
		return jpeg.Decode(bytes.NewReader(bin))
	case bytes.HasPrefix(bin, []byte("RIFF")):
		return webp.Decode(bytes.NewReader(bin))
	default:
		return avif.Decode(bytes.NewReader(bin))
	}
}

func images(folder string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

func scaleWidth(src image.Image, width int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 || w == width {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, width, height(width, w, h)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

func height(width, w, h int) int {
	hh := h * width / w
	if hh < 1 {
		hh = 1
	}
	return hh
}

// compare is the mean 8x8-window SSIM of two images on the luma channel.
// Windows near an edge are skipped so different sizes do not bias the score.
func compare(a, b image.Image) float64 {
	w, h := 8, 8
	bb := b.Bounds()
	if bb.Dx() < w || bb.Dy() < h {
		return 0
	}
	var sum, n float64
	for y := 0; y+h <= bb.Dy(); y += 4 {
		for x := 0; x+w <= bb.Dx(); x += 4 {
			sa, sb, saa, sbb, sab := 0.0, 0.0, 0.0, 0.0, 0.0
			for dy := 0; dy < h; dy++ {
				for dx := 0; dx < w; dx++ {
					va := luma(a, x+dx, y+dy)
					vb := luma(b, x+dx, y+dy)
					sa += va
					sb += vb
					saa += va * va
					sbb += vb * vb
					sab += va * vb
				}
			}
			n2 := float64(w * h)
			ma, mb := sa/n2, sb/n2
			va := saa/n2 - ma*ma
			vb := sbb/n2 - mb*mb
			cov := sab/n2 - ma*mb
			c1, c2 := 6.5025, 58.5225
			s := ((2*ma*mb + c1) * (2*cov + c2)) / ((ma*ma + mb*mb + c1) * (va + vb + c2))
			sum += s
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func luma(img image.Image, x, y int) float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return 0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(b>>8)
}
