package misecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestMise checks this repo's own mise file, by itself, as any repo does: mise run root:check-mise sets MISE_FILE.
func TestMise(t *testing.T) { Check(t, FromEnv()) }

// ---- The checker's own tests: what it reads, and how ----

// TestReadTasks: every way mise writes a task is read, or said to be unreadable, and no other table is taken for one.
func TestReadTasks(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		name, text string
		want       []task
		unreadable int
	}{
		{"mise.toml", `[tools]
go = "1.27"

[settings]
skills.fetch = false

[env]
A = "b"

[tasks."a:b"]
run = "mise-cloudflare b"

[tasks.setup]
usage = 'flag "--yes" help="do not ask"'
run = 'aube install{% if usage.yes %} --yes{% endif %}'

[tasks.list]
run = ["a", "b"]

[tasks.lines]
run = """
a
"""
`, []task{{"a:b", "mise-cloudflare b", ""}, {"setup", "aube install{% if usage.yes %} --yes{% endif %}", `flag "--yes" help="do not ask"`}, {"list", "", ""}, {"lines", "", ""}}, 2},
		{"tasks.toml", `["a:b"]
run = "x"

[setup]
usage = '''
flag "--yes"
'''
run = 'y'
`, []task{{"a:b", "x", ""}, {"setup", "y", ""}}, 1},
		{"crlf.toml", "[\"a:b\"]\r\nrun = \"x\"\r\n\r\n[setup]\r\nrun = 'y'\r\n", []task{{"a:b", "x", ""}, {"setup", "y", ""}}, 0},
	} {
		file := filepath.Join(dir, c.name)
		os.WriteFile(file, []byte(c.text), 0o644)
		got, unreadable, err := readTasks(file)
		if err != nil || !slices.Equal(got, c.want) || len(unreadable) != c.unreadable {
			t.Errorf("%s: %+v, %d unreadable (%q), %v; want %+v, %d unreadable", c.name, got, len(unreadable), unreadable, err, c.want, c.unreadable)
		}
	}
}

// TestRunForms: a run line's commands are read in the two forms, and anything else is neither.
func TestRunForms(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "cmd", "tool"), 0o755)
	g := goSide{cmd: filepath.Join(dir, "cmd"), layout: "file", binaries: []string{"tool"}}
	in := func(p ...string) string { return abs(filepath.Join(append([]string{dir}, p...)...)) }
	for run, want := range map[string][]ran{
		"tool set-secrets": {{place: in("cmd", "tool", "set_secrets.go"), binary: "tool"}},
		"tool cf-r2":       {{place: in("cmd", "tool", "cf_r2.go"), binary: "tool"}},
		"tool{% if usage.json %} --json{% endif %}":                {{place: in("cmd", "tool"), binary: "tool"}},
		`tool deploy{% for a in usage.args %} "{{a}}"{% endfor %}`: {{place: in("cmd", "tool", "deploy.go"), binary: "tool"}},
		"tool":                            {{place: in("cmd", "tool"), binary: "tool"}},
		"CI=1 tool check --json":          {{place: in("cmd", "tool", "check.go"), binary: "tool"}},
		"go run ./cmd/tool -x":            {{place: in("cmd", "tool"), binary: "tool"}},
		"go test ./e2e -run '^TestX$' -v": {{place: in("e2e")}},
		"go test ./...":                   {{place: in()}},
		"go vet ./... && go test ./...":   {{notGo: "go vet ./..."}, {place: in()}},
		"go tool tool dev":                {{place: in("cmd", "tool", "dev.go"), binary: "tool"}},
		"go tool tool{% if usage.json %} --json{% endif %}": {{place: in("cmd", "tool"), binary: "tool"}},
		"go tool gsx generate":                              {{notGo: "go tool gsx generate"}},
		"hk check --all":                                    {{notGo: "hk check --all"}},
		"echo tool":                                         {{notGo: "echo tool"}},
	} {
		if got := g.runsOf(run, dir); !slices.Equal(got, want) {
			t.Errorf("%q: %+v, want %+v", run, got, want)
		}
	}
}

// ---- The example, checked: it passes, and each way of breaking it is caught ----

