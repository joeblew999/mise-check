// Package e2e is mise-check's end-to-end test, on its own walk package (github.com/joeblew999/mise-check/walk), as
// mise-cloudflare's is: the template's numbered tasks through the repo's own mise file, on the template where it is
// (mise run template:e2e) and on a new repo made from nothing as docs/mise-check/README.md says (mise run
// new-repo:e2e); and update on a new repo given old pins of every kind, each moved (mise run new-repo:e2e-update).
// Without REPO_PATH every test here skips, so go test ./... never walks.
package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/joeblew999/mise-check/walk"
)

// miseCheckWalk is the walk: the template's numbered tasks (templates/.mise/tasks/mise-check.toml), in their order,
// each checked by what a person would see it do.
var miseCheckWalk = walk.Walk[*walker]{Steps: steps, New: newRepo, Prepare: withGo, Of: func(r walk.Repo) *walker { return &walker{r} }}

func TestWalk(t *testing.T)  { miseCheckWalk.Run(t) }
func TestSteps(t *testing.T) { walk.CheckSteps(t, "../templates/.mise/tasks/mise-check.toml", steps) }

// walker is mise-check's walk of a repo: what each step's Check is given.
type walker struct{ walk.Repo }

// steps are the template's numbered tasks, each checked. TestSteps fails unless they're exactly those, in their order.
var steps = []walk.Step[*walker]{
	{Task: "mise-check:03-init", Check: func(w *walker) {
		before := w.Snapshot("node_modules")
		w.MustTask("mise-check:03-init")
		if changed := walk.Diff(before, w.Snapshot("node_modules")); len(changed) > 0 {
			w.T.Fatalf("mise-check:03-init changed %s: the repo isn't what init writes", strings.Join(changed, ", "))
		}
		w.Pass("mise-check:03-init changed nothing: the repo is what init writes")
	}},
	{Task: "mise-check:04-check", Check: func(w *walker) {
		if out := w.MustTask("mise-check:04-check"); !strings.Contains(out, "rules: pass") {
			w.T.Fatalf("mise-check:04-check: %s", out)
		}
		w.Pass("mise-check:04-check: the rules pass")
		w.MustTask("mise-check:verify")
		w.Pass("mise-check:verify: what the repo's GitHub workflow runs passes (its setup and test tasks, then check)")
	}},
	{Task: "mise-check:05-show", Check: func(w *walker) {
		var s struct {
			App  bool
			Next string
		}
		w.TaskJSON(&s, "mise-check:05-show", "--json")
		if !s.App || s.Next != "mise run mise-check:update" {
			w.T.Fatalf("mise-check:05-show --json: %+v", s)
		}
		w.Pass("mise-check:05-show says next: %s", s.Next)
		if out := w.MustTask("start"); !strings.Contains(out, "hello, app") {
			w.T.Fatalf("the app (mise run start): %s", out)
		}
		w.Pass("the app runs (mise run start): hello, app")
	}},
}

// withGo gives the template, where it is, the Go of the module it's a folder of. It has no go.mod of its own (one
// would make it a module apart from mise-check's, whose embed it is), and the walk cuts mise off from the repo around
// it, so its own mise files have no Go to give its tasks; the Go this test runs with is that module's (its toolchain
// line). A new repo has a go.mod of its own, whose Go mise installs, so it needs nothing.
func withGo(r walk.Repo) walk.Repo {
	if _, err := os.Stat(filepath.Join(r.Dir, "go.mod")); err == nil {
		return r
	}
	r.With = append(r.With, "MISE_GO_VERSION="+strings.TrimPrefix(runtime.Version(), "go"))
	return r
}

// newRepo is a new repo, made as docs/mise-check/README.md says a person makes one: an empty folder with its own git,
// mise-check init at the release version names (mise exec github:joeblew999/mise-check@<version> -- mise-check init,
// which pins that release), or at this checkout's build without one (local, which mise run tool:install links), then
// mise-check:template, which gives it the template's app, its packages and its go.mod.
func newRepo(r walk.Repo, version string) walk.Repo {
	at := "local"
	if version != "" {
		at = strings.TrimPrefix(version, "v")
	}
	n := r.New("mise-check-new")
	n.Must("mise", "exec", "github:joeblew999/mise-check@"+at, "--", "mise-check", "init")
	n.MustTask("mise-check:template")
	n.Pass("%s: an empty folder, its own git, mise-check init at %s, then mise-check:template", n.Name, at)
	return n
}

