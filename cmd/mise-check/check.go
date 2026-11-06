package main

import (
	"fmt"
	"os"
	"path/filepath"
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

// rules are check's problems: init's files as this version writes them, and .mise/config.toml pinning mise-check, the
// one pin of it. None, and the repo passes.
func (r repo) rules() []string {
	problems := r.stale()
	b, err := os.ReadFile(filepath.Join(r.dir, ".mise", "config.toml"))
	switch {
	case err != nil:
		problems = append(problems, ".mise/config.toml is missing: it pins mise-check; run mise run mise-check:03-init")
	case !pinLine.Match(b):
		problems = append(problems, ".mise/config.toml doesn't pin "+self+": add it, mise use --path .mise/config.toml "+self+"@latest")
	}
	return problems
}
