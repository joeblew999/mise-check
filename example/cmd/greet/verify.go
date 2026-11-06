package main

import (
	"fmt"
	"os"
	"os/exec"
)

// verify is greet verify: what the repo's GitHub workflow runs once it has checked out, the repo's own setup and test
// tasks when it has them (with mise), then check. Run here, it does what GitHub does.
func verify(dir string) {
	if _, err := exec.LookPath("mise"); err == nil {
		runTask(dir, "setup")
		runTask(dir, "test")
	} else {
		fmt.Println("· no mise here: the repo's own tasks are left to it (go test ./...)")
	}
	check(dir, false)
}

// runTask runs the repo's own task when it has one.
func runTask(dir, task string) {
	has := exec.Command("mise", "tasks", "info", task)
	has.Dir = dir
	if has.Run() != nil {
		return
	}
	fmt.Println("$ mise run " + task)
	cmd := exec.Command("mise", "run", task)
	cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("mise run %s: %w", task, err))
	}
}
