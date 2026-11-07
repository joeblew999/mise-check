// Package misecheck forces one structure on a mise file and the Go its tasks run, so the two can't drift apart and
// every task runs the same on macOS, Linux and Windows. That is all it does: it checks the shape, never what the Go
// does.
//
// It is half of mise-check, which a repo uses through mise, never through this package alone. At the repo's top:
//
//	mise exec "github:joeblew999/mise-check[minimum_release_age=0s]@latest" -- mise-check init   # its tasks, its pin
//	mise run mise-check                                                # what's done, and what to run next
//
// This package is the strict checks a tool repo adds in its own tests, required in go.mod at the release init pinned.
// Check fails in a repo that has this package without mise-check's tasks, or with the two pins apart, and says what
// to run: https://github.com/joeblew999/mise-check#use-it-in-your-repo.
//
// A repo calls it from a test of its own, one for each mise file it checks, saying in Go what to check, and a task of
// its mise file runs that test by name:
//
//	func TestRootMise(t *testing.T) { misecheck.Check(t, misecheck.Config{MiseFile: "mise.toml"}) }
//
//	[tasks."root:check-mise"]
//	run = 'go test . -run "^TestRootMise$" -count=1 -v'
//
// Three sides, each checked against the others:
//
//   - the mise file, the interface: each task is one line that runs something, with the flags and arguments it takes
//     declared in its usage;
//   - the Go its tasks run;
//   - Go that runs the tasks by name, an end-to-end test say (optional, GoCallers).
//
// How it's used, in mise-cloudflare say: a loop that runs both ways, mise to Go and back, several levels deep. Its
// root mise.toml runs Go tests (mise run template:e2e-serve runs TestWalkTemplate), which run its template's tasks by name
// (cloudflare:06-serve), which run its tool's Go (mise-cloudflare serve). A task of each mise file runs Check on it,
// even on the file that holds that task: root:check-mise checks the mise.toml it's in, and template:check-mise checks
// the template's tasks against the Go they run and against the end-to-end Go that runs them, so every link is checked
// in both directions.
//
// It doesn't prescribe one way to lay things out; it accepts many and holds whichever a repo picks to it. Each command
// of a task's run line (they can be joined by &&, || and ;) is read on its own:
//
//   - Go from its source: go run, go test or go build, on a folder (./cmd/mise-cloudflare), several, a whole tree
//     (./...), or none, the folder the task runs in. Each folder must hold Go; nothing else is asked of its layout,
//     so a binary that is one main.go fits as it is.
//   - A binary of GoCmd: <binary> <command>, or <binary> alone for its main.go, built and on the PATH; or the same
//     through the module's tool line, go tool <binary> <command>, which Go builds at the version go.mod pins (a repo's
//     own, or one a repo gets from the module it imports, so go.sum and the compiler check it). Its commands are laid out the
//     way GoLayout says: file, a file each, <GoCmd>/<binary>/<command>.go (dashes as underscores) holding the
//     function named for it (set_secrets.go, setSecrets); or folder, a folder each, <GoCmd>/<binary>/<command>/.
//     Code the commands share isn't a command and is left alone. This is the strictest: every command a task runs
//     must be there, every command must be one a task runs, and every flag must be declared on both sides, in the
//     usage of the tasks running the binary and in its Go (flag.Bool("yes", …)).
//   - Anything else (hugo …, wrangler …, go vet …, go tool of a binary that isn't GoCmd's): logged as not checked,
//     never failed.
//
// The tasks can be in a mise.toml ([tasks.…]) or in a file of tasks only (.mise/tasks/*.toml, or one a mise.toml
// includes); GoCmd and GoCallers are each optional, so a repo checks as much as it has. A file of tasks that only run
// tools (hugo …, process-compose …), with no GoCmd, is held to the rules every task is, below; one given GoCmd,
// whose tasks then run no Go at all, fails: the Go its binary's tasks ran has moved.
//
// Every task, however it runs Go:
//
//   - its run and its usage are each one line, "…" or '…': longer logic belongs in Go, not in the mise file. One it
//     can't read fails, rather than being skipped;
//   - it declares the flags and arguments it takes in mise's usage, and its run passes each by name with mise's own
//     templates, which mise fills in before any shell runs: {% if usage.yes %} --yes{% endif %}, "{{usage.branch}}".
//     Everything declared is passed, nothing passed is undeclared, and nothing is read mise's deprecated way,
//     {{flag(…)}};
//   - its run has no POSIX shell syntax (${…}, $name, NAME=value in front of a command), which Windows' cmd doesn't
//     read: variables go in the task's env.
//
// Go that runs tasks (GoCallers) names each in full, "namespace:task", and every name must be a task of the mise
// file. A name glued together from pieces (the namespace's string plus a variable) can't be checked, so it fails, and
// so does a folder of callers that names no task at all.
//
// It knows nothing of any repo. The repo's test says what to check, in a Config:
//
//	MiseFile   the mise file. Its tasks run in its own folder, or, for a .mise/tasks/*.toml, in the project's root; a
//	           task's ./path is read from there
//	GoCmd      the folder of the built binaries, <GoCmd>/<binary>/   } for built binaries: both, or neither
//	GoLayout   file or folder: how a binary's commands are laid out    }
//	GoCallers  the folder of Go that runs the tasks by name (optional)
//	GoBinaries which binaries of GoCmd to hold to the strict way (optional; none, all of them): for a repo taking it
//	           up one binary at a time
//	Skills     the agent skill of the tasks, and any page that must cover them too (optional)
//
// A task's go test -run "^TestX$" must name a test there is, in the folder it tests: a test renamed, and the task
// that runs it says so, as a command renamed does.
//
// On the Go side it reads only Go: .go files and Go's flag package. It never passes on nothing, and it logs what it
// checked.
package misecheck

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Config says what Check checks: the mise file, and the Go on the other side of it, written in the repo's test.
type Config struct {
	MiseFile  string // the mise file
	GoCmd     string // the folder of the built binaries, <GoCmd>/<binary>/   } for built binaries:
	GoLayout  string // how a binary's commands are laid out, file or folder   } both, or neither
	GoCallers string // the folder of Go that runs the tasks by name (optional)
	// GoBinaries are which binaries of GoCmd to hold to the strict way (optional; none, all of them). For a repo taking
	// it up one binary at a time: the others are left alone, as anything else a task runs is.
	GoBinaries []string
	// Skills are the agent skill of these tasks (a SKILL.md), and any other page that must cover them (the tool's
	// docs), each held to them both ways: every task and flag of the mise file is named in it, and every task it
	// names, in full ("namespace:task"), is one (optional).
	Skills []string
	// Names are other files that name these tasks without covering them all (a hook, a workflow, a config file's
	// comments), held to them one way: every task one names, as "mise run namespace:task" or in full in a namespace
	// of the mise file, is one, so none points a person or an agent at a task that isn't there (optional).
	Names []string
}

