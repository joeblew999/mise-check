package example

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joeblew999/mise-check/walk"
)

// greetWalk is greet's walk: the template's numbered tasks (templates/.mise/tasks/greet.toml), in their order, through
// the repo's own mise file, each checked by what a person would see it do; on the template where it is (mise run
// template:e2e), and on a new repo made from nothing as docs/greet/README.md says (mise run new-repo:e2e). The walk
// package does the rest.
var greetWalk = walk.Walk[*greeter]{Steps: steps, New: newRepo, Prepare: withGo, Of: func(r walk.Repo) *greeter { return &greeter{r} }}

func TestWalk(t *testing.T)  { greetWalk.Run(t) }
func TestSteps(t *testing.T) { walk.CheckSteps(t, "templates/.mise/tasks/greet.toml", steps) }

// greeter is greet's walk of a repo: what each step's Check is given.
type greeter struct{ walk.Repo }

// steps are the template's numbered tasks, each checked. TestSteps fails unless they're exactly those, in their order.
var steps = []walk.Step[*greeter]{
	{Task: "greet:03-init", Check: func(g *greeter) {
		before := g.Snapshot()
		g.MustTask("greet:03-init")
		if changed := walk.Diff(before, g.Snapshot()); len(changed) > 0 {
			g.T.Fatalf("greet:03-init changed %s: the repo isn't what init writes", strings.Join(changed, ", "))
		}
		g.Pass("greet:03-init changed nothing: the repo is what init writes")
	}},
	{Task: "greet:04-check", Check: func(g *greeter) {
		if out := g.MustTask("greet:04-check"); !strings.Contains(out, "rules: pass") {
			g.T.Fatalf("greet:04-check: %s", out)
		}
		g.Pass("greet:04-check: the rules pass")
		g.MustTask("greet:verify")
		g.Pass("greet:verify: what the repo's GitHub workflow runs passes (its test task, then check)")
	}},
	{Task: "greet:05-show", Check: func(g *greeter) {
		var s struct {
			App           bool
			Next, Command string
		}
		g.TaskJSON(&s, "greet:05-show", "--json")
		if !s.App || s.Next != "mise run greet:06-hello" || s.Command != "go tool greet hello" {
			g.T.Fatalf("greet:05-show --json: %+v", s)
		}
		g.Pass("greet:05-show says next: %s (without mise: %s)", s.Next, s.Command)
	}},
	{Task: "greet:06-hello", Check: func(g *greeter) {
		if out := g.MustTask("greet:06-hello", "--name", "walk", "--shout"); !strings.Contains(out, "HELLO, WALK") {
			g.T.Fatalf("greet:06-hello --name walk --shout: %s", out)
		}
		g.Pass("greet:06-hello --name walk --shout: HELLO, WALK")
		if out := g.MustTask("start"); !strings.Contains(out, "hello, app") {
			g.T.Fatalf("the app (mise run start): %s", out)
		}
		g.Pass("the app greets with greet's module half, at the version go.mod pins: hello, app")
	}},
	{Task: "greet:07-bye", Check: func(g *greeter) {
		if out := g.MustTask("greet:07-bye"); !strings.Contains(out, "bye") {
			g.T.Fatalf("greet:07-bye: %s", out)
		}
		g.Pass("greet:07-bye: bye")
	}},
}

// withGo gives the template, where it is, the Go of the module it's a folder of. It has no go.mod of its own, and the
// walk cuts mise off from the repo around it, so its own mise files have no Go to give its tasks (go tool greet …);
// the Go this test runs with is that module's (its toolchain line). A new repo has a go.mod of its own, whose Go mise
// installs (.mise/config.toml), so it needs nothing.
func withGo(r walk.Repo) walk.Repo {
	if _, err := os.Stat(filepath.Join(r.Dir, "go.mod")); err == nil {
		return r
	}
	r.With = append(r.With, "MISE_GO_VERSION="+strings.TrimPrefix(runtime.Version(), "go"))
	return r
}

// greetTool is greet's tool line: the one line of a repo's go.mod that pins the command and the module.
const greetTool = "github.com/joeblew999/mise-check/example/cmd/greet"

// newRepo is a new repo, made as docs/greet/README.md says a person makes one: an empty folder with its own git, go mod
// init, greet's tool line, then go tool greet init and greet:template. The example isn't released, so its go.mod
// requires mise-check at this checkout (a replace, as a person trying a change does), and the tool line goes in with
// go mod edit, which needs no network. A tool that is released makes it at the release -version names instead (go get
// -tool …@version).
func newRepo(r walk.Repo, version string) walk.Repo {
	if version != "" {
		r.T.Fatalf("-version %s: the example isn't released, so a new repo is made at this checkout only", version)
	}
	n := r.New("greet-new")
	n.Must("go", "mod", "init", "greet-new")
	n.Must("go", "mod", "edit", "-require=github.com/joeblew999/mise-check@v0.0.0", "-replace=github.com/joeblew999/mise-check="+moduleRoot(r), "-tool="+greetTool)
	n.Must("go", "tool", "greet", "init")
	n.MustTask("greet:template")
	n.Pass("%s: an empty folder, its own git, go mod init, greet's tool line at this checkout, go tool greet init, then greet:template", n.Name)
	return n
}

// moduleRoot is the folder of the module this checkout is (mise-check's, in mise-check; a copy's own, in a copy).
func moduleRoot(r walk.Repo) string {
	r.T.Helper()
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = r.Top
	out, err := cmd.Output()
	if err != nil {
		r.T.Fatalf("go env GOMOD: %v", err)
	}
	return filepath.Dir(strings.TrimSpace(string(out)))
}
