// The root's tool: task that isn't a one-line command: mise run tool:release, as every mise tool repo's, on
// mise-check's release package. It runs only from its own task, so go test ./... never releases.

package misecheck_test

import (
	"testing"

	"github.com/joeblew999/mise-check/release"
)

// TestToolRelease: mise run tool:release releases mise-check from the commit checked out (release.yml runs the same on
// GitHub), by the release package: the commit's v* tag or the next patch, goreleaser's binaries on its GitHub release,
// a new repo made at it and walked (new-repo:e2e --version), the version on proxy.golang.org; then root:show-users, the
// owner's repos that could move to it. With --dry-run (the test's flag -dry-run) it tags and publishes nothing.
func TestToolRelease(t *testing.T) {
	release.Run(t, release.Config{NewRepo: "new-repo:e2e", After: []string{"root:show-users"}})
}