// Check checks the mise file c names against its Go, both ways, by the rules of the package documentation. A repo
// calls it from a test of its own: func TestRootMise(t *testing.T) { misecheck.Check(t, misecheck.Config{MiseFile: "mise.toml"}) }.
func Check(t testing.TB, c Config) {
	t.Helper()
	file, cmd, layout, callers := c.MiseFile, c.GoCmd, c.GoLayout, c.GoCallers
	if file == "" {
		t.Fatal("Config.MiseFile is empty: the test says which mise file to check")
	}
	checkThroughMise(t, ".")
	if (cmd == "") != (layout == "") {
		t.Fatalf("GoCmd %q GoLayout %q: give both, for tasks that run a built binary, or neither", cmd, layout)
	}
	if len(c.GoBinaries) > 0 && cmd == "" {
		t.Fatalf("GoBinaries %v: it picks binaries of GoCmd, and there's no GoCmd", c.GoBinaries)
	}
	if layout != "" && layout != "file" && layout != "folder" {
		t.Fatalf("GoLayout %q: it's file (a command is <GoCmd>/<binary>/<command>.go) or folder (<GoCmd>/<binary>/<command>/)", layout)
	}
	tasks, unreadable, err := readTasks(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range unreadable {
		t.Error(u)
	}
	if len(tasks) == 0 {
		t.Errorf("MiseFile %s has no tasks: did it move?", file)
	}
	g := goSide{cmd: cmd, layout: layout}
	if cmd != "" {
		if g.binaries = binariesIn(cmd); len(g.binaries) == 0 {
			t.Errorf("GoCmd %s holds no binary (a folder %s/<binary>/): did the Go move?", cmd, cmd)
		}
		if len(c.GoBinaries) > 0 {
			var picked []string
			for _, b := range c.GoBinaries {
				if !slices.Contains(g.binaries, b) {
					t.Errorf("GoBinaries names %s, which isn't a binary of GoCmd %s (%s/%s/)", b, cmd, cmd, b)
				} else {
					picked = append(picked, b)
				}
			}
			t.Logf("GoBinaries: %d of the %d binaries of %s held to the strict way: %s", len(picked), len(g.binaries), cmd, strings.Join(picked, ", "))
			g.binaries = picked
		}
	}
	checkMiseToGo(t, file, tasks, g)
	if cmd != "" {
		checkGoToMise(t, file, tasks, g)
	}
	checkUsage(t, file, tasks, g)
	for _, skill := range c.Skills {
		checkSkill(t, file, tasks, skill)
	}
	for _, f := range c.Names {
		checkNames(t, file, tasks, f)
	}
	if callers == "" {
		t.Logf("Go → mise: no GoCallers, so no Go that runs %s by name is checked", file)
		return
	}
	checkGoCallsMise(t, file, tasks, callers)
}

// ---- The two forms ----

// goSide is the Go a mise file's built binaries are: their folder (cmd), their layout, and the binaries in it. Empty,
// the mise file runs no built binary of ours.
type goSide struct {
	cmd, layout string
	binaries    []string
}

// ran is what one command of a task's run line runs: a place of Go (a folder, or a binary command's file or folder),
// and the binary when the place is one of ours; or, in neither form, the command, in notGo.
type ran struct {
	place, binary, notGo string
	tree                 bool   // ./... or ./path/...: Go anywhere below place, not in place itself
	test                 string // go test -run "^TestX$": the test it runs, TestX
}

// runsOf reads a run line command by command (apart by &&, || and ;), each in one of the two forms:
//
//  1. Go from its source: go run ./path, go test ./path, go build ./path → the folder path/, from dir, the folder the
//     task runs in (./... is all of it, and no path is dir itself)
//  2. a built binary, first after any NAME=value: <binary> <command>, or go tool <binary> <command> (the binary from
//     the module's tool line) → by the layout, the file <cmd>/<binary>/<command>.go (dashes as underscores) or the
//     folder <cmd>/<binary>/<command>/; with no command, the binary itself, <cmd>/<binary>/
//
// Anything else is neither: go tool of another binary, go vet, shell, other programs.
func (g goSide) runsOf(run, dir string) []ran {
	var rans []ran
	run = templateTag.ReplaceAllString(run, " ") // what mise fills in isn't a command's words
	for _, command := range regexp.MustCompile(`&&|\|\||;`).Split(run, -1) {
		words := strings.Fields(command)
		for len(words) > 0 && regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`).MatchString(words[0]) {
			words = words[1:]
		}
		if len(words) == 0 {
			continue
		}
		// go tool <binary> <command> is the binary itself, from the module's tool line: read it as the binary.
		if len(words) > 2 && words[0] == "go" && words[1] == "tool" && g.cmd != "" && slices.Contains(g.binaries, words[2]) {
			words = words[2:]
		}
		switch {
		case words[0] == "go" && len(words) > 1 && slices.Contains([]string{"run", "test", "build"}, words[1]):
			var paths []string
			test := ""
			for i, w := range words[2:] {
				if w == "." || strings.HasPrefix(w, "./") {
					paths = append(paths, w)
				}
				if w == "-run" && words[1] == "test" && i+3 < len(words) {
					if m := regexp.MustCompile(`^["']?\^?([A-Za-z_][A-Za-z0-9_]*)\$?["']?$`).FindStringSubmatch(words[i+3]); m != nil {
						test = m[1]
					}
				}
			}
			if len(paths) == 0 {
				paths = []string{"."}
			}
			for _, w := range paths {
				p := strings.TrimSuffix(strings.TrimSuffix(w, "..."), "/")
				rans = append(rans, ran{place: abs(filepath.Join(dir, p)), binary: g.binaryAt(abs(filepath.Join(dir, p))), tree: strings.HasSuffix(w, "..."), test: test})
			}
		case g.cmd != "" && slices.Contains(g.binaries, words[0]):
			place := filepath.Join(g.cmd, words[0])
			if len(words) > 1 && regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(words[1]) { // a command: open, cf-r2, http2
				if g.layout == "file" {
					place = filepath.Join(place, strings.ReplaceAll(words[1], "-", "_")+".go")
				} else {
					place = filepath.Join(place, words[1])
				}
			}
			rans = append(rans, ran{place: abs(place), binary: words[0]})
		default:
			rans = append(rans, ran{notGo: strings.TrimSpace(command)})
		}
	}
	return rans
}

