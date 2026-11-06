//go:build !windows

package walk

import (
	"os/exec"
	"syscall"
)

// OwnGroup starts cmd in a process group of its own, so KillGroup ends it with everything it started (a server and
// what it runs), not only mise. os_windows.go is the same on Windows.
func OwnGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// KillGroup kills cmd's process group.
func KillGroup(cmd *exec.Cmd) { syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }

// LongPath is p as it is: only Windows has short names (os_windows.go).
func LongPath(p string) string { return p }
