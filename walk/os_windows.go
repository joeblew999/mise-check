package walk

import (
	"os/exec"
	"strconv"
	"syscall"
)

// OwnGroup starts cmd in a process group of its own, so KillGroup ends it with everything it started, not only mise:
// Windows' own, as os_unix.go is elsewhere.
func OwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// KillGroup kills cmd and every process under it (taskkill's tree).
func KillGroup(cmd *exec.Cmd) {
	exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}

// LongPath is p with Windows' long names (C:\Users\runneradmin, not C:\Users\RUNNER~1), as mise compares a ceiling.
func LongPath(p string) string {
	u, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return p
	}
	buf := make([]uint16, 1024)
	n, err := syscall.GetLongPathName(u, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || int(n) > len(buf) {
		return p
	}
	return syscall.UTF16ToString(buf[:n])
}
