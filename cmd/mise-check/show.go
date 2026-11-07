package main

import (
	"fmt"
	"strings"
)

// report is mise-check show --json: where the repo is, and what to run next.
type report struct {
	Version  string   `json:"version"`  // this build of mise-check
	App      bool     `json:"app"`      // the repo has Go of its own (a go.mod, or a Go program), or the template's app
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
	fmt.Printf("%s app       Go of the repo's own, a go.mod (or the template's app, in a repo with none: mise run mise-check:template)\n", mark[s.App])
	fmt.Println("· update    every pin at its newest release, whenever you want them moved")
	for _, p := range s.Problems {
		fmt.Println("  FAIL " + p)
	}
	fmt.Println("next: " + s.Next)
}

// status is where the repo is, which show says: the problems, whether it has Go of its own, and the step to take next.
// A repo with a go.mod has code of its own, a tool's or a library's, even with no program: the template, which would
// add its little app and packages beside it, isn't next.
func (r repo) status() report {
	s := report{Version: buildVersion(), Problems: r.rules(), App: r.hasApp() || r.exists("go.mod")}
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
