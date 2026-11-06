package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/mise-check/example/templates"
)

// theirs are the template's files a repo may have of its own already: init writes them only when it has none.
var theirs = map[string]bool{".gitignore": true, ".claude/settings.json": true, ".mise/config.toml": true}

// initRepo is greet init (03): the template's tool half written into the repo, each file at its path there (the
// numbered tasks, the GitHub workflow, the Claude Code hook and skill, the docs), all at the version go.mod pins. It
// never writes the repo's go.mod: go get -tool did, and that line is the one pin. Run again after upgrading, it writes
// what the new version would.
func initRepo(dir string) {
	if p, _ := goMod(dir); p == "" {
		fail(errNoModule)
	}
	must(fs.WalkDir(templates.Tool, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		to := filepath.Join(dir, filepath.FromSlash(p))
		if _, err := os.Stat(to); err == nil && theirs[p] {
			fmt.Printf("✓ %s is the repo's own: left as it is\n", p)
			return nil
		}
		b, err := fs.ReadFile(templates.Tool, p)
		if err != nil {
			return err
		}
		writeFile(to, b)
		if strings.HasSuffix(p, ".sh") {
			return os.Chmod(to, 0o755)
		}
		return nil
	}))
	trustConfig(dir, ".mise/config.toml")
	fmt.Println("✓ next: go tool greet (or mise run greet): what's done, and what to run next")
}

// stale names the files init wrote that this version would write differently, and a .claude/settings.json of the
// repo's own that doesn't run the hook.
func stale(dir string) []string {
	var out []string
	fs.WalkDir(templates.Tool, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || theirs[p] {
			return err
		}
		want, _ := fs.ReadFile(templates.Tool, p)
		// Line endings aside: a Windows checkout of the repo may have \r\n where init wrote \n.
		crlf := func(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		switch {
		case err != nil:
			out = append(out, p+" is missing: run go tool greet init")
		case crlf(b) != crlf(want):
			out = append(out, p+" is out of date: run go tool greet init")
		}
		return nil
	})
	if b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json")); err == nil && !strings.Contains(string(b), templates.HookFile) {
		out = append(out, ".claude/settings.json doesn't run .claude/hooks/"+templates.HookFile+": add its SessionStart entry (docs/greet/README.md)")
	}
	return out
}