// binaryAt is the binary of ours whose folder place is, go run ./cmd/proxy's: "" if none.
func (g goSide) binaryAt(place string) string {
	for _, b := range g.binaries {
		if place == abs(filepath.Join(g.cmd, b)) {
			return b
		}
	}
	return ""
}

// CheckSkills holds every agent skill of the repo at dir to the tasks it's for, as checkSkill does one: a skill in
// .claude/skills/<name>/SKILL.md is the skill of the tasks of namespace <name> (the task <name>, and <name>:…), in the
// repo's mise.toml and .mise/tasks/*.toml. Each such skill names every one of its tasks and every flag they take, and
// every task of its namespace it names is one. A skill whose namespace has no tasks is the repo's own business, left
// alone. With GoCmd holding each tool's tasks to its Go, every tool's skill is held to everything its binary can do.
// mise-check check runs it in every repo; a test of a repo's own can too.
func CheckSkills(t testing.TB, dir string) {
	t.Helper()
	files := []string{}
	if _, err := os.Stat(filepath.Join(dir, "mise.toml")); err == nil {
		files = append(files, filepath.Join(dir, "mise.toml"))
	}
	more, _ := filepath.Glob(filepath.Join(dir, ".mise", "tasks", "*.toml"))
	files = append(files, more...)
	skills, _ := filepath.Glob(filepath.Join(dir, ".claude", "skills", "*", "SKILL.md"))
	for _, skill := range skills {
		ns := filepath.Base(filepath.Dir(skill))
		for _, file := range files {
			tasks, _, err := readTasks(file)
			if err != nil {
				continue
			}
			var mine []task
			for _, tk := range tasks {
				if tk.name == ns || strings.HasPrefix(tk.name, ns+":") {
					mine = append(mine, tk)
				}
			}
			if len(mine) > 0 {
				checkSkill(t, file, mine, skill)
			}
		}
	}
}

// ---- The checks ----

// checkSkill holds the agent skill of the tasks to them, both ways, so an agent is never told of a task that isn't
// there, nor left without one that is: every task of the mise file is named in the skill, as a word of its own, and
// every flag a task declares is named in it too; and every task the skill names in full ("namespace:task", in a
// namespace of the file's) is a task. A task added, renamed
// or taken out without the skill, and the skill says so.
func checkSkill(t testing.TB, file string, tasks []task, skill string) {
	t.Helper()
	b, err := os.ReadFile(skill)
	if err != nil {
		t.Errorf("skill: Skills: %s can't be read: %v", skill, err)
		return
	}
	text := string(b)
	names := map[string]bool{}
	namespaces := map[string]bool{}
	for _, task := range tasks {
		names[task.name] = true
		if ns, _, ok := strings.Cut(task.name, ":"); ok {
			namespaces[ns] = true
		}
	}
	missing := 0
	for _, task := range tasks {
		if !regexp.MustCompile(`(^|[^A-Za-z0-9:_-])` + regexp.QuoteMeta(task.name) + `($|[^A-Za-z0-9:_-])`).MatchString(text) {
			missing++
			t.Errorf("skill: %s's task %s isn't named in the skill %s: an agent there never learns of it; say when to run it", file, task.name, skill)
		}
	}
	// Every flag a task declares, so the skill says what each command can do, not only that it's there.
	flags := map[string]string{}
	for _, task := range tasks {
		for _, n := range usageNames(task.usage) {
			if n.flag {
				if _, ok := flags[n.name]; !ok {
					flags[n.name] = task.name
				}
			}
		}
	}
	for _, f := range slices.Sorted(maps.Keys(flags)) {
		if !regexp.MustCompile(`(^|[^A-Za-z0-9_-])--` + regexp.QuoteMeta(f) + `($|[^A-Za-z0-9_-])`).MatchString(text) {
			missing++
			t.Errorf("skill: %s's task %s takes the flag --%s, which the skill %s doesn't name: say what it does", file, flags[f], f, skill)
		}
	}
	unknown := 0
	for _, name := range named(text, namespaces, false) {
		if !names[name] {
			unknown++
			t.Errorf("skill: %s names the task %s, which isn't a task of %s: did it move?", skill, name, file)
		}
	}
	t.Logf("skill: %d tasks and %d flags of %s, against the skill %s: %d not named in it, %d it names that aren't tasks", len(tasks), len(flags), file, skill, missing, unknown)
}

// checkNames holds a file that names tasks to them one way (Names): every task it names is one. A file of another
// tool's namespace named after mise run (a hook copied from another tool, say) is caught too.
func checkNames(t testing.TB, file string, tasks []task, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("names: Names: %s can't be read: %v", path, err)
		return
	}
	names := map[string]bool{}
	namespaces := map[string]bool{}
	for _, task := range tasks {
		names[task.name] = true
		if ns, _, ok := strings.Cut(task.name, ":"); ok {
			namespaces[ns] = true
		}
	}
	found, unknown := named(string(b), namespaces, true), 0
	for _, name := range found {
		if !names[name] {
			unknown++
			t.Errorf("names: %s names the task %s, which isn't a task of %s: name the task there is", path, name, file)
		}
	}
	t.Logf("names: %d tasks named in %s, against %s: %d that aren't tasks", len(found), path, file, unknown)
}

// named are the tasks text names in full, each once, sorted: "namespace:task" in a namespace of namespaces, and, with
// runs, any "mise run namespace:task", whatever its namespace.
func named(text string, namespaces map[string]bool, runs bool) []string {
	found := map[string]bool{}
	for ns := range namespaces {
		for _, m := range regexp.MustCompile(`(?:^|[^A-Za-z0-9:_-])(`+regexp.QuoteMeta(ns)+`:[a-z0-9][a-z0-9-]*)`).FindAllStringSubmatch(text, -1) {
			found[m[1]] = true
		}
	}
	if runs {
		for _, m := range miseRun.FindAllStringSubmatch(text, -1) {
			found[m[1]] = true
		}
	}
	return slices.Sorted(maps.Keys(found))
}

