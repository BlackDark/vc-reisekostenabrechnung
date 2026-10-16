// Command beleg-benchmark times the receipt pipeline on a folder of photos.
// Re-run it when real receipt photos are available (O7); ADR 0005 stays proposed until then.
//
// Without flags it measures the shipped settings, which is what a container does with an
// unmodified environment. The flags exist for the ADR 0005 question: whether AVIF at the
// shipped quality and speed is worth its encode time against the WebP fallback.
//
//	go run ./scripts/beleg-benchmark photos/
//	go run ./scripts/beleg-benchmark -format webp photos/
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BlackDark/vc-reisekostenabrechnung/internal/belegpipe"
)

func main() {
	format := flag.String("format", "", "archive format: avif (default) or webp")
	quality := flag.Int("avif-quality", 0, "AVIF quality, 0 keeps the shipped default of 70")
	speed := flag.Int("avif-speed", 0, "AVIF speed 0-10, lower is slower and smaller (default 8)")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(flag.CommandLine.Output(),
			"usage: go run ./scripts/beleg-benchmark [-format avif|webp] "+
				"[-avif-quality n] [-avif-speed n] <folder>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	folder := flag.Arg(0)
	settings := belegpipe.NormalizeSettings(belegpipe.Settings{
		Format:      *format,
		AVIFQuality: *quality,
		AVIFSpeed:   *speed,
	})
	if settings.Format != "avif" && settings.Format != "webp" {
		fmt.Fprintf(os.Stderr, "format %q is neither avif nor webp\n", settings.Format)
		os.Exit(2)
	}
	var files int
	var archiv int64
	err := filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
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
	fmt.Printf("files=%d\tarchiv_avg=%d\tpipeline=%s\tformat=%s\tavif_quality=%d\tavif_speed=%d\twebp_quality=%d\n",
		files, archiv/int64(files), belegpipe.Version, settings.Format, settings.AVIFQuality,
		settings.AVIFSpeed, settings.WebPQuality)
}
