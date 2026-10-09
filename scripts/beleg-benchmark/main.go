// Command beleg-benchmark times the receipt pipeline on a folder of photos.
// Re-run it when real receipt photos are available (O7); ADR 0005 stays proposed until then.
//
//	go run ./scripts/beleg-benchmark photos/
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/beleg-benchmark <folder>")
		os.Exit(2)
	}
	settings := belegpipe.NormalizeSettings(belegpipe.Settings{})
	var files int
	var archiv int64
	err := filepath.WalkDir(os.Args[1], func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		img, _, err := belegpipe.DecodeLimited(f, belegpipe.MaxPixels)
		_ = f.Close()
		if err != nil {
			fmt.Printf("%s\terror\t%s\n", path, err)
			return nil
		}
		start := time.Now()
		photo, err := belegpipe.Process(img, settings)
		if err != nil {
			fmt.Printf("%s\terror\t%s\n", path, err)
			return nil
		}
		files++
		archiv += int64(len(photo.Archiv))
		fmt.Printf("%s\t%dms\tarchiv=%d\tpreview=%d\tjpeg=%d\n", path, time.Since(start).Milliseconds(), len(photo.Archiv), len(photo.Preview), len(photo.ExportJPEG))
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if files == 0 {
		fmt.Fprintln(os.Stderr, "no images")
		os.Exit(1)
	}
	fmt.Printf("files=%d\tarchiv_avg=%d\tpipeline=%s\tformat=%s\tquality=%d\tspeed=%d\n", files, archiv/int64(files), belegpipe.Version, settings.Format, settings.AVIFQuality, settings.AVIFSpeed)
}
