package belegpipe

import (
	"image"
	"image/draw"
)

// Normalize removes a smooth shadow and a paper tint.
// Per channel: morphological closing (kernel ≈ width/25), box blur, divide, then a 2–98% stretch.
// The arithmetic is integer-only so the same input always yields the same pixels.
func Normalize(src image.Image) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w == 0 || h == 0 {
		return img
	}
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	k := w / 25
	if k < 3 {
		k = 3
	}
	if k%2 == 0 {
		k++
	}
	radius := k / 2
	for ch := 0; ch < 3; ch++ {
		orig := plane(img, ch)
		closed := morphClose(orig, w, h, radius)
		bg := boxBlur(closed, w, h, radius)
		div := divide(orig, bg)
		stretched := stretch(div)
		putPlane(img, ch, stretched)
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	return img
}

func plane(img *image.NRGBA, ch int) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	out := make([]byte, w*h)
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			out[y*w+x] = row[x*4+ch]
		}
	}
	return out
}

func putPlane(img *image.NRGBA, ch int, px []byte) {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	for y := 0; y < h; y++ {
		row := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			row[x*4+ch] = px[y*w+x]
		}
	}
}

func morphClose(src []byte, w, h, radius int) []byte {
	return minFilter(maxFilter(src, w, h, radius), w, h, radius)
}

func maxFilter(src []byte, w, h, radius int) []byte {
	tmp := filter(src, w, h, radius, true, true)
	return filter(tmp, w, h, radius, false, true)
}

func minFilter(src []byte, w, h, radius int) []byte {
	tmp := filter(src, w, h, radius, true, false)
	return filter(tmp, w, h, radius, false, false)
}

func filter(src []byte, w, h, radius int, horizontal, maximum bool) []byte {
	dst := make([]byte, len(src))
	if horizontal {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst[y*w+x] = window(src, w, h, x, y, radius, 1, 0, maximum)
			}
		}
		return dst
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst[y*w+x] = window(src, w, h, x, y, radius, 0, 1, maximum)
		}
	}
	return dst
}

func window(src []byte, w, h, x, y, radius, dx, dy int, maximum bool) byte {
	best := src[y*w+x]
	for k := -radius; k <= radius; k++ {
		xx := x + k*dx
		yy := y + k*dy
		if xx < 0 {
			xx = 0
		}
		if yy < 0 {
			yy = 0
		}
		if xx >= w {
			xx = w - 1
		}
		if yy >= h {
			yy = h - 1
		}
		v := src[yy*w+xx]
		if maximum {
			if v > best {
				best = v
			}
		} else if v < best {
			best = v
		}
	}
	return best
}

func boxBlur(src []byte, w, h, radius int) []byte {
	if radius < 1 {
		out := make([]byte, len(src))
		copy(out, src)
		return out
	}
	tmp := blurAxis(src, w, h, radius, true)
	return blurAxis(tmp, w, h, radius, false)
}

func blurAxis(src []byte, w, h, radius int, horizontal bool) []byte {
	dst := make([]byte, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum, n int
			for k := -radius; k <= radius; k++ {
				xx, yy := x, y
				if horizontal {
					xx += k
				} else {
					yy += k
				}
				if xx < 0 {
					xx = 0
				}
				if yy < 0 {
					yy = 0
				}
				if xx >= w {
					xx = w - 1
				}
				if yy >= h {
					yy = h - 1
				}
				sum += int(src[yy*w+xx])
				n++
			}
			dst[y*w+x] = byte(sum / n)
		}
	}
	return dst
}

func divide(orig, bg []byte) []byte {
	out := make([]byte, len(orig))
	for i := range orig {
		den := int(bg[i])
		if den < 1 {
			den = 1
		}
		v := int(orig[i]) * 255 / den
		if v > 255 {
			v = 255
		}
		out[i] = byte(v)
	}
	return out
}

func stretch(px []byte) []byte {
	var hist [256]int
	for _, v := range px {
		hist[v]++
	}
	n := len(px)
	if n == 0 {
		return px
	}
	lo := percentile(hist, n, 2)
	hi := percentile(hist, n, 98)
	if hi <= lo {
		out := make([]byte, len(px))
		copy(out, px)
		return out
	}
	span := hi - lo
	out := make([]byte, len(px))
	for i, v := range px {
		x := (int(v) - lo) * 255 / span
		if x < 0 {
			x = 0
		}
		if x > 255 {
			x = 255
		}
		out[i] = byte(x)
	}
	return out
}

func percentile(hist [256]int, n, pct int) int {
	target := n * pct / 100
	if target < 1 {
		target = 1
	}
	cum := 0
	for v, c := range hist {
		cum += c
		if cum >= target {
			return v
		}
	}
	return 255
}
