package example

import (
	"testing"

	misecheck "github.com/joeblew999/mise-check"
)

// TestMise is all a repo writes to use misecheck: one line, run by a task of its mise file whose env says what to check
// (template:check-mise checks the template's tasks file, root:check-mise this repo's own mise.toml).
func TestMise(t *testing.T) { misecheck.Check(t, misecheck.FromEnv()) }
