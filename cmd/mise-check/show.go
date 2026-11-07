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

// show is mise-check show, and mise-check with no command (mise run mise-check): the steps, which are done for
// this repo, and the next one.
func (r repo) show(jsonOut bool) {
	s := r.status()
	if jsonOut {
		emit(s)
		return
	}
	mark := map[bool]string{true: "✓", false: "·"}
	fmt.Printf("mise-check %s\n", s.Version)
	fmt.Printf("%s init      mise-check's files, as this version writes them\n", mark[!anyHas(s.Problems, initTask)])
	fmt.Printf("%s check     the repo's rules\n", mark[len(s.Problems) == 0])
	fmt.Printf("%s app       a Go program of the repo's own: main.go, app/main.go or cmd/<name>/main.go (or the template's: mise run mise-check:template)\n", mark[s.App])
	fmt.Println("· update    every pin at its newest release, whenever you want them moved")
	for _, p := range s.Problems {
		fmt.Println("  FAIL " + p)
	}
	fmt.Println("next: " + s.Next)
}

// status is where the repo is, which show says: the problems, whether it has a Go program, and the step to take next.
func (r repo) status() report {
	s := report{Version: buildVersion(), Problems: r.rules(), App: r.hasApp()}
	switch {
	case anyHas(s.Problems, initTask):
		s.Next = "mise run mise-check:init"
	case len(s.Problems) > 0:
		s.Next = "mise run mise-check:check"
	case !s.App:
		s.Next = "mise run mise-check:template"
	default:
		s.Next = "mise run mise-check:update"
	}
	return s
}

// initTask is the task that writes init's files again, which every problem with them names: show's init step is done
// when no problem names it.
const initTask = "mise run mise-check:init"

// anyHas reports whether any of problems says s.
func anyHas(problems []string, s string) bool {
	for _, p := range problems {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}
