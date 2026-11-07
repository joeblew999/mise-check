// Package release releases a mise tool from the commit checked out, as every mise tool repo's tool:release task runs
// it, and its release.yml on GitHub: the commit's v* tag, or the next patch version, tagged and pushed, when it has
// none; goreleaser's binaries for every machine on that tag's GitHub release (the repo's .goreleaser.yaml), where mise's
// github backend finds them; a new repo made at the release and walked, by the task that walks one (its --version);
// the version fetched by proxy.golang.org, so go get and pkg.go.dev have it at once; then any task the tool runs after a
// release. With the test's flag -dry-run it tags and publishes nothing: goreleaser builds a snapshot into dist/.
//
// A tool's release is one test of its own, run only by its task:
//
//	func TestToolRelease(t *testing.T) { release.Run(t, release.Config{NewRepo: "new-repo:e2e"}) }
//
//	[tasks."tool:release"]
//	usage = 'flag "--dry-run" help="tag and publish nothing: goreleaser builds a snapshot into dist/"'
//	run = 'go test . -run "^TestToolRelease$" -count=1 -v -timeout 30m{% if usage.dry_run %} -args -dry-run{% endif %}'
package release

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/joeblew999/mise-check/walk"
)

// Config is what a tool's release runs of its own: its tasks, by name.
type Config struct {
	NewRepo string   // the task that walks a new repo made at a release, given --version <tag>: "new-repo:e2e"
	After   []string // tasks run after the release, in turn (mise-check's root:show-users); optional
}

// DryRun is the test's flag -dry-run, which the tool:release task passes for its --dry-run: tag nothing, publish
// nothing, and build a goreleaser snapshot into dist/.
var DryRun = flag.Bool("dry-run", false, "tag nothing and publish nothing: goreleaser builds a snapshot into dist/")

// Run releases the tool from the commit checked out (see the package), or with -dry-run only says what it would
// release and builds a snapshot. It runs only when its task names the test (walk.OnlyByName), so go test ./... never
// releases, and it stops the test at the first step that fails.
func Run(t *testing.T, c Config) {
	walk.OnlyByName(t)
	if c.NewRepo == "" {
		t.Fatal("release.Config.NewRepo is the task that walks a new repo made at a release (new-repo:e2e): every release is tried as a person tries it")
	}
	tag, isNew, err := Tag()
	if err != nil {
		t.Fatal(err)
	}
	if *DryRun {
		fmt.Printf("· would release %s%s\n", tag, map[bool]string{true: " (a new tag on this commit)", false: " (this commit's tag)"}[isNew])
		run(t, nil, "goreleaser", "release", "--snapshot", "--clean")
		fmt.Println("· dry run: nothing tagged, nothing published")
		return
	}
	if isNew {
		var identity []string
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			identity = []string{"-c", "user.name=github-actions", "-c", "user.email=github-actions@users.noreply.github.com"}
		}
		run(t, nil, "git", append(identity, "tag", "-a", tag, "-m", tag)...)
		run(t, nil, "git", "push", "-q", "origin", tag)
		fmt.Println("tagged", tag)
	}
	run(t, []string{"GORELEASER_CURRENT_TAG=" + tag}, "goreleaser", "release", "--clean")
	run(t, nil, "mise", "run", c.NewRepo, "--version", tag)
	module, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	run(t, []string{"GOPROXY=https://proxy.golang.org", "GOFLAGS=-mod=mod"}, "go", "list", "-m", strings.TrimSpace(string(module))+"@"+tag)
	for _, task := range c.After {
		run(t, nil, "mise", "run", task)
	}
}

// Tag is the tag to release: the commit's own v* tag (isNew false), or the next patch version after the repo's newest
// v* tag, v0.0.1 for the first (isNew true).
func Tag() (tag string, isNew bool, err error) {
	if out, err := exec.Command("git", "describe", "--tags", "--exact-match", "--match", "v*").Output(); err == nil {
		return strings.TrimSpace(string(out)), false, nil
	}
	out, err := exec.Command("git", "tag", "-l", "v*").Output()
	if err != nil {
		return "", false, fmt.Errorf("git tag -l: %w", err)
	}
	return Next(strings.Fields(string(out))), true, nil
}

// Next is the patch version after the newest of tags (vMAJOR.MINOR.PATCH; others aren't releases): v0.0.1 for none.
func Next(tags []string) string {
	newest := [3]int{}
	for _, t := range tags {
		if v, ok := parse(t); ok && less(newest, v) {
			newest = v
		}
	}
	return fmt.Sprintf("v%d.%d.%d", newest[0], newest[1], newest[2]+1)
}

// parse reads vMAJOR.MINOR.PATCH; anything else isn't a release tag.
func parse(tag string) ([3]int, bool) {
	parts := strings.Split(strings.TrimPrefix(tag, "v"), ".")
	if !strings.HasPrefix(tag, "v") || len(parts) != 3 {
		return [3]int{}, false
	}
	var v [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

// less reports whether release a is before release b.
func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// run runs a command with env added to this one's, its output shown, and stops the test at its failure.
func run(t *testing.T, env []string, name string, args ...string) {
	t.Helper()
	fmt.Println("$", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}
