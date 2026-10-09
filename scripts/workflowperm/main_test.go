package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoWorkflowsGrantCalledPermissions(t *testing.T) {
	root := moduleRoot(t)
	problems, err := check(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatal(strings.Join(problems, "\n"))
	}
}

func TestCallerMustGrantNestedWrite(t *testing.T) {
	dir := t.TempDir()
	workflows := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	callee := "name: called\npermissions:\n  contents: read\njobs:\n  scan:\n    runs-on: ubuntu-latest\n    permissions:\n      contents: read\n      security-events: write\n    steps:\n      - run: echo scan\n"
	if err := os.WriteFile(filepath.Join(workflows, "called.yml"), []byte(callee), 0o644); err != nil {
		t.Fatal(err)
	}
	caller := "name: caller\npermissions:\n  contents: read\njobs:\n  ci:\n    uses: ./.github/workflows/called.yml\n"
	if err := os.WriteFile(filepath.Join(workflows, "caller.yml"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := check(workflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "security-events: write") {
		t.Fatalf("%q", problems)
	}

	fixed := "name: caller\npermissions:\n  contents: read\njobs:\n  ci:\n    uses: ./.github/workflows/called.yml\n    permissions:\n      contents: read\n      security-events: write\n"
	if err := os.WriteFile(filepath.Join(workflows, "caller.yml"), []byte(fixed), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err = check(workflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatal(problems)
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
