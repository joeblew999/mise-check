package example

import (
	"testing"

	misecheck "github.com/joeblew999/mise-check"
)

// TestMise is all a repo writes to use misecheck: one line, run by a task of the mise file it checks
// (greet:check-mise), whose env says what to check.
func TestMise(t *testing.T) { misecheck.Check(t, misecheck.FromEnv()) }
