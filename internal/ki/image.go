package ki

import (
	"bytes"
	"image"
	"image/jpeg"
)

// FitJPEG re-encodes a JPEG so its longest edge is at most maxEdge.
func FitJPEG(src []byte, maxEdge int) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxEdge > 0 && (w > maxEdge || h > maxEdge) {
		var nw, nh int
		if w >= h {
			nw = maxEdge
			nh = h * maxEdge / w
		} else {
			nh = maxEdge
			nw = w * maxEdge / h
		}
		if nw < 1 {
			nw = 1
		}
		if nh < 1 {
			nh = 1
		}
		img = scaleNearest(img, nw, nh)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func scaleNearest(src image.Image, nw, nh int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	for y := 0; y < nh; y++ {
		sy := sb.Min.Y + y*sh/nh
		for x := 0; x < nw; x++ {
			sx := sb.Min.X + x*sw/nw
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
