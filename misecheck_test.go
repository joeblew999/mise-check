package misecheck

import (
	"os"
	"path/filepath"
	"slices"
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
		"tool set-secrets":                                         {{place: in("cmd", "tool", "set_secrets.go"), binary: "tool"}},
		"tool{% if usage.json %} --json{% endif %}":                {{place: in("cmd", "tool"), binary: "tool"}},
		`tool deploy{% for a in usage.args %} "{{a}}"{% endfor %}`: {{place: in("cmd", "tool", "deploy.go"), binary: "tool"}},
		"tool":                            {{place: in("cmd", "tool"), binary: "tool"}},
		"CI=1 tool check --json":          {{place: in("cmd", "tool", "check.go"), binary: "tool"}},
		"go run ./cmd/tool -x":            {{place: in("cmd", "tool"), binary: "tool"}},
		"go test ./e2e -run '^TestX$' -v": {{place: in("e2e")}},
		"go test ./...":                   {{place: in()}},
		"go vet ./... && go test ./...":   {{notGo: "go vet ./..."}, {place: in()}},
		"go tool tool dev":                {{notGo: "go tool tool dev"}},
		"hk check --all":                  {{notGo: "hk check --all"}},
		"echo tool":                       {{notGo: "echo tool"}},
	} {
		if got := g.runsOf(run, dir); !slices.Equal(got, want) {
			t.Errorf("%q: %+v, want %+v", run, got, want)
		}
	}
}
