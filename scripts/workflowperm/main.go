// Command workflowperm fails when a reusable workflow asks for a GITHUB_TOKEN
// permission the caller does not grant. GitHub rejects that at load time
// (startup_failure) before any job runs. actionlint 1.7 does not check it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
		found, err := checkFile(root, path, map[string]bool{})
		if err != nil {
			return nil, err
		}
		problems = append(problems, found...)
	}
	return problems, nil
}

func checkFile(root, path string, seen map[string]bool) ([]string, error) {
	if seen[path] {
		return nil, nil
	}
	seen[path] = true
	caller, err := load(path)
	if err != nil {
		return nil, err
	}
	callerWorkflow, callerSet := decodePerms(caller.Permissions)
	var problems []string
	names := make([]string, 0, len(caller.Jobs))
	for name := range caller.Jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		job := caller.Jobs[name]
		uses := strings.TrimSpace(job.Uses)
		if !strings.HasPrefix(uses, "./") {
			continue
		}
		target := filepath.Join(root, strings.TrimPrefix(uses, "./"))
		callee, err := load(target)
		if err != nil {
			return nil, fmt.Errorf("%s: job %s: %w", path, name, err)
		}
		grant, grantSet := decodePerms(job.Permissions)
		if !grantSet {
			grant, grantSet = callerWorkflow, callerSet
		}
		if !grantSet {
			continue
		}
		nested, err := checkFile(root, target, seen)
		if err != nil {
			return nil, err
		}
		problems = append(problems, nested...)
		jobNames := make([]string, 0, len(callee.Jobs))
		for n := range callee.Jobs {
			jobNames = append(jobNames, n)
		}
		sort.Strings(jobNames)
		calleeWorkflow, calleeSet := decodePerms(callee.Permissions)
		for _, nestedName := range jobNames {
			nestedJob := callee.Jobs[nestedName]
			requested, ok := decodePerms(nestedJob.Permissions)
			if !ok {
				requested, ok = calleeWorkflow, calleeSet
			}
			if !ok {
				continue
			}
			for _, scope := range sortedKeys(requested) {
				want := rank(requested[scope])
				have := rank(grant[scope])
				if want < 0 {
					problems = append(problems, fmt.Sprintf("%s job %q calls %s: nested job %q has invalid permission %s: %s", filepath.Base(path), name, uses, nestedName, scope, requested[scope]))
					continue
				}
				if have < want {
					got := grant[scope]
					if got == "" {
						got = "none"
					}
					problems = append(problems, fmt.Sprintf("%s job %q calls %s: nested job %q requests %s: %s, caller grants %s", filepath.Base(path), name, uses, nestedName, scope, requested[scope], got))
				}
			}
		}
	}
	return problems, nil
}

type workflow struct {
	Permissions yaml.Node      `yaml:"permissions"`
	Jobs        map[string]job `yaml:"jobs"`
}

type job struct {
	Uses        string    `yaml:"uses"`
	Permissions yaml.Node `yaml:"permissions"`
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

func decodePerms(node yaml.Node) (map[string]string, bool) {
	if node.Kind == 0 || node.Tag == "!!null" {
		return nil, false
	}
	if node.Kind == yaml.ScalarNode {
		switch node.Value {
		case "":
			return nil, false
		case "none":
			return map[string]string{}, true
		case "read-all":
			return fill("read"), true
		case "write-all":
			return fill("write"), true
		default:
			return map[string]string{"*": node.Value}, true
		}
	}
	var raw map[string]string
	if err := node.Decode(&raw); err != nil {
		return map[string]string{"*": "invalid"}, true
	}
	return raw, true
}

func fill(level string) map[string]string {
	out := make(map[string]string, len(scopes))
	for _, scope := range scopes {
		out[scope] = level
	}
	return out
}

func rank(level string) int {
	switch level {
	case "", "none":
		return 0
	case "read":
		return 1
	case "write":
		return 2
	default:
		return -1
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var scopes = []string{
	"actions", "artifact-metadata", "attestations", "checks", "contents", "deployments",
	"discussions", "id-token", "issues", "models", "packages", "pages", "pull-requests",
	"repository-projects", "security-events", "statuses",
}
