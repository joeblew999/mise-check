package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joeblew999/mise-check/templates"
)

// template is mise-check template: the template's little app, a Go program with its go.mod (named for the repo's
// folder, at the Go mise-check was built with) and the packages it pins (package.json, its lockfile), written into a repo with no Go program
// yet (no main.go, nor the template's app/main.go), so the repo runs straight away. It never writes over a file: a
// repo with a program of its own gets none of it, and a mise.toml of the repo's own is left as it is, with what to
// add. README.md, the repo's front page, goes in whenever the repo has none, under the repo's name.
func (r repo) template() {
	name := filepath.Base(r.dir)
	if r.exists("README.md") {
		fmt.Println("✓ README.md is the repo's own: left as it is")
	} else {
		b, _ := fs.ReadFile(templates.App, "README.md")
		writeFile(filepath.Join(r.dir, "README.md"), bytes.ReplaceAll(b, []byte(templates.AppName), []byte(name)))
	}
	if r.hasApp() {
		fmt.Println("✓ the repo has a Go program of its own (main.go, app/main.go or cmd/<name>/main.go): none of the template's written, its mise.toml included")
		return
	}
	for _, f := range []string{"app/main.go", "app/main_test.go", "package.json", "pnpm-lock.yaml"} {
		if r.exists(f) {
			fmt.Printf("✓ %s is the repo's own: left as it is\n", f)
			continue
		}
		b, _ := fs.ReadFile(templates.App, f)
		writeFile(filepath.Join(r.dir, filepath.FromSlash(f)), bytes.ReplaceAll(b, []byte(templates.AppName), []byte(name)))
	}
	if r.exists("go.mod") {
		fmt.Println("✓ go.mod is the repo's own: left as it is")
	} else {
		// Written, not go mod init: before the repo has a go.mod, mise has no Go to give (it installs the Go a go.mod
		// names), and a machine needn't have one of its own (macOS doesn't). Its go line is the Go mise-check was built
		// with; mise install then installs it.
		writeFile(filepath.Join(r.dir, "go.mod"), []byte("module "+name+"\n\ngo "+strings.TrimPrefix(runtime.Version(), "go")+"\n"))
	}
	tasks, _ := fs.ReadFile(templates.App, "mise.toml")
	if r.exists("mise.toml") {
		fmt.Printf("✓ mise.toml is the repo's own: left as it is. The template's app runs and tests with these:\n\n%s\n", tasks)
	} else {
		writeFile(filepath.Join(r.dir, "mise.toml"), tasks)
		trustConfig(r.dir, "mise.toml")
	}
	fmt.Println("✓ the template's app: next, mise run mise-check")
}

// hasApp reports whether the repo has a Go program of its own: a main.go at its top, the template's, app/main.go, or
// a binary's, cmd/<name>/main.go, as a mise tool repo has it.
func (r repo) hasApp() bool {
	_, err := os.Stat(filepath.Join(r.dir, "app", "main.go"))
	cmds, _ := filepath.Glob(filepath.Join(r.dir, "cmd", "*", "main.go"))
	return r.exists("main.go") || err == nil || len(cmds) > 0
}