// ours is mise-check's own template checked the strict way, in a copy at dir (or this checkout, "."): its tasks file
// against the binary's Go (cmd/mise-check) and against the walk that runs them (e2e/).
func ours(dir string) Config {
	return Config{
		MiseFile:  filepath.Join(dir, tasksFile),
		GoCmd:     filepath.Join(dir, "cmd"),
		GoLayout:  "file",
		GoCallers: filepath.Join(dir, "e2e"),
	}
}

// TestTemplate: mise-check's own template passes, the strict way, and so does this repo's own mise file (its
// lib:check-mise).
func TestTemplate(t *testing.T) {
	if errs := checked(ours(".")); len(errs) > 0 {
		t.Errorf("templates/'s tasks fail misecheck:\n%s", strings.Join(errs, "\n"))
	}
	if errs := checked(Config{MiseFile: "mise.toml"}); len(errs) > 0 {
		t.Errorf("mise.toml fails misecheck:\n%s", strings.Join(errs, "\n"))
	}
}

// TestBroken: a copy of mise-check's own template, its binary and its walk, broken one way at a time, each the way a
// mise file and its Go drift apart; misecheck says so, each time.
func TestBroken(t *testing.T) {
	for _, c := range []struct {
		name    string
		breakIt func(dir string)
		want    string
	}{
		{"a task renamed, and Go still runs it by its old name",
			edit(tasksFile, `:05-show"]`, `:05-view"]`),
			"05-show, which isn't a task"},
		{"a command's Go deleted, and its task still runs it",
			func(dir string) { os.Remove(filepath.Join(dir, "cmd", "mise-check", "show_version.go")) },
			"and there's none there"},
		{"a flag added in Go, not in its task's usage",
			edit(filepath.Join("cmd", "mise-check", "main.go"), `	pick := flags.Bool(`, `	flags.Bool("loud", false, "")
	pick := flags.Bool(`),
			"defines the flag --loud"},
		{"a flag added to a task's usage, not in its Go",
			edit(tasksFile, `flag "--pick" help="ask for each one, with each tool's own picker";`, `flag "--pick" help="ask for each one, with each tool's own picker"; flag "--color" help="c";`),
			"declares the flag --color"},
		{"a flag declared, and its run doesn't pass it",
			edit(tasksFile, `{% if usage.pick %} --pick{% endif %}`, ``),
			"declares the flag --pick, and its run doesn't pass it"},
		{"POSIX shell syntax, which Windows' cmd doesn't read",
			edit(tasksFile, `run = "mise-check show-version"`, `run = "GREETING=1 mise-check show-version"`),
			"has POSIX shell syntax"},
		{"a run over several lines: logic that belongs in Go",
			edit(tasksFile, `run = "mise-check show-version"`, "run = \"\"\"\nmise-check show-version\n\"\"\""),
			"isn't one line"},
		{"a command added to the binary, and no task runs it",
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "cmd", "mise-check", "wave.go"), []byte("package main\n\nfunc wave(args []string) {}\n"), 0o644)
			},
			"is a command of mise-check"},
		{"a task name glued together from pieces in Go",
			edit(filepath.Join("e2e", "walk_test.go"), `"github.com/joeblew999/mise-check/walk"`, "\"github.com/joeblew999/mise-check/walk\"\n)\n\nvar glued = \"mise-check:\" + \"05-show\"\n\nvar (_ = glued"),
			"builds a task name from pieces"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			copyOurs(t, dir)
			if errs := checked(ours(dir)); len(errs) > 0 {
				t.Fatalf("the copy fails before it's broken:\n%s", strings.Join(errs, "\n"))
			}
			c.breakIt(dir)
			errs := checked(ours(dir))
			if !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, c.want) }) {
				t.Errorf("not caught: want an error with %q, got:\n%s", c.want, strings.Join(errs, "\n"))
			}
		})
	}
}

// edit is a break: the one place old is in file (from the copy's root), made new.
func edit(file, old, new string) func(dir string) {
	return func(dir string) {
		p := filepath.Join(dir, file)
		b, _ := os.ReadFile(p)
		if strings.Count(string(b), old) != 1 {
			panic(fmt.Sprintf("%s: %q isn't there exactly once", file, old))
		}
		os.WriteFile(p, []byte(strings.Replace(string(b), old, new, 1)), 0o644)
	}
}

