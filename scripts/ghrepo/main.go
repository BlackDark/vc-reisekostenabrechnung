// Command ghrepo fails when a job with no actions/checkout runs gh without
// --repo. gh otherwise shells out to git to discover the repository and exits
// with "fatal: not a git repository" before it calls the API.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

func main() {
	dir := ".github/workflows"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	problems, err := check(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, strings.Join(problems, "\n"))
		os.Exit(1)
	}
}

func check(workflowsDir string) ([]string, error) {
	root := filepath.Dir(filepath.Dir(workflowsDir))
	matches, err := filepath.Glob(filepath.Join(workflowsDir, "*.yml"))
	if err != nil {
		return nil, err
	}
	more, err := filepath.Glob(filepath.Join(workflowsDir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	matches = append(matches, more...)
	sort.Strings(matches)
	var problems []string
	for _, path := range matches {
		found, err := checkFile(root, path)
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func checkFile(root, path string) ([]string, error) {
	doc, err := load(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(doc.Jobs))
	for name := range doc.Jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	var problems []string
	for _, name := range names {
		job := doc.Jobs[name]
		if job.Uses != "" || hasCheckout(job.Steps) {
			continue
		}
		seen := map[string]bool{}
		for _, step := range job.Steps {
			script := strings.TrimSpace(step.Run)
			if script == "" {
				continue
			}
			problems = append(problems, scanShell(root, filepath.Base(path), name, script, seen)...)
		}
	}
	return problems, nil
}

func scanShell(root, workflow, jobName, script string, seen map[string]bool) []string {
	var problems []string
	for _, cmd := range splitCommands(stripHeredocs(script)) {
		tokens := tokenize(cmd)
		if args, ok := ghArgs(tokens); ok && !hasRepoFlag(args) {
			problems = append(problems, fmt.Sprintf("%s job %q runs gh without --repo and has no actions/checkout: %s", workflow, jobName, shorten(cmd)))
		}
		for _, rel := range scriptRefs(tokens) {
			full, ok := resolveScript(root, rel)
			if !ok || seen[full] {
				continue
			}
			seen[full] = true
			body, err := os.ReadFile(full)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s job %q: read %s: %v", workflow, jobName, rel, err))
				continue
			}
			problems = append(problems, scanShell(root, rel, jobName, string(body), seen)...)
		}
	}
	return problems
}

func hasCheckout(steps []step) bool {
	for _, step := range steps {
		if actionName(step.Uses) == "actions/checkout" {
			return true
		}
	}
	return false
}

func actionName(uses string) string {
	uses = strings.TrimSpace(uses)
	if i := strings.IndexByte(uses, '@'); i >= 0 {
		uses = uses[:i]
	}
	return uses
}

func ghArgs(tokens []string) ([]string, bool) {
	i := 0
	for i < len(tokens) && isEnvAssign(tokens[i]) {
		i++
	}
	if i < len(tokens) && filepath.Base(tokens[i]) == "gh" {
		return tokens[i:], true
	}
	return nil, false
}

func hasRepoFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--repo" || strings.HasPrefix(arg, "--repo=") || arg == "-R" || strings.HasPrefix(arg, "-R=") {
			return true
		}
	}
	return false
}

