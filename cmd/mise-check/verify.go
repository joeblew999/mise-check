package main

import (
	"fmt"
	"os"
	"os/exec"
)

// verify is mise-check verify: what the repo's GitHub workflow runs once it has checked out and installed mise, the
// repo's own setup and test tasks when it has them, then check. Run here, it does what GitHub does.
func (r repo) verify() {
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