// miseRun is a task named after mise run, in full: mise run cloudflare:03-init.
var miseRun = regexp.MustCompile(`mise run ([a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*)`)

// ---- Used through mise ----

// selfTool is mise-check as a repo's mise pins it, and selfModule its Go module, whose packages a tool repo's tests
// import.
const (
	selfTool   = "github:joeblew999/mise-check"
	selfModule = "github.com/joeblew999/mise-check"
)

// CheckToolRepo holds a mise tool repo, at dir (its top), to the one shape every tool repo has, mise-check's own and
// every one new-tool makes, saying of each thing that isn't what to change: its mise.toml has root:ci, exactly what
// GitHub's CI runs, and root:verify, the checks and the walks; its .github/workflows/ci.yml runs mise run root:ci and
// nothing else of its own (no other task, no go command), once a change: on a push to main and on a pull request,
// never on a tag or every branch's push; its README.md opens with **The problem.** and **What <name> does.**
// (CheckReadme); and no task of its template (templates/.mise/tasks/*.toml) runs mise-check, whose own tasks the
// tool's init gives a repo (toolfiles.MiseCheckInit). A tool repo's test of its root mise file calls it:
//
//	func TestRootMise(t *testing.T) {
//		misecheck.Check(t, misecheck.Config{MiseFile: "mise.toml"})
//		misecheck.CheckToolRepo(t, ".")
//	}
func CheckToolRepo(t testing.TB, dir string) {
	t.Helper()
	tasks, _, err := readTasks(filepath.Join(dir, "mise.toml"))
	if err != nil {
		t.Errorf("a tool repo's mise.toml, at its top: %v", err)
	}
	has := map[string]bool{}
	for _, k := range tasks {
		has[k.name] = true
	}
	for name, what := range map[string]string{"root:ci": "exactly what GitHub's CI runs, the one task ci.yml runs: root:verify, and anything CI adds", "root:verify": "the checks and the walks, what root:ci runs first"} {
		if !has[name] {
			t.Errorf("mise.toml has no %s task: add [tasks.%q], %s", name, name, what)
		}
	}
	ci, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Errorf("no .github/workflows/ci.yml: a tool repo's CI is one workflow that runs mise run root:ci, on a push to main and on a pull request, as mise-check's does")
	}
	for _, p := range ciProblems(strings.ReplaceAll(string(ci), "\r\n", "\n")) {
		t.Error(".github/workflows/ci.yml " + p)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); err != nil {
		t.Errorf("no README.md: a tool repo's opens, after its title, with **The problem.** and **What %s does.**", repoName(dir))
	}
	CheckReadme(t, dir)
	files, _ := filepath.Glob(filepath.Join(dir, "templates", ".mise", "tasks", "*.toml"))
	for _, f := range files {
		if filepath.Base(f) == "mise-check.toml" { // mise-check's own tasks, which init writes
			continue
		}
		ts, _, _ := readTasks(f)
		for _, k := range ts {
			if wrapsMiseCheck.MatchString(k.run) {
				t.Errorf("%s: the task %s runs mise-check (%s): a tool's tasks never wrap mise-check's. Its init runs mise-check's own init (toolfiles.MiseCheckInit), and a repo runs mise run mise-check:<command> itself", displayPath(f), k.name, k.run)
			}
		}
	}
	t.Logf("tool repo: %s has root:ci and root:verify, ci.yml runs only mise run root:ci on main's pushes and pull requests, and %d template task files wrap no mise-check command", dir, len(files))
}

// CheckReadme holds the README.md at dir, when there is one, to how every repo's opens: right after its title (and
// its badges), a paragraph starting **The problem.**, what's hard without the repo, then one starting
// **What <name> does.**, how it solves it, both within the 30 lines after the title. Two literal markers, never the
// prose: each one missing is said with exactly what to add. mise-check check runs it on every repo that uses
// mise-check, and CheckToolRepo on a tool repo, which must have a README.md.
func CheckReadme(t testing.TB, dir string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		return
	}
	for _, p := range readmeProblems(strings.ReplaceAll(string(b), "\r\n", "\n"), repoName(dir)) {
		t.Error(p)
	}
}

// readmeProblems are the markers a README's text s lacks in the 30 lines after its title (the first line that starts
// "# "; the top, with none), each with what to add; name is the repo's, for the second.
func readmeProblems(s, name string) []string {
	lines := strings.Split(s, "\n")
	start := 0
	for i, l := range lines {
		if strings.HasPrefix(l, "# ") {
			start = i + 1
			break
		}
	}
	top := strings.Join(lines[start:min(start+30, len(lines))], "\n")
	var out []string
	if !strings.Contains(top, "**The problem.**") {
		out = append(out, "README.md doesn't open with the problem: right after its title and badges, add a paragraph starting **The problem.**, what's hard without this repo")
	}
	if !regexp.MustCompile(`\*\*What [^*\n]+ does\.\*\*`).MatchString(top) {
		out = append(out, "README.md doesn't say what the repo does: after **The problem.**, add a paragraph starting **What "+name+" does.**, how it solves it")
	}
	return out
}

// repoName is the name of the repo at dir: its folder's.
func repoName(dir string) string {
	if a, err := filepath.Abs(dir); err == nil {
		return filepath.Base(a)
	}
	return filepath.Base(dir)
}

// wrapsMiseCheck matches a run line that runs mise-check, the binary or its tasks.
var wrapsMiseCheck = regexp.MustCompile(`(?:^|[\s;&|(])mise-check(?:\s|$)|mise run mise-check(?:[:\s]|$)`)