func isEnvAssign(token string) bool {
	eq := strings.IndexByte(token, '=')
	if eq <= 0 || strings.HasPrefix(token, "-") {
		return false
	}
	for _, r := range token[:eq] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func scriptRefs(tokens []string) []string {
	var refs []string
	for i, token := range tokens {
		if isShell(token) && i+1 < len(tokens) && isScript(tokens[i+1]) {
			refs = append(refs, tokens[i+1])
		}
		if isScript(token) {
			refs = append(refs, token)
		}
	}
	return refs
}

func isShell(token string) bool {
	base := filepath.Base(token)
	return base == "bash" || base == "sh" || base == "source" || token == "."
}

func isScript(token string) bool {
	if strings.ContainsAny(token, "$#{*`") || strings.Contains(token, "..") {
		return false
	}
	return strings.HasSuffix(token, ".sh")
}

func resolveScript(root, rel string) (string, bool) {
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || strings.HasPrefix(rel, "/") {
		return "", false
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

func shorten(cmd string) string {
	cmd = strings.Join(strings.Fields(cmd), " ")
	if len(cmd) > 160 {
		return cmd[:157] + "..."
	}
	return cmd
}

type workflow struct {
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	Uses  string `yaml:"uses"`
	Steps []step `yaml:"steps"`
}

type step struct {
	Uses string `yaml:"uses"`
	Run  string `yaml:"run"`
}

func load(path string) (workflow, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return workflow{}, err
	}
	var doc workflow
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return workflow{}, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

func stripHeredocs(script string) string {
	lines := strings.Split(script, "\n")
	var out []string
	end := ""
	for _, line := range lines {
		if end != "" {
			if strings.TrimSpace(line) == end {
				end = ""
			}
			continue
		}
		if delim, ok := heredocDelim(line); ok {
			end = delim
			if i := strings.Index(line, "<<"); i >= 0 {
				out = append(out, line[:i])
			}
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func heredocDelim(line string) (string, bool) {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case !inSingle && !inDouble && c == '<' && i+1 < len(line) && line[i+1] == '<':
			rest := strings.TrimSpace(line[i+2:])
			rest = strings.TrimPrefix(rest, "-")
			rest = strings.TrimSpace(rest)
			if rest == "" {
				return "", false
			}
			if rest[0] == '\'' || rest[0] == '"' {
				q := rest[0]
				rest = rest[1:]
				j := strings.IndexByte(rest, q)
				if j < 0 {
					return "", false
				}
				return rest[:j], true
			}
			end := len(rest)
			if j := strings.IndexAny(rest, " \t;"); j >= 0 {
				end = j
			}
			return rest[:end], true
		}
	}
	return "", false
}

func splitCommands(script string) []string {
	var cmds []string
	var b strings.Builder
	inSingle, inDouble, escaped := false, false, false
	flush := func() {
		s := strings.TrimSpace(b.String())
		b.Reset()
		if s != "" {
			cmds = append(cmds, s)
		}
	}
	for i := 0; i < len(script); i++ {
		c := script[i]
		if escaped {
			b.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' && !inSingle {
			if i+1 < len(script) && script[i+1] == '\n' {
				i++
				b.WriteByte(' ')
				continue
			}
			escaped = true
			b.WriteByte(c)
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			b.WriteByte(c)
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(c)
			continue
		}
		if !inSingle && !inDouble {
			if c == '#' {
				for i+1 < len(script) && script[i+1] != '\n' {
					i++
				}
				continue
			}
			if c == '\n' || c == ';' || c == '|' {
				if c == '|' && i+1 < len(script) && script[i+1] == '|' {
					i++
				}
				flush()
				continue
			}
			if c == '&' && i+1 < len(script) && script[i+1] == '&' {
				i++
				flush()
				continue
			}
		}
		b.WriteByte(c)
	}
	flush()
	return cmds
}

func tokenize(cmd string) []string {
	var tokens []string
	var b strings.Builder
	inSingle, inDouble := false, false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tokens = append(tokens, b.String())
		b.Reset()
	}
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				b.WriteByte(c)
			}
		case inDouble:
			if c == '\\' && i+1 < len(cmd) {
				i++
				b.WriteByte(cmd[i])
				continue
			}
			if c == '"' {
				inDouble = false
			} else {
				b.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == ' ' || c == '\t' || c == '\n':
			flush()
		case c == '>':
			flush()
			if i+1 < len(cmd) && cmd[i+1] == '>' {
				tokens = append(tokens, ">>")
				i++
			} else {
				tokens = append(tokens, ">")
			}
		case c == '<':
			flush()
			tokens = append(tokens, "<")
		default:
			b.WriteByte(c)
		}
	}
	flush()
	return tokens
}
