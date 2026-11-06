// The repo's mise files, checked by misecheck (github.com/joeblew999/mise-check), as any tool repo checks its own:
// the root's, and the template's tasks the strict way. A tool repo new-tool makes keeps this file.
package misecheck_test

import (
	"testing"

	misecheck "github.com/joeblew999/mise-check"
)

// TestRootMise checks this repo's own mise file: mise run root:check-mise runs it.
func TestRootMise(t *testing.T) { misecheck.Check(t, misecheck.Config{MiseFile: "mise.toml"}) }

// TestTemplateMise checks the template's tasks file the strict way, against the binary's Go (cmd/), the walk that
// runs them (this folder's tests), the skill and the docs: mise run template:check-mise runs it.
func TestTemplateMise(t *testing.T) {
	misecheck.Check(t, misecheck.Config{
		MiseFile:  "templates/.mise/tasks/mise-check.toml",
		GoCmd:     "cmd",
		GoLayout:  "file",
		GoCallers: ".",
		Skills:    []string{"templates/.claude/skills/mise-check/SKILL.md", "templates/docs/mise-check/README.md"},
	})
}
