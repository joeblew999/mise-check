package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	misecheck "github.com/joeblew999/mise-check"
)

// check is mise-check check (04): the repo's rules, each with what fixes it.
func (r repo) check(jsonOut bool) {
	problems := r.rules()
	if jsonOut {
		emit(struct {
			OK       bool     `json:"ok"`
			Problems []string `json:"problems"`
		}{len(problems) == 0, append([]string{}, problems...)})
	} else {
		if len(problems) == 0 {
			fmt.Println("rules: pass")
		}
		for _, p := range problems {
			fmt.Println("FAIL " + p)
		}
	}
	if len(problems) > 0 {
		os.Exit(1)
	}
}

// rules are check's problems: init's files as this version writes them, .mise/config.toml pinning mise-check, the one
// pin of it, the binary's pin and the packages' (go.mod) at one release, misecheck's rules on the repo's own mise files, and every skill of the repo against its tasks. None, and
// the repo passes.
func (r repo) rules() []string {
	problems := append(r.stale(), r.miseRules()...)
	b, err := os.ReadFile(filepath.Join(r.dir, ".mise", "config.toml"))
	switch {
	case err != nil:
		problems = append(problems, ".mise/config.toml is missing: it pins mise-check; run mise run mise-check:init")
	case !pinLine.Match(b):
		problems = append(problems, ".mise/config.toml doesn't pin "+self+": add it, mise use --path .mise/config.toml "+self+"@latest")
	}
	if err == nil && !versionsHostLine.Match(b) {
		problems = append(problems, ".mise/config.toml doesn't set use_versions_host, so latest waits for mise's versions host to list a release: run mise run mise-check:init")
	}
	if missing := excludesMissing(b); err == nil && len(missing) > 0 {
		problems = append(problems, ".mise/config.toml doesn't let mise take the owner's tools at their newest release ("+strings.Join(missing, ", ")+" not in minimum_release_age_excludes): run mise run mise-check:init")
	}
	if p := r.selves(r.goModules()).apart(); p != "" {
		problems = append(problems, p)
	}
	return problems
}

// miseRules are misecheck's rules (github.com/joeblew999/mise-check, the package) on the repo's own mise files, the
// mise side, as a test of the repo would run them: the mise.toml at its top, when it has tasks, and every file of tasks
// in .mise/tasks. Each task one line, its usage passed by name, no POSIX shell syntax, so it runs the same on macOS,
// Linux and Windows. Each problem is named for its file.
func (r repo) miseRules() []string {
	var files []string
	if b, err := os.ReadFile(filepath.Join(r.dir, "mise.toml")); err == nil && strings.Contains(string(b), "[tasks") {
		files = append(files, "mise.toml")
	}
	if tasks, _ := filepath.Glob(filepath.Join(r.dir, ".mise", "tasks", "*.toml")); len(tasks) > 0 {
		for _, f := range tasks {
			rel, _ := filepath.Rel(r.dir, f)
			files = append(files, filepath.ToSlash(rel))
		}
	}
	var problems []string
	for _, f := range files {
		for _, e := range misecheckOn(filepath.Join(r.dir, filepath.FromSlash(f))) {
			problems = append(problems, f+": "+e)
		}
	}
	// Every agent skill of the repo against the tasks it's for (.claude/skills/<namespace>/SKILL.md ↔ <namespace>:…):
	// it names each of them and each flag they take, and only tasks there are.
	return append(problems, recorded(func(t testing.TB) { misecheck.CheckSkills(t, r.dir) })...)
}

// misecheckOn is what misecheck reports as errors on the mise file, its mise side only.
func misecheckOn(file string) []string {
	return recorded(func(t testing.TB) { misecheck.Check(t, misecheck.Config{MiseFile: file}) })
}

// recorded is what a misecheck check reports as errors, given a recorder for its testing.TB.
func recorded(check func(testing.TB)) []string {
	rec := &recorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		check(rec)
	}()
	<-done
	return rec.errors
}

// recorder is the testing.TB misecheck is given here, keeping what it reports as errors: Fatal and Skip end its
// goroutine, as they end a test.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Helper()             {}
func (r *recorder) Logf(string, ...any) {}
func (r *recorder) Error(args ...any)   { r.errors = append(r.errors, fmt.Sprint(args...)) }
func (r *recorder) Errorf(format string, a ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, a...))
}
func (r *recorder) Fatal(args ...any)              { r.Error(args...); runtime.Goexit() }
func (r *recorder) Fatalf(format string, a ...any) { r.Errorf(format, a...); runtime.Goexit() }
func (r *recorder) Skip(...any)                    { runtime.Goexit() }