// TestUpdate is mise-check:update on a new repo given old pins of every kind it moves: a mise tool (node, in
// mise.toml), a package (wrangler, in package.json), and Go modules, one a major behind (go-chi/chi v1, its import in
// the app) and one a minor behind (google/uuid). Every one must move, chi to /v5 with its import rewritten, and
// mise-check:verify must pass after. Run by mise run new-repo:e2e-update (E2E_UPDATE=true).
func TestUpdate(t *testing.T) {
	path := os.Getenv("REPO_PATH")
	if path == "" || os.Getenv("E2E_UPDATE") != "true" {
		t.Skip("mise run new-repo:e2e-update runs it (REPO_PATH=../templates E2E_UPDATE=true)")
	}
	n := newRepo(walk.At(t, path), "")
	n.SetUp()

	n.Heading("Old pins of every kind")
	replace(t, filepath.Join(n.Dir, "mise.toml"), `node = "26.13.0"`, `node = "24.11.0"`)
	replace(t, filepath.Join(n.Dir, "package.json"), `"wrangler": "4.155.0"`, `"wrangler": "4.100.0"`)
	must(t, os.WriteFile(filepath.Join(n.Dir, "app", "deps.go"), []byte(`package main

import (
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

// router and id are the app's use of two modules: update moves both.
var router, id = chi.NewRouter(), uuid.NewString
`), 0o644))
	n.Must("mise", "exec", "--", "go", "get", "github.com/go-chi/chi@v1.5.4", "github.com/google/uuid@v1.3.0")
	n.Must("mise", "exec", "--", "go", "mod", "tidy")
	n.Pass("node 24.11.0 (mise.toml), wrangler 4.100.0 (package.json), go-chi/chi v1.5.4 and google/uuid v1.3.0 (go.mod)")

	// update refuses in CI, which only checks: here it's the walk's own repo, made to be updated.
	out, err := n.Task(n.Env("CI="), "", "mise-check:update")
	if err != nil {
		t.Fatalf("mise-check:update: %v\n%s", err, out)
	}
	for _, want := range []string{"node 24.11.0 → ", "wrangler 4.100.0 → ", "github.com/google/uuid v1.3.0 → ", "github.com/go-chi/chi v1.5.4 → none", "github.com/go-chi/chi/v5 none → v5."} {
		if !strings.Contains(out, want) {
			t.Errorf("mise-check:update didn't say %q", want)
		}
	}
	gomod := read(t, filepath.Join(n.Dir, "go.mod"))
	deps := read(t, filepath.Join(n.Dir, "app", "deps.go"))
	switch {
	case !regexp.MustCompile(`github.com/go-chi/chi/v5 v5\.`).MatchString(gomod):
		t.Errorf("go.mod has no go-chi/chi/v5:\n%s", gomod)
	case strings.Contains(gomod, "github.com/google/uuid v1.3.0"):
		t.Errorf("go.mod still has google/uuid v1.3.0:\n%s", gomod)
	case !strings.Contains(deps, `"github.com/go-chi/chi/v5"`):
		t.Errorf("app/deps.go's import of chi wasn't rewritten to /v5:\n%s", deps)
	case strings.Contains(read(t, filepath.Join(n.Dir, "mise.toml")), `node = "24.11.0"`):
		t.Error("mise.toml still pins node 24.11.0")
	case strings.Contains(read(t, filepath.Join(n.Dir, "package.json")), `"4.100.0"`):
		t.Error("package.json still pins wrangler 4.100.0")
	}
	if t.Failed() {
		t.FailNow()
	}
	n.Pass("every pin moved: node, wrangler, google/uuid, and go-chi/chi to /v5 with app/deps.go's import rewritten")
	n.MustTask("mise-check:verify")
	n.Pass("mise-check:verify passes after: setup, the app's tests, then check")
}

func replace(t *testing.T, path, old, new string) {
	t.Helper()
	s := read(t, path)
	if strings.Count(s, old) != 1 {
		t.Fatalf("%s: %q isn't there exactly once", path, old)
	}
	must(t, os.WriteFile(path, []byte(strings.Replace(s, old, new, 1)), 0o644))
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	must(t, err)
	return string(b)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
