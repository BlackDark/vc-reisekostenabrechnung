package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoWorkflowsPassRepo(t *testing.T) {
	root := moduleRoot(t)
	problems, err := check(filepath.Join(root, ".github", "workflows"))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatal(strings.Join(problems, "\n"))
	}
}

func TestMissingRepoWithoutCheckout(t *testing.T) {
	dir := t.TempDir()
	workflows := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := "" +
		"name: release\n" +
		"jobs:\n" +
		"  notes:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - run: |\n" +
		"          gh release view \"$tag\" --json body --jq .body > /tmp/notes.md\n" +
		"          python3 - << 'PY'\n" +
		"          gh release view ignored-inside-heredoc\n" +
		"          PY\n" +
		"          gh release upload \"$tag\" sbom.spdx.json --clobber\n" +
		"          gh release edit \"$tag\" --notes-file /tmp/notes.md --draft=false\n" +
		"  checked:\n" +
		"    runs-on: ubuntu-latest\n" +
		"    steps:\n" +
		"      - uses: actions/checkout@v4\n" +
		"      - run: gh release view v1.0.0\n"
	if err := os.WriteFile(filepath.Join(workflows, "release.yml"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := check(workflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 3 {
		t.Fatalf("%q", problems)
	}
	for _, problem := range problems {
		if !strings.Contains(problem, `job "notes"`) || !strings.Contains(problem, "without --repo") {
			t.Fatal(problem)
		}
	}

	fixed := strings.ReplaceAll(broken, "gh release view \"$tag\"", "gh release view \"$tag\" --repo \"$GITHUB_REPOSITORY\"")
	fixed = strings.ReplaceAll(fixed, "gh release upload \"$tag\"", "gh release upload \"$tag\" --repo \"$GITHUB_REPOSITORY\"")
	fixed = strings.ReplaceAll(fixed, "gh release edit \"$tag\"", "gh release edit \"$tag\" --repo \"$GITHUB_REPOSITORY\"")
	if err := os.WriteFile(filepath.Join(workflows, "release.yml"), []byte(fixed), 0o644); err != nil {
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

func TestCalledScriptWithoutCheckout(t *testing.T) {
	dir := t.TempDir()
	workflows := filepath.Join(dir, ".github", "workflows")
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	caller := "name: call\njobs:\n  dispatch:\n    runs-on: ubuntu-latest\n    steps:\n      - run: bash scripts/dispatch-workflow.sh ci.yml main\n"
	if err := os.WriteFile(filepath.Join(workflows, "caller.yml"), []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nset -eu\ngh workflow run \"$1\" --ref \"$2\"\n"
	if err := os.WriteFile(filepath.Join(scripts, "dispatch-workflow.sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, err := check(workflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], "dispatch-workflow.sh") {
		t.Fatalf("%q", problems)
	}

	script = "#!/bin/sh\nset -eu\ngh workflow run \"$1\" --repo \"$GH_REPO\" --ref \"$2\"\n"
	if err := os.WriteFile(filepath.Join(scripts, "dispatch-workflow.sh"), []byte(script), 0o644); err != nil {
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
