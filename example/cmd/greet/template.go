package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/joeblew999/mise-check/example/templates"
)

// template is greet template: the template's little app, a Go program that imports greet's package, written into a
// repo with no Go program yet (no main.go, nor the template's app/main.go), so the repo runs straight away. It never writes over a file: a repo with
// a program of its own gets none of it, and a mise.toml of the repo's own is left as it is, with what to add. README.md,
// the repo's front page, goes in whenever the repo has none, under the repo's name.
func template(dir string) {
	if p, _ := goMod(dir); p == "" {
		fail(errNoModule)
	}
	name := filepath.Base(dir)
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err == nil {
		fmt.Println("✓ README.md is the repo's own: left as it is")
	} else {
		b, _ := fs.ReadFile(templates.App, "README.md")
		writeFile(filepath.Join(dir, "README.md"), bytes.ReplaceAll(b, []byte(templates.AppName), []byte(name)))
	}
	if hasApp(dir) {
		fmt.Println("✓ the repo has a Go program of its own (main.go, or app/main.go): none of the template's written, its mise.toml included")
		return
	}
	for _, f := range []string{"app/main.go", "app/main_test.go"} {
		b, _ := fs.ReadFile(templates.App, f)
		writeFile(filepath.Join(dir, f), b)
	}
	tasks, _ := fs.ReadFile(templates.App, "mise.toml")
	if _, err := os.Stat(filepath.Join(dir, "mise.toml")); err == nil {
		fmt.Printf("✓ mise.toml is the repo's own: left as it is. The template's app runs and tests with these:\n\n%s\n", tasks)
	} else {
		writeFile(filepath.Join(dir, "mise.toml"), tasks)
		trustConfig(dir, "mise.toml")
	}
	fmt.Println("✓ the template's app: next, go tool greet (or mise run greet)")
}

// hasApp reports whether the repo in dir has a Go program of its own: a main.go at its top, or the template's,
// app/main.go.
func hasApp(dir string) bool {
	for _, p := range []string{"main.go", filepath.Join("app", "main.go")} {
		if _, err := os.Stat(filepath.Join(dir, p)); err == nil {
			return true
		}
	}
	return false
}
