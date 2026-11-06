package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestHelp: show-help names every task of the template's tasks file, as mise run <task>, and only tasks there are, so
// the help a person reads covers everything the binary can do, as the skill and the docs page do (template:check-mise).
func TestHelp(t *testing.T) {
	b, err := os.ReadFile("../../templates/.mise/tasks/mise-check.toml")
	if err != nil {
		t.Fatal(err)
	}
	tasks := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\["([^"]+)"\]$`).FindAllStringSubmatch(string(b), -1) {
		tasks[m[1]] = true
	}
	for task := range tasks {
		if !regexp.MustCompile(`mise run ` + regexp.QuoteMeta(task) + `(\s|$)`).MatchString(help) {
			t.Errorf("show-help doesn't name the task %s: add it, with what it does", task)
		}
	}
	for _, m := range regexp.MustCompile(`mise run (mise-check(?::[a-z0-9-]+)?)`).FindAllStringSubmatch(help, -1) {
		if !tasks[m[1]] {
			t.Errorf("show-help names %s, which isn't a task of the template's tasks file", m[1])
		}
	}
	if !strings.Contains(help, "--json") || !strings.Contains(help, "--no-major") {
		t.Error("show-help doesn't name --json and --no-major, the flags the tasks take")
	}
}
