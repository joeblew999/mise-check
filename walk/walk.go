// Package walk is the end-to-end test of a repo through its own mise file, the way a person there uses it: its tasks
// run by name in the order a person takes them (mise run greet:init, greet:check, …), each checked by what it should
// have done. The order is the walk's, never the tasks' names: no task is numbered. A tool's repo walks its template where it is, and a new repo made from nothing as its README says, so every
// step a user takes is known to work, through the tasks they run.
//
// It is half of mise-check, which a repo uses through mise, never through this package alone: at the repo's top,
// mise exec "github:joeblew999/mise-check[minimum_release_age=0s]@latest" -- mise-check init writes its tasks and pins it, then mise run
// mise-check says what's done and what to run next. A tool repo's tests import this package (and misecheck, whose
// Check fails without init's tasks) at the release init pinned: https://github.com/joeblew999/mise-check#use-it-in-your-repo.
//
// It is mise-cloudflare's walk with everything of Cloudflare's taken out. The tool's own test says which steps there
// are, which task each runs and what it checks; walk does the rest:
//
//   - a repo with its own mise only, cut off from the repo around it (MISE_CEILING_PATHS), as it would be on its own:
//     At (a folder of the tool's checkout, its template) and New (a new repo, an empty folder with its own git);
//   - its commands and tasks run by name, their output shown as they go (Run, Must, Task, MustTask, Stdout, TaskJSON);
//   - its setup as a person does it (SetUp: mise install, then its setup task);
//   - the steps in their order (Steps), and the files a step changed (Snapshot, Diff);
//   - what each OS does its own way (OwnGroup, KillGroup, LongPath).
//
// A tool's whole walk is one value, its steps and how a person makes a new repo with it, so the tool's own code is
// as little as it can be:
//
//	var greetWalk = walk.Walk[*greeter]{Steps: steps, New: newRepo, Of: func(r walk.Repo) *greeter { return &greeter{r} }}
//
//	func TestWalkTemplate(t *testing.T) { greetWalk.Template(t, "templates") }
//	func TestWalkNewRepo(t *testing.T)   { greetWalk.NewRepo(t) }
//	func TestSteps(t *testing.T) {
//		walk.CheckSteps(t, "templates/.mise/tasks/greet.toml", steps, map[string]string{"greet:help": "it only prints the commands"})
//	}
//
// Which repo is the test's name, and the task that runs it names it (go test . -run "^TestWalkTemplate$"); how far is
// the test's flag -stage, and the release a new repo is made at its flag -version, both from the task's own usage
// (-args -stage deploy). A walk runs only when its test is named, so go test ./... never walks (OnlyByName).
//
// The tool's test names each task in full ("greet:init"), never glued from pieces, so misecheck (GoCallers) checks
// every name against the tasks file, and CheckSteps holds the steps to that file's tasks: each step a task, and every
// task a step or in a short list of the ones not walked, each with why.
package walk

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Repo is a repo walked through its own mise file: the checkout the test runs in (Top, the tool's repo), what the walk
// calls the repo, and its folder.
type Repo struct {
	T    *testing.T
	Top  string // the tool's checkout, where the test runs
	Name string // the repo, as the walk names it: its folder in Top (templates), or a new repo's
	Dir  string // its folder
	// With is env every command in the repo gets, after Env's own: what the tool's Prepare gives it (the Go of the
	// module a template is a folder of, say).
	With []string
}

// At is the repo at path, a folder of the checkout the test runs in ("templates": the template, where it is; ".": the
// checkout itself).
func At(t *testing.T, path string) Repo {
	t.Helper()
	if path == "" {
		t.Fatal("no repo to walk: At takes a folder of the checkout (templates)")
	}
	top, err := filepath.Abs(".")
	if err == nil {
		// mise stops only at a ceiling given as the real path, in long names (macOS's /var is /private/var).
		top, err = filepath.EvalSymlinks(LongPath(top))
	}
	if err != nil {
		t.Fatal(err)
	}
	return Repo{T: t, Top: top, Name: path, Dir: filepath.Join(top, path)}
}

// New is a new repo, as a person starts one: an empty folder named folder, with its own git, in a folder of its own
// that goes when the test ends. What makes it the tool's (its init, as its README says) the tool's test runs in it next,
// with Must.
func (r Repo) New(folder string) Repo {
	r.T.Helper()
	tmp, err := filepath.EvalSymlinks(LongPath(r.T.TempDir()))
	if err != nil {
		r.T.Fatal(err)
	}
	n := Repo{T: r.T, Top: r.Top, Name: folder + " (a new repo)", Dir: filepath.Join(tmp, folder)}
	if err := os.Mkdir(n.Dir, 0o755); err != nil {
		r.T.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", n.Dir, "init", "-q").CombinedOutput(); err != nil {
		r.T.Fatalf("git init: %v\n%s", err, out)
	}
	return n
}

// Env is what every command in the repo runs with: this process's environment, with mise reading the repo's own config
// files only (MISE_CEILING_PATHS, its parent), trusting them (MISE_TRUSTED_CONFIG_PATHS) and asking nothing
// (MISE_YES), then env.
func (r Repo) Env(env ...string) []string {
	out := append(os.Environ(), "MISE_CEILING_PATHS="+filepath.Dir(r.Dir), "MISE_TRUSTED_CONFIG_PATHS="+r.Dir, "MISE_YES=1")
	return append(append(out, r.With...), env...)
}

// Exec runs a command in dir with env (the whole environment), input on its stdin, its output shown as it goes and
// returned: the general form that Run and Task use.
func Exec(dir string, env []string, input, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stdout, &buf)
	err := cmd.Run()
	if out := buf.String(); out != "" && !strings.HasSuffix(out, "\n") {
		fmt.Println() // a value printed bare, so the next line starts on its own
	}
	return buf.String(), err
}