// ciProblems are what ci (a workflow's text) does that a tool repo's ci.yml mustn't, each with what to change: run on
// a tag or every branch's push, miss main's push, or run anything of its own but mise run root:ci.
func ciProblems(ci string) []string {
	if ci == "" {
		return nil
	}
	var out []string
	lines := strings.Split(ci, "\n")
	var on []string // the on: block's lines
	for i, l := range lines {
		if m := regexp.MustCompile(`^"?on"?:\s*(.*)$`).FindStringSubmatch(l); m != nil {
			if v := strings.TrimSpace(m[1]); v != "" && !strings.HasPrefix(v, "#") {
				out = append(out, "runs on: "+v+", every branch's push among them: on: push: branches: [main], and pull_request:")
			}
			for _, b := range lines[i+1:] {
				if b != "" && !strings.HasPrefix(b, " ") && !strings.HasPrefix(b, "#") {
					break
				}
				on = append(on, b)
			}
			break
		}
	}
	block := strings.Join(on, "\n")
	switch {
	case regexp.MustCompile(`(?m)^\s+tags(-ignore)?:`).MatchString(block):
		out = append(out, "runs on tags: drop on.push.tags, as a tag's commit ran as main's push already")
	case !regexp.MustCompile(`(?m)^\s+push:`).MatchString(block) && len(on) > 0:
		out = append(out, "doesn't run on a push to main: on: push: branches: [main]")
	case len(on) > 0 && !regexp.MustCompile(`(?m)^\s+branches:\s*\[\s*["']?main["']?\s*\]|^\s+branches:\s*\n\s+-\s*["']?main["']?\s*$`).MatchString(block):
		out = append(out, "runs on every branch's push, or on branches but main: on.push.branches: [main] (a branch's pull request runs it)")
	}
	ran := false
	for _, l := range lines {
		if s := strings.TrimSpace(l); strings.HasPrefix(s, "#") || strings.HasPrefix(s, "name:") {
			continue
		}
		for _, m := range regexp.MustCompile(`mise run ([^\s"'#;&|()]+)`).FindAllStringSubmatch(l, -1) {
			if m[1] != "root:ci" {
				out = append(out, "runs mise run "+m[1]+": it runs only mise run root:ci; put "+m[1]+" in root:verify (or root:ci)")
			}
			ran = true
		}
		if m := regexp.MustCompile(`(?:^|[\s;&|])(go (?:test|vet|build|run|generate)|goreleaser)\b`).FindStringSubmatch(l); m != nil {
			out = append(out, "runs "+m[1]+" itself: put it in a task root:verify runs, and let ci.yml run only mise run root:ci")
		}
	}
	if !ran {
		out = append(out, "doesn't run mise run root:ci: its last step is run: mise run root:ci")
	}
	return out
}

// checkThroughMise holds a repo whose tests import mise-check's packages to using mise-check itself, through mise:
// the repo has the tasks mise-check init writes (.mise/tasks/mise-check.toml), and .mise/config.toml pins the binary
// at the release go.mod requires the packages at. The packages are only the strict checks for a tool repo's own tests;
// mise-check is its tasks, so a repo that only requires the packages is told, by its own tests, how to use it. It
// looks from dir (the test's folder) up to the module's go.mod, and for the tasks from there up to the repo's top. Left
// alone: mise-check's own checkout, which runs it from source; a go.mod that doesn't require it; and anything but a
// test (mise-check check, the binary, is run through its tasks already).
func checkThroughMise(t testing.TB, dir string) {
	t.Helper()
	if !testing.Testing() {
		return
	}
	mod := upTo(dir, "go.mod", false)
	if mod == "" {
		return
	}
	b, err := os.ReadFile(mod)
	if err != nil {
		return
	}
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	if m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindStringSubmatch(text); m != nil && m[1] == selfModule {
		t.Logf("through mise: this is mise-check's own checkout, which runs it from source")
		return
	}
	replaced := regexp.MustCompile(`(?m)^\s*(?:replace\s+)?` + regexp.QuoteMeta(selfModule) + `(?:\s+v\S+)?\s+=>`).MatchString(text)
	m := regexp.MustCompile(`(?m)^\s*(?:require\s+)?` + regexp.QuoteMeta(selfModule) + `\s+(v\S+)`).FindStringSubmatch(text)
	if m == nil {
		return
	}
	at := strings.TrimPrefix(m[1], "v")
	release := at
	if replaced { // a checkout of mise-check, as its packages: its binary is the same checkout's build, local
		release = "latest"
	}
	tasks := upTo(filepath.Dir(mod), filepath.Join(".mise", "tasks", "mise-check.toml"), true)
	if tasks == "" {
		t.Errorf("mise-check isn't used through mise in this repo: go.mod requires its packages (%s), which are only the strict checks for a tool's own tests, and the repo has no mise-check tasks (.mise/tasks/mise-check.toml). At the repo's top, run\n"+
			"    mise exec %s@%s -- mise-check init\n"+
			"It writes mise-check's tasks, pins that release in .mise/config.toml (the same as go.mod's), and adds its GitHub workflow, its agent skill and docs/mise-check/README.md. Then mise run mise-check says what's done and what to run next, and mise run mise-check:update moves every pin, these two at one release. https://github.com/joeblew999/mise-check#use-it-in-your-repo",
			m[1], selfTool, release)
		return
	}
	top := filepath.Dir(filepath.Dir(filepath.Dir(tasks)))
	cfg, _ := os.ReadFile(filepath.Join(top, ".mise", "config.toml"))
	p := regexp.MustCompile(regexp.QuoteMeta(`"`+selfTool+`" = "`) + `([^"]*)"`).FindSubmatch(cfg)
	switch {
	case p == nil:
		t.Errorf("mise-check's tasks are in %s, and its .mise/config.toml doesn't pin it (%s): mise run mise-check:init adds the pin, at its release", top, selfTool)
	case replaced || string(p[1]) == "local" || string(p[1]) == "latest" || string(p[1]) == at:
		t.Logf("through mise: mise-check's tasks in %s, its binary pinned at %s, its packages at %s", top, p[1], m[1])
	case olderRelease(string(p[1]), at):
		// The binary is the older, maybe too old to move itself well, so the way out is the one line that needs
		// nothing of it: init at go.mod's release, an exact one, which mise gets at once, and which moves the pin there.
		t.Errorf("mise-check's two pins are apart: .mise/config.toml pins the binary at %s, and go.mod requires the packages at %s (a go get of the packages alone leaves the binary behind). For the binary at the packages' release, at the repo's top, run\n"+
			"    mise exec %s@%s -- mise-check init", p[1], m[1], selfTool, at)
	default:
		t.Errorf("mise-check's two pins are apart: .mise/config.toml pins the binary at %s, and go.mod requires the packages at %s. mise run mise-check:update moves both to one release", p[1], m[1])
	}
}

// olderRelease reports whether release a (0.4.29) is before release b (0.4.30); false for anything but x.y.z.
func olderRelease(a, b string) bool {
	var x, y [3]int
	if n, _ := fmt.Sscanf(a, "%d.%d.%d", &x[0], &x[1], &x[2]); n != 3 {
		return false
	}
	if n, _ := fmt.Sscanf(b, "%d.%d.%d", &y[0], &y[1], &y[2]); n != 3 {
		return false
	}
	return slices.Compare(x[:], y[:]) < 0
}

