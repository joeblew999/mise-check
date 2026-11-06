package main

import (
	"fmt"
	"strings"
)

// report is mise-check show --json: where the repo is, and what to run next.
type report struct {
	Version  string   `json:"version"`  // this build of mise-check
	App      bool     `json:"app"`      // the repo has a Go program of its own (main.go, or the template's app/main.go)
	Problems []string `json:"problems"` // check's
	Next     string   `json:"next"`     // the task to run next
}

// show is mise-check show (05), and mise-check with no command (mise run mise-check): the steps, which are done for
// this repo, and the next one.
func (r repo) show(jsonOut bool) {
	s := report{Version: buildVersion(), Problems: r.rules(), App: r.hasApp()}
	switch {
	case anyHas(s.Problems, "03-init"):
		s.Next = "mise run mise-check:03-init"
	case len(s.Problems) > 0:
		s.Next = "mise run mise-check:04-check"
	case !s.App:
		s.Next = "mise run mise-check:template"
	default:
		s.Next = "mise run mise-check:update"
	}
	if jsonOut {
		emit(s)
		return
	}
	mark := map[bool]string{true: "✓", false: "·"}
	fmt.Printf("mise-check %s\n", s.Version)
	fmt.Printf("%s 03-init    mise-check's files, as this version writes them\n", mark[!anyHas(s.Problems, "03-init")])
	fmt.Printf("%s 04-check   the repo's rules\n", mark[len(s.Problems) == 0])
	fmt.Printf("%s app        a Go program of the repo's own (or the template's: mise run mise-check:template)\n", mark[s.App])
	fmt.Println("· update  every pin at its newest release, whenever you want them moved")
	for _, p := range s.Problems {
		fmt.Println("  FAIL " + p)
	}
	fmt.Println("next: " + s.Next)
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
