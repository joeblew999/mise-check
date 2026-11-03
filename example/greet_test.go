package example

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestGreet runs this folder's tasks back by name, as a repo's end-to-end test does: Go runs mise, which runs Go.
// misecheck (greet:check-mise, GO_CALLERS=.) checks that every task name here is a task of mise.toml. Only greet:test
// runs it, which sets GREET_E2E.
func TestGreet(t *testing.T) {
	if os.Getenv("GREET_E2E") == "" {
		t.Skip("run by mise run greet:test")
	}
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"run", "greet:hello", "--name", "misecheck"}, "hello, misecheck"},
		{[]string{"run", "greet:hello", "--name", "misecheck", "--shout"}, "HELLO, MISECHECK"},
		{[]string{"run", "greet:bye"}, "bye"},
	} {
		out, err := exec.Command("mise", c.args...).Output()
		if err != nil || !strings.Contains(string(out), c.want) {
			t.Errorf("mise %s: %q, %v; want %q", strings.Join(c.args, " "), out, err, c.want)
		}
	}
}