// upTo is the file name, a path from a folder, found in dir or a folder above it, or "": with repo, no higher than the
// repo's top (the folder with .git).
func upTo(dir, name string, repo bool) string {
	d, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(d, name)); err == nil {
			return filepath.Join(d, name)
		}
		if _, err := os.Stat(filepath.Join(d, ".git")); repo && err == nil {
			return ""
		}
		up := filepath.Dir(d)
		if up == d {
			return ""
		}
		d = up
	}
}

// checkMiseToGo checks mise to Go: every place of Go a task runs is there, and is Go. A task with nothing in either
// form is reported, as not checked. A mise file whose tasks run no Go at all fails if it's given GoCmd, which says
// it holds a binary's tasks; without it, it's a file of tool tasks, held to the usage rules alone.
func checkMiseToGo(t testing.TB, file string, tasks []task, g goSide) {
	t.Helper()
	places, missing, notGo := 0, 0, 0
	for _, task := range tasks {
		for _, r := range g.runsOf(task.run, taskDir(file)) {
			if r.notGo != "" {
				notGo++
				t.Logf("not checked: %s's task %s runs %q, which is neither form: go run|test|build ./path, or a binary of GoCmd (itself, or go tool <binary>)", file, task.name, r.notGo)
				continue
			}
			places++
			if !isGo(r.place) && !(r.tree && hasGoBelow(r.place)) {
				missing++
				t.Errorf("mise → Go: %s's task %s runs %q, so its Go is %s, and there's none there", file, task.name, task.run, displayPath(r.place))
			} else if r.test != "" && !hasTest(r.place, r.tree, r.test) {
				missing++
				t.Errorf("mise → Go: %s's task %s runs the test %s, and there's none in %s: was it renamed?", file, task.name, r.test, displayPath(r.place))
			}
		}
	}
	if len(tasks) > 0 && places == 0 {
		if g.cmd != "" {
			t.Errorf("mise → Go: no task of %s runs Go, in either form: did the Go move, or the tasks?", file)
		} else {
			t.Logf("mise → Go: no task of %s runs Go: a file of tool tasks, held to the usage rules alone", file)
		}
	}
	t.Logf("mise → Go: %d tasks of %s, running %d places of Go: %d with no Go there; %d commands in neither form, not checked", len(tasks), file, places, missing, notGo)
}

// checkGoToMise checks Go to mise: every entry point of a binary under GoCmd (commandsIn) is one a task runs. A
// binary itself, its main.go, is in use once any task runs it, in either form.
func checkGoToMise(t testing.TB, file string, tasks []task, g goSide) {
	t.Helper()
	run := map[string]bool{}
	for _, task := range tasks {
		for _, r := range g.runsOf(task.run, taskDir(file)) {
			if r.binary != "" {
				run[r.place] = true
				run[abs(filepath.Join(g.cmd, r.binary))] = true
			}
		}
	}
	commands, unrun := 0, 0
	for _, b := range g.binaries {
		for _, place := range commandsIn(filepath.Join(g.cmd, b), g.layout) {
			commands++
			if !run[abs(place)] {
				unrun++
				t.Errorf("Go → mise: %s is a command of %s (GoLayout %s), and no task of %s runs it", displayPath(place), b, g.layout, file)
			}
		}
	}
	t.Logf("Go → mise: %d entry points under %s (GoLayout %s), against the tasks of %s: %d no task runs", commands, g.cmd, g.layout, file, unrun)
}

// checkUsage checks the golden rule: a task declares the flags and arguments it takes in mise's usage spec, and its
// run line passes each by name with mise's templates (usage.<name>), and nothing undeclared; then mise refuses anything
// else before the Go runs. Within each task: every name its usage declares is in its run, every usage.<name> its run
// uses is declared, nothing is read mise's old way, {{flag(…)}}, and no run line, a task's with usage or not, has POSIX
// shell syntax, which breaks under Windows' cmd. And flags both ways, at each binary of GoCmd: every flag a task
// running it declares is one its Go defines (flag.Bool("yes", …)), and every flag its Go defines is declared by such a
// task. A task that takes nothing has no usage, and the Go refuses what it doesn't know.
func checkUsage(t testing.TB, file string, tasks []task, g goSide) {
	t.Helper()
	declared := map[string]map[string]string{} // binary → flag → the task that declares it
	withUsage, flags := 0, 0
	for _, task := range tasks {
		for _, m := range deprecated.FindAllStringSubmatch(task.run, -1) {
			t.Errorf("usage: %s's task %s's run uses {{%s(…)}}, which mise has deprecated and removes in mise 2027.5.0: declare it in usage instead", file, task.name, m[1])
		}
		if m := posix.FindString(task.run); m != "" {
			t.Errorf("usage: %s's task %s's run has POSIX shell syntax, %q, which Windows' cmd doesn't read: pass usage values with mise's templates ({{usage.<name>}}), and set variables in the task's env: %s", file, task.name, m, task.run)
		}
		if task.usage == "" {
			continue
		}
		withUsage++
		names := usageNames(task.usage)
		for _, n := range names {
			if !regexp.MustCompile(`\busage\.` + n.env() + `\b`).MatchString(task.run) {
				t.Errorf("usage: %s's task %s declares %s, and its run doesn't pass it (usage.%s): %s", file, task.name, n, n.env(), task.run)
			}
		}
		for _, m := range regexp.MustCompile(`\busage\.([a-z0-9_]+)`).FindAllStringSubmatch(task.run, -1) {
			if !slices.ContainsFunc(names, func(n usageName) bool { return n.env() == m[1] }) {
				t.Errorf("usage: %s's task %s's run passes usage.%s, which its usage doesn't declare", file, task.name, m[1])
			}
		}
		for _, r := range g.runsOf(task.run, taskDir(file)) {
			if r.binary == "" {
				continue
			}
			for _, n := range names {
				if n.flag {
					flags++
					if declared[r.binary] == nil {
						declared[r.binary] = map[string]string{}
					}
					declared[r.binary][n.name] = task.name
				}
			}
		}
	}
	goFlags := 0
	for _, b := range g.binaries {
		defined := goFlagsIn(filepath.Join(g.cmd, b))
		goFlags += len(defined)
		for f, task := range declared[b] {
			if !slices.Contains(defined, f) {
				t.Errorf("usage, mise → Go: %s's task %s declares the flag --%s, and %s's Go defines no flag %q", file, task, f, b, f)
			}
		}
		for _, f := range defined {
			if _, ok := declared[b][f]; !ok {
				t.Errorf("usage, Go → mise: %s's Go defines the flag --%s, and no task of %s running it declares it in its usage", b, f, file)
			}
		}
	}
	t.Logf("usage: %d tasks of %s with a usage, declaring %d flags of binaries; %d flags in the Go of GoCmd", withUsage, file, flags, goFlags)
}

