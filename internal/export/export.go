// Package export renders a one-page Typst sample. Real Abrechnung layout arrives in later milestones.
package export

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const sample = `#set page(paper: "a4", margin: 2cm)
= Reisekostenabrechnung
Sample export for the container smoke test.
`

// Sample compiles the embedded sample document to outPath.
func Sample(typstPath, outPath string) error {
	dir, err := os.MkdirTemp("", "rk-export-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	in := filepath.Join(dir, "sample.typ")
	if err := os.WriteFile(in, []byte(sample), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	cmd := exec.Command(typstPath, "compile", "--ignore-system-fonts", in, outPath)
	cmd.Env = append(os.Environ(), "HOME=/tmp")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("typst compile: %w: %s", err, out)
	}
	return nil
}
