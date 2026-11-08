package main

import (
	"fmt"
	"os"
	"os/exec"
)

// verify is mise-check verify: what the repo's GitHub workflow runs once it has checked out and installed mise. First
// each kind of pin the updates move is checked consistent with what pins it, as the updates leave them: the packages
// with their lockfile in every package root (aube install --frozen-lockfile, which fails when package.json and the lockfile differ) and every
// go.mod with its code (go mod tidy -diff -e, which fails when go.mod or go.sum aren't what the imports need, and goes
// on past a package the repo's setup generates, which a fresh checkout doesn't have yet); mise's tools
// are pinned exactly in the mise files, which are their own lock. Then the repo's own setup and test tasks when it has
// them, then check. Run here, it does what GitHub does.
func (r repo) verify() {
	for _, d := range r.packageRoots() { // each package root, its top's first
		r.runOwnMise(d, "exec", "--", "aube", "install", "--frozen-lockfile")
	}
	for _, m := range r.goModules() {
		r.runOwnMise(m, "exec", "--", "go", "mod", "tidy", "-diff", "-e")
	}
	r.runTask("setup")
	r.runTask("test")
	r.check(false)
}

// runTask runs the repo's own task when it has one, and says whether it had.
func (r repo) runTask(task string) bool {
	has := exec.Command("mise", "tasks", "info", task)
	has.Dir = r.dir
	if has.Run() != nil {
		return false
	}
	fmt.Println("$ mise run " + task)
	run := exec.Command("mise", "run", task)
	run.Dir, run.Stdin, run.Stdout, run.Stderr = r.dir, os.Stdin, os.Stdout, os.Stderr
	if err := run.Run(); err != nil {
		fail(fmt.Errorf("mise run %s: %w", task, err))
	}
	return true
}