// checkGoCallsMise checks the other mapping, Go that runs mise tasks: Go names a task in full, as a string
// ("<namespace>:<task>"), and every name it gives is a task of the mise file. What is a name comes from the tasks
// themselves: their namespaces, the part before the ":". A name glued together from its two parts can't be checked,
// so it fails on one. It reads the .go files of the folder callers, and fails if none of them names a task.
func checkGoCallsMise(t testing.TB, file string, tasks []task, callers string) {
	t.Helper()
	names, namespaces := map[string]bool{}, map[string]bool{}
	for _, task := range tasks {
		names[task.name] = true
		if ns, _, ok := strings.Cut(task.name, ":"); ok {
			namespaces[ns] = true
		}
	}
	files, _ := filepath.Glob(filepath.Join(callers, "*.go"))
	if len(files) == 0 {
		t.Errorf("GoCallers %s has no .go files: did the Go that runs %s move?", callers, file)
		return
	}
	found, inFiles, wrong := 0, map[string]bool{}, 0
	for ns := range namespaces {
		name := regexp.MustCompile(`\b` + regexp.QuoteMeta(ns) + `:[0-9a-z][a-z0-9-]*`)
		glued := regexp.MustCompile(`"` + regexp.QuoteMeta(ns) + `:"\s*\+`)
		for _, f := range files {
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range glued.FindAllString(string(b), -1) {
				t.Errorf("Go → mise: %s builds a task name from pieces (%s …), so it can't be checked: write the name in full", f, g)
			}
			for _, n := range name.FindAllString(string(b), -1) {
				found++
				inFiles[f] = true
				if !names[n] {
					wrong++
					t.Errorf("Go → mise: %s names the task %s, which isn't a task of %s", f, n, file)
				}
			}
		}
	}
	if found == 0 {
		t.Errorf("Go → mise: no Go in %s names a task of %s, in full (\"<namespace>:<task>\"): did the Go move?", callers, file)
	}
	t.Logf("Go → mise: %d task names, in %d of the %d Go files in %s, against the tasks of %s: %d not tasks", found, len(inFiles), len(files), callers, file, wrong)
}

// ---- Reading the mise file ----

// task is one mise task: its name, its run line, and its usage, the flags and arguments it declares.
type task struct{ name, run, usage string }

// A task's table is [tasks."name"] or [tasks.name] in a mise.toml, which has other tables too ([tools], [settings],
// [env]), and ["name"] or [name] in a file of tasks only (.mise/tasks/*.toml, or one a mise.toml includes). A file with
// any [tasks.…] table is a mise.toml.
var (
	configTaskHeader = regexp.MustCompile(`^\[tasks\.(?:"([^"]+)"|([A-Za-z0-9_-]+))\]$`)
	tasksFileHeader  = regexp.MustCompile(`^\[(?:"([^"]+)"|([A-Za-z0-9_-]+))\]$`)
	runLine          = regexp.MustCompile(`^run\s*=\s*(.*?)\s*$`)
	usageLine        = regexp.MustCompile(`^usage\s*=\s*(.*?)\s*$`)
)

// readTasks reads a mise file's tasks, each with its run line and its usage, each written on one line, "…" or '…'.
// One written any other way (a list, or a string over several lines) it can't read: it says which, in unreadable,
// rather than skip it.
func readTasks(file string) (tasks []task, unreadable []string, err error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}
	header := tasksFileHeader
	if regexp.MustCompile(`(?m)^\[tasks\.`).Match(b) {
		header = configTaskHeader
	}
	in := false // in a task's table, not another's
	// Line by line, \r\n as well as \n: git on Windows checks a repo out with \r\n unless its .gitattributes says not to.
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "[") {
			m := header.FindStringSubmatch(line)
			if in = m != nil; in {
				tasks = append(tasks, task{name: m[1] + m[2]})
			}
			continue
		}
		if !in {
			continue
		}
		last := &tasks[len(tasks)-1]
		for _, field := range []struct {
			line *regexp.Regexp
			to   *string
		}{{runLine, &last.run}, {usageLine, &last.usage}} {
			if m := field.line.FindStringSubmatch(line); m != nil {
				if v, ok := oneLine(m[1]); ok {
					*field.to = v
				} else {
					what := strings.Fields(line)[0]
					unreadable = append(unreadable, fmt.Sprintf("%s: the task %s's %s isn't one line (%s = %s): the checker reads one written on one line, \"…\" or '…'", file, last.name, what, what, m[1]))
				}
			}
		}
	}
	return tasks, unreadable, nil
}

// oneLine is the text of a TOML string written on one line, "…" or '…', and whether it is one.
func oneLine(v string) (string, bool) {
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' && !strings.HasPrefix(v, `"""`) ||
		v[0] == '\'' && v[len(v)-1] == '\'' && !strings.HasPrefix(v, `'''`)) {
		return v[1 : len(v)-1], true
	}
	return "", false
}

// taskDir is the folder a mise file's tasks run in, as mise runs them: a mise.toml's own folder, or for a file in
// .mise/tasks/ the project's, two folders up. A task's ./path is from there.
func taskDir(file string) string {
	dir := filepath.Dir(file)
	if filepath.Base(dir) == "tasks" && filepath.Base(filepath.Dir(dir)) == ".mise" {
		return filepath.Dir(filepath.Dir(dir))
	}
	return dir
}

// deprecated is mise's old way of reading flags and arguments in a run line, {{arg(…)}}, {{option(…)}} and
// {{flag(…)}}, which mise has deprecated for usage, and removes in mise 2027.5.0.
// posix is shell syntax that only a POSIX shell reads: ${…}, $name, and NAME=value in front of a command.
var posix = regexp.MustCompile(`\$\{[^}]*\}|\$[A-Za-z_][A-Za-z0-9_]*|(?:^|&&|\|\||;)\s*[A-Za-z_][A-Za-z0-9_]*=\S*\s`)