// Run runs a command in the repo, on its own mise, with env added.
func (r Repo) Run(env []string, name string, args ...string) (string, error) {
	return Exec(r.Dir, r.Env(env...), "", name, args...)
}

// Must is Run, stopping the test when the command fails.
func (r Repo) Must(name string, args ...string) string {
	r.T.Helper()
	out, err := r.Run(nil, name, args...)
	if err != nil {
		r.T.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return out
}

// Task runs the repo's own task, named in full (greet:04-check), as a person there runs it, with env added and input
// typed into it (a secret's value, when it asks): mise run greet:04-check.
func (r Repo) Task(env []string, input, task string, args ...string) (string, error) {
	fmt.Printf("$ mise run %s   (in %s)\n", strings.TrimSpace(task+" "+strings.Join(args, " ")), r.Name)
	return Exec(r.Dir, r.Env(env...), input, "mise", append([]string{"run", task}, args...)...)
}

// MustTask is Task, stopping the test when the task fails.
func (r Repo) MustTask(task string, args ...string) string {
	r.T.Helper()
	out, err := r.Task(nil, "", task, args...)
	if err != nil {
		r.T.Fatalf("%s %s: %v", task, strings.Join(args, " "), err)
	}
	return out
}

// Stdout is what a command prints on stdout in the repo, on its own mise, without showing it; its stderr shows. mise
// prints what it runs on stderr (MISE_TASK_OUTPUT=interleave keeps a task's stdout unprefixed), so a task's JSON reads
// as JSON.
func (r Repo) Stdout(name string, args ...string) []byte {
	r.T.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env, cmd.Stderr = r.Dir, r.Env("MISE_TASK_OUTPUT=interleave"), os.Stderr
	out, err := cmd.Output()
	if err != nil {
		r.T.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return out
}

// TaskJSON decodes into v the JSON document a task prints on stdout (from its first {), as its --json flag asks.
func (r Repo) TaskJSON(v any, task string, args ...string) {
	r.T.Helper()
	out := r.Stdout("mise", append([]string{"run", task}, args...)...)
	i := bytes.IndexByte(out, '{')
	if i < 0 {
		r.T.Fatalf("mise run %s %s printed no JSON: %s", task, strings.Join(args, " "), out)
	}
	if err := json.NewDecoder(bytes.NewReader(out[i:])).Decode(v); err != nil {
		r.T.Fatalf("mise run %s %s: %v\n%s", task, strings.Join(args, " "), err, out)
	}
}

// HasTask reports whether the repo's mise file has the task.
func (r Repo) HasTask(task string) bool {
	cmd := exec.Command("mise", "tasks", "info", task)
	cmd.Dir, cmd.Env = r.Dir, r.Env()
	return cmd.Run() == nil
}

// SetUp sets the repo up as a person does, with its own mise file: mise install, then mise run setup when it has a
// setup task.
func (r Repo) SetUp() {
	r.T.Helper()
	r.Must("mise", "install")
	how := "mise install"
	if r.HasTask("setup") {
		r.MustTask("setup")
		how += ", mise run setup"
	}
	r.Pass("%s is set up: %s", r.Name, how)
}

// Heading starts a part of the walk's output.
// NeedLocal fails the walk, saying what to run, unless this checkout's build of the tool ("github:owner/name") is
// installed as the mise version local, which the walk's mise exec tool@local runs: mise run tool:install makes it, and
// the walk's own task runs it first, but go test run by hand doesn't.
func (r Repo) NeedLocal(tool string) {
	r.T.Helper()
	if _, err := r.Run(nil, "mise", "where", tool+"@local"); err != nil {
		r.T.Fatalf("%s@local isn't installed: this checkout's build, which the walk runs, is made by mise run tool:install (the walk's own task runs it first)", tool)
	}
}

func (r Repo) Heading(format string, a ...any) { fmt.Printf("\n=== %s\n", fmt.Sprintf(format, a...)) }

// Pass says what the walk has seen work.
func (r Repo) Pass(format string, a ...any) { fmt.Printf("PASS %s\n", fmt.Sprintf(format, a...)) }

// Step is one task of the repo's mise file, in the walk's order: the task, in full (greet:init), the first stage of the walk
// that runs it, and how the walk runs it and checks it did what it says. A step the walk can't show working says why
// in Skip, instead of a Check.
type Step[W any] struct {
	Task  string
	Stage int
	Skip  string
	Check func(w W)
}

// Steps runs the steps in their order, as far as the stage upTo: each one's Check, under a heading of its task. A step
// of a later stage, or one with a Skip, is said and not run. w is what each Check is given: the tool's own walk, which
// has the Repo in it and what the steps leave for the steps after them.
func Steps[W any](r Repo, w W, steps []Step[W], upTo int) {
	r.T.Helper()
	for _, s := range steps {
		switch {
		case s.Stage > upTo:
			r.Pass("%s: for a later stage of the walk than this one", s.Task)
		case s.Skip != "":
			r.Pass("%s: %s", s.Task, s.Skip)
		case s.Check == nil:
			r.T.Fatalf("%s has no Check and no Skip", s.Task)
		default:
			r.Heading("%s", s.Task)
			s.Check(w)
		}
	}
	fmt.Println("\nALL PASSED")
}

// Version is the release a new repo is made at: the test's flag -version, which the task that runs the walk passes
// from its own usage (--version v0.4.0 → go test … -args -version "v0.4.0"). Empty, it's this checkout's build.
var Version = flag.String("version", "", "the release a new repo is made at (default this checkout's build)")

// Stage is how far the walk goes: the test's flag -stage, one of the Walk's Stages, which the task that runs it passes
// (go test … -args -stage deploy). Empty, the first.
var Stage = flag.String("stage", "", "how far the walk goes, one of its stages (default the first)")

// Walk is a tool's whole walk: its steps, the stages they're in, how a person makes a new repo with the tool (as its
// README says), and the tool's own walk type each step's Check is given. Template and NewRepo do the rest, so each of
// a tool's walk tests is one line: func TestWalkTemplate(t *testing.T) { greetWalk.Template(t, "templates") }.
type Walk[W any] struct {
	Steps  []Step[W]
	Stages []string                          // the stages, as -stage names them, in order; none, one stage
	New    func(r Repo, version string) Repo // a new repo, made from nothing at version ("": this checkout's build)
	// Prepare, when there is one, makes the repo ready before it's set up: env it needs (Repo.With) that its own
	// mise files can't give it, cut off from the repo around it.
	Prepare func(r Repo) Repo
	Of      func(r Repo) W // the tool's walk of a repo, which each Check is given
}

// Template walks path, a folder of this checkout (templates: the template, where it is): set up as a person does
// (SetUp), then the steps as far as -stage. It runs only when its test is named (OnlyByName).
func (w Walk[W]) Template(t *testing.T, path string) {
	t.Helper()
	OnlyByName(t)
	if *Version != "" {
		t.Fatalf("-version %s is the release a new repo is made at, and this walks %s where it is", *Version, path)
	}
	w.walk(At(t, path))
}

// NewRepo walks a new repo, made from nothing by New at -version (this checkout's build, without it): set up as a
// person does (SetUp), then the steps as far as -stage. It runs only when its test is named (OnlyByName).
func (w Walk[W]) NewRepo(t *testing.T) {
	t.Helper()
	OnlyByName(t)
	if w.New == nil {
		t.Fatal("the walk has no New: how a person makes a new repo with the tool")
	}
	w.walk(w.New(At(t, "."), *Version))
}

func (w Walk[W]) walk(r Repo) {
	r.T.Helper()
	upTo := 0
	if *Stage != "" {
		if upTo = slices.Index(w.Stages, *Stage); upTo < 0 {
			r.T.Fatalf("-stage %s: the stages are %s", *Stage, strings.Join(w.Stages, ", "))
		}
	}
	if w.Prepare != nil {
		r = w.Prepare(r)
	}
	r.SetUp()
	Steps(r, w.Of(r), w.Steps, upTo)
}

// OnlyByName skips the test unless go test's -run names it alone ("^TestWalkTemplate$", as the task that runs it
// does), so go test ./... never walks, deploys or releases: a test that changes things runs only when asked for.
func OnlyByName(t *testing.T) {
	t.Helper()
	run := ""
	if f := flag.Lookup("test.run"); f != nil {
		run = f.Value.String()
	}
	if strings.TrimSuffix(strings.TrimPrefix(run, "^"), "$") != t.Name() {
		t.Skipf("runs only when named: go test -run '^%s$' (the task that runs it names it)", t.Name())
	}
}

// CheckSteps fails unless the walk is the mise file's whole walk: each step is a task of the file, once, and every
// task of the file is a step or one of notWalked, the tasks the walk doesn't run, each with why ("greet:help": "it
// only prints the commands"). A task added, renamed or removed, and the walk says so. The steps' order is the test's
// own, the order a person takes them, as the tool's docs page and what mise run <namespace> says next give it: never
// the tasks' names.
func CheckSteps[W any](t *testing.T, file string, steps []Step[W], notWalked map[string]string) {
	t.Helper()
	tasks, err := tasksIn(file)
	if err != nil {
		t.Fatal(err)
	}
	var walked []string
	for _, s := range steps {
		walked = append(walked, s.Task)
	}
	for _, p := range stepsProblems(file, tasks, walked, notWalked) {
		t.Error(p)
	}
}

// stepsProblems are what keeps walked (the steps' tasks) and notWalked from being exactly the tasks of file, in
// tasks, each saying what to change.
func stepsProblems(file string, tasks, walked []string, notWalked map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range walked {
		switch {
		case !slices.Contains(tasks, s):
			out = append(out, fmt.Sprintf("the walk's step %s isn't a task of %s: rename the step, or add the task", s, file))
		case seen[s]:
			out = append(out, fmt.Sprintf("the walk has %s as a step twice", s))
		case notWalked[s] != "":
			out = append(out, fmt.Sprintf("%s is a step and in notWalked: take it out of notWalked", s))
		}
		seen[s] = true
	}
	for s, why := range notWalked {
		if !slices.Contains(tasks, s) {
			out = append(out, fmt.Sprintf("notWalked names %s, which isn't a task of %s: take it out", s, file))
		} else if strings.TrimSpace(why) == "" {
			out = append(out, fmt.Sprintf("notWalked names %s without saying why it isn't walked", s))
		}
	}
	for _, s := range tasks {
		if !seen[s] && notWalked[s] == "" {
			out = append(out, fmt.Sprintf("%s's task %s is neither a step of the walk nor in notWalked: walk it, in the order a person takes it, or say in notWalked why not", file, s))
		}
	}
	return out
}

// taskHeader is a task's table: ["name"] or [name] in a file of tasks only, [tasks."name"] or [tasks.name] in a
// mise.toml, which has other tables too.
var taskHeader = regexp.MustCompile(`^\[(tasks\.)?(?:"([^"]+)"|([A-Za-z0-9_:-]+))\]$`)

// tasksIn is a mise file's tasks, in their order in it: a mise.toml's [tasks.…], or every table of a file of tasks
// only.
func tasksIn(file string) ([]string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	config := regexp.MustCompile(`(?m)^\[tasks\.`).MatchString(s)
	var tasks []string
	for _, l := range strings.Split(s, "\n") {
		if m := taskHeader.FindStringSubmatch(l); m != nil && (m[1] != "") == config {
			tasks = append(tasks, m[2]+m[3])
		}
	}
	return tasks, nil
}

// Files is a repo's files by their content: the path from its folder, and its hash.
type Files map[string][32]byte

// Snapshot is the repo's files by their content, all but its git and the folders and files named in skip, wherever
// they are (installed packages, what a local run leaves): Diff says what a step changed.
func (r Repo) Snapshot(skip ...string) Files {
	r.T.Helper()
	out := Files{}
	err := filepath.WalkDir(r.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" || slices.Contains(skip, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(r.Dir, p)
		out[filepath.ToSlash(rel)] = sha256.Sum256(b)
		return nil
	})
	if err != nil {
		r.T.Fatal(err)
	}
	return out
}

// Diff names the files added, removed or changed between two snapshots, in order.
func Diff(a, b Files) []string {
	var changed []string
	for p, h := range b {
		if a[p] != h {
			changed = append(changed, p)
		}
	}
	for p := range a {
		if _, ok := b[p]; !ok {
			changed = append(changed, p)
		}
	}
	slices.Sort(changed)
	return changed
}

// CopyRepo copies a repo's files from src into dst as its owner has them: not its git, nor the folders and files named
// in skip (installed packages, local state), nor a link.
func CopyRepo(src, dst string, skip ...string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" || slices.Contains(skip, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, info.Mode().Perm())
	})
}