// tasksFile is the template's tasks file, from the repo's root: what TestBroken breaks.
var tasksFile = filepath.Join("templates", ".mise", "tasks", "mise-check.toml")

// copyOurs copies what ours checks, the binary's Go, the template and the walk, into dir.
func copyOurs(t *testing.T, dir string) {
	t.Helper()
	for _, f := range []string{"cmd", "templates", "e2e"} {
		copyTree(t, f, filepath.Join(dir, f))
	}
}

// copyTree copies a folder of this checkout into to.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checked runs Check against c and returns what it reports as errors, rather than failing the test that calls it.
func checked(c Config) []string {
	r := &recorder{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		Check(r, c)
	}()
	<-done
	return r.errors
}

// recorder is a testing.TB that keeps what Check reports. Fatal and Skip end Check's goroutine, as they end a test.
type recorder struct {
	testing.TB
	errors []string
}

func (r *recorder) Helper()             {}
func (r *recorder) Log(...any)          {}
func (r *recorder) Logf(string, ...any) {}
func (r *recorder) Error(args ...any)   { r.errors = append(r.errors, fmt.Sprint(args...)) }
func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
func (r *recorder) Fatal(args ...any)                 { r.Error(args...); runtime.Goexit() }
func (r *recorder) Fatalf(format string, args ...any) { r.Errorf(format, args...); runtime.Goexit() }
func (r *recorder) Skip(...any)                       { runtime.Goexit() }

// TestBinaries: GO_BINARIES holds only the binaries it names to the strict way, so a repo can take it up one binary at
// a time; without it every binary of GO_CMD is held to it, and a name that isn't a binary is an error.
// TestToolTasks: a file of tasks that only run tools passes the usage rules without GO_CMD, and still fails them (POSIX
// shell syntax here); given GO_CMD, a file whose tasks run no Go fails, as its binary's Go has moved.
func TestToolTasks(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "tools.toml")
	write := func(run string) {
		os.WriteFile(file, []byte("[\"site:dev\"]\nrun = '"+run+"'\n"), 0o644)
	}
	write("hugo server --bind 0.0.0.0")
	if errs := checked(Config{MiseFile: file}); len(errs) > 0 {
		t.Errorf("tool tasks without GO_CMD should pass; got:\n%s", strings.Join(errs, "\n"))
	}
	write("hugo server --port $PORT")
	if errs := checked(Config{MiseFile: file}); !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, "POSIX shell syntax") }) {
		t.Errorf("a tool task with POSIX shell syntax should fail; got:\n%s", strings.Join(errs, "\n"))
	}
	write("hugo server --bind 0.0.0.0")
	copyOurs(t, dir)
	withCmd := Config{MiseFile: file, GoCmd: filepath.Join(dir, "cmd"), GoLayout: "file"}
	if errs := checked(withCmd); !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, "runs Go, in either form") }) {
		t.Errorf("given GO_CMD, tasks that run no Go should fail; got:\n%s", strings.Join(errs, "\n"))
	}
}

func TestBinaries(t *testing.T) {
	dir := t.TempDir()
	copyOurs(t, dir)
	os.MkdirAll(filepath.Join(dir, "cmd", "other"), 0o755)
	os.WriteFile(filepath.Join(dir, "cmd", "other", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)

	all := ours(dir)
	if errs := checked(all); !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, "is a command of other") }) {
		t.Errorf("without GO_BINARIES, a binary no task runs should fail; got:\n%s", strings.Join(errs, "\n"))
	}
	mc := ours(dir)
	mc.GoBinaries = "mise-check"
	if errs := checked(mc); len(errs) > 0 {
		t.Errorf("GO_BINARIES=mise-check should leave other alone; got:\n%s", strings.Join(errs, "\n"))
	}
	wrong := ours(dir)
	wrong.GoBinaries = "mise-check,nope"
	if errs := checked(wrong); !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, "GO_BINARIES names nope") }) {
		t.Errorf("GO_BINARIES naming no binary should fail; got:\n%s", strings.Join(errs, "\n"))
	}
}
