package main

import (
	"fmt"
	"os"
)

// check is greet check (04): the repo's rules, each with what fixes it. greet's files are the ones this version writes
// (go tool greet init writes them again), and the repo's go.mod has greet's tool line, the one pin of greet's module.
func check(dir string, jsonOut bool) {
	problems := rules(dir)
	if jsonOut {
		emit(struct {
			OK       bool     `json:"ok"`
			Problems []string `json:"problems"`
		}{len(problems) == 0, problems})
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

// rules are check's problems: none, and the repo passes.
func rules(dir string) []string {
	problems := []string{}
	path, text := goMod(dir)
	switch {
	case path == "":
		problems = append(problems, errNoModule.Error())
	case !hasTool(text):
		problems = append(problems, path+" has no tool line for greet, the one pin of greet's module, its command and its package: go get -tool "+tool+"@latest")
	}
	return append(problems, stale(dir)...)
}
