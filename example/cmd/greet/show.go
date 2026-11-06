package main

import (
	"fmt"
	"strings"
)

// report is greet show --json: where the repo is, and what to run next.
type report struct {
	Version  string   `json:"version"`  // this build of greet: the module version go.mod pins
	App      bool     `json:"app"`      // the repo has a Go program of its own (main.go, or the template's app/main.go)
	Problems []string `json:"problems"` // check's
	Next     string   `json:"next"`     // the task to run next
	Command  string   `json:"command"`  // the same without mise
}

// show is greet show (05), and greet with no command (mise run greet): the steps, which are done for this repo, and
// the next one.
func show(dir string, jsonOut bool) {
	r := report{Version: buildVersion(), Problems: rules(dir)}
	r.App = hasApp(dir)
	switch {
	case anyHas(r.Problems, "go tool greet init"):
		r.Next, r.Command = "mise run greet:03-init", "go tool greet init"
	case len(r.Problems) > 0:
		r.Next, r.Command = "mise run greet:04-check", "go tool greet check"
	case !r.App:
		r.Next, r.Command = "mise run greet:template", "go tool greet template"
	default:
		r.Next, r.Command = "mise run greet:06-hello", "go tool greet hello"
	}
	if jsonOut {
		emit(r)
		return
	}
	mark := map[bool]string{true: "✓", false: "·"}
	fmt.Printf("greet %s, from the module go.mod pins\n", r.Version)
	fmt.Printf("%s 03-init   greet's files, as this version writes them\n", mark[!anyHas(r.Problems, "go tool greet init")])
	fmt.Printf("%s 04-check  the repo's rules\n", mark[len(r.Problems) == 0])
	fmt.Printf("%s app       a Go program of the repo's own (or the template's: mise run greet:template)\n", mark[r.App])
	for _, p := range r.Problems {
		fmt.Println("  FAIL " + p)
	}
	fmt.Printf("next: %s (without mise: %s)\n", r.Next, r.Command)
}

// anyHas reports whether any of problems says s.
func anyHas(problems []string, s string) bool {
	for _, p := range problems {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}