// templateTag is a mise template tag in a run line, {% … %} or {{ … }}, which mise fills in before the shell runs.
var templateTag = regexp.MustCompile(`\{%.*?%\}|\{\{.*?\}\}`)

var deprecated = regexp.MustCompile(`\{\{-?\s*(arg|option|flag)\(`)

// usageName is a flag or an argument a usage spec declares, by its long name.
type usageName struct {
	name string
	flag bool
}

func (n usageName) String() string {
	if n.flag {
		return "the flag --" + n.name
	}
	return "the argument " + n.name
}

// env is the name as mise's templates have it, usage.<env> (and its environment, usage_<env>): dashes as underscores.
func (n usageName) env() string { return strings.ReplaceAll(n.name, "-", "_") }

// usageNames are the flags (flag "-y --yes", flag "--branch <branch>") and arguments (arg "<name>", arg "[args]...") a
// usage spec declares.
func usageNames(usage string) []usageName {
	var names []usageName
	for _, m := range regexp.MustCompile(`\bflag "(?:-[A-Za-z] )?--([a-z][a-z0-9-]*)`).FindAllStringSubmatch(usage, -1) {
		names = append(names, usageName{m[1], true})
	}
	for _, m := range regexp.MustCompile(`\barg "[<\[]([a-z][a-z0-9_-]*)[>\]]`).FindAllStringSubmatch(usage, -1) {
		names = append(names, usageName{m[1], false})
	}
	return names
}

// ---- Reading the Go ----

// binariesIn are the binaries the Go builds: the folders under cmd.
func binariesIn(cmd string) []string {
	entries, _ := os.ReadDir(cmd)
	var bins []string
	for _, e := range entries {
		if e.IsDir() {
			bins = append(bins, e.Name())
		}
	}
	return bins
}

// commandsIn are a binary's commands, its entry points, by the layout: each folder in its folder (folder), or each .go
// file of its folder that holds the function named for it, as set_secrets.go holds setSecrets (file). Go reserves two
// names, so two files go by their names alone: main.go, the binary itself, and init.go, the command init, whose
// function can't be named init. Code the commands call further down, like a file of what several share, isn't one:
// the checker leaves it alone.
func commandsIn(dir, layout string) []string {
	entries, _ := os.ReadDir(dir)
	var commands []string
	for _, e := range entries {
		name := e.Name()
		switch {
		case layout == "folder" && e.IsDir():
			commands = append(commands, filepath.Join(dir, name))
		case layout == "file" && name == "main.go":
			commands = append(commands, dir) // the binary itself, in use once any task runs it
		case layout == "file" && name == "init.go":
			commands = append(commands, filepath.Join(dir, name)) // Go reserves init: the command's function can't be named for it
		case layout == "file" && !e.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go"):
			if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil && entryFunc(strings.TrimSuffix(name, ".go")).Match(b) {
				commands = append(commands, filepath.Join(dir, name))
			}
		}
	}
	return commands
}

// entryFunc finds, in a command's file, the function named for it: set_secrets is setSecrets, a function or a method.
func entryFunc(stem string) *regexp.Regexp {
	words := strings.Split(stem, "_")
	name := words[0]
	for _, w := range words[1:] {
		name += strings.ToUpper(w[:1]) + w[1:]
	}
	return regexp.MustCompile(`(?m)^func (\([^)]*\) )?` + regexp.QuoteMeta(name) + `\(`)
}

// goFlagsIn are the flags the Go in dir defines with the flag package: flag.Bool("yes", …), flags.String("branch", …),
// flag.BoolVar(&yes, "yes", …).
func goFlagsIn(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var defined []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		for _, m := range regexp.MustCompile(`\.(?:Bool|String|Int|Int64|Uint|Uint64|Float64|Duration)(?:Var\([^,]+,\s*|\()"([a-z][a-z0-9-]*)"`).FindAllStringSubmatch(string(b), -1) {
			if !slices.Contains(defined, m[1]) {
				defined = append(defined, m[1])
			}
		}
	}
	return defined
}

// isGo reports whether place is a .go file, or a folder with a .go file of its own.
func isGo(place string) bool {
	if strings.HasSuffix(place, ".go") {
		st, err := os.Stat(place)
		return err == nil && !st.IsDir()
	}
	entries, err := os.ReadDir(place)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(entries, func(e os.DirEntry) bool { return !e.IsDir() && strings.HasSuffix(e.Name(), ".go") })
}

// hasTest reports whether the folder (and below it, for ./...) has the test func name(t *testing.T) in a _test.go.
func hasTest(place string, tree bool, name string) bool {
	re := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(\s*\w+ \*testing\.T\s*\)`)
	found := false
	filepath.WalkDir(place, func(p string, d os.DirEntry, err error) error {
		switch {
		case err != nil || found:
			return filepath.SkipAll
		case d.IsDir() && p != place && (!tree || d.Name() == "vendor" || d.Name() == "testdata" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")):
			return filepath.SkipDir
		case !d.IsDir() && strings.HasSuffix(d.Name(), "_test.go"):
			if b, err := os.ReadFile(p); err == nil && re.Match(b) {
				found = true
			}
		}
		return nil
	})
	return found
}

// hasGoBelow reports whether a folder holds Go anywhere below it, as go test ./... finds it: none in vendor, testdata or a
// folder whose name starts with a dot or an underscore, nor in node_modules.
func hasGoBelow(place string) bool {
	found := false
	filepath.WalkDir(place, func(p string, d os.DirEntry, err error) error {
		switch {
		case err != nil || found:
			return filepath.SkipAll
		case d.IsDir() && p != place && (d.Name() == "vendor" || d.Name() == "testdata" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")):
			return filepath.SkipDir
		case !d.IsDir() && strings.HasSuffix(d.Name(), ".go"):
			found = true
		}
		return nil
	})
	return found
}

// abs is place by its full path, so places compare whatever folder they were named from.
func abs(place string) string {
	if a, err := filepath.Abs(place); err == nil {
		return a
	}
	return place
}

// displayPath is place as the checker says it: from here when it's under here, else in full.
func displayPath(place string) string {
	wd, _ := os.Getwd()
	if r, err := filepath.Rel(wd, place); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return place
}
