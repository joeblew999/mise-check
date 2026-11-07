package walk

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestTasks: a mise file's tasks, in their order, in a file of tasks only and in a mise.toml; a name in a run line
// isn't one.
func TestTasks(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"tasks.toml": "[\"greet\"]\nrun = 'greet'\n\n[\"greet:init\"]\nrun = 'greet init'\n\n[\"greet:verify\"]\nrun = 'mise run greet:check'\n\n[\"greet:check\"]\r\nrun = 'greet check'\r\n",
		"mise.toml":  "[tools]\ngo = \"1.27\"\n\n[tasks.greet]\nrun = 'greet'\n\n[tasks.\"greet:init\"]\nrun = 'greet init'\n\n[tasks.\"greet:verify\"]\nrun = 'mise run greet:check'\n\n[tasks.\"greet:check\"]\nrun = 'greet check'\n",
	} {
		file := filepath.Join(dir, name)
		os.WriteFile(file, []byte(text), 0o644)
		got, err := Tasks(file)
		if want := []string{"greet", "greet:init", "greet:verify", "greet:check"}; err != nil || !slices.Equal(got, want) {
			t.Errorf("%s: %v, %v; want %v", name, got, err, want)
		}
	}
	if _, err := Tasks(filepath.Join(dir, "none.toml")); err == nil {
		t.Error("a file that isn't there: no error")
	}
}

// TestStepsAgree: steps in the file's order with the others are the file's tasks; out of order, one missing, one
// that isn't in the file, or one named twice, aren't.
func TestStepsAgree(t *testing.T) {
	tasks := []string{"greet", "greet:init", "greet:verify", "greet:check"}
	if err := StepsAgree("f", tasks, []string{"greet:init", "greet:check"}, []string{"greet", "greet:verify"}); err != nil {
		t.Errorf("agree: %v", err)
	}
	for name, c := range map[string][2][]string{
		"out of order":    {{"greet:check", "greet:init"}, {"greet", "greet:verify"}},
		"one missing":     {{"greet:init", "greet:check"}, {"greet"}},
		"not in the file": {{"greet:init", "greet:check"}, {"greet", "greet:verify", "greet:deploy"}},
		"named twice":     {{"greet:init", "greet:check"}, {"greet", "greet:verify", "greet:init"}},
	} {
		if err := StepsAgree("f", tasks, c[0], c[1]); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// TestSnapshot: Snapshot hashes every file but the git and what it's told to skip, CopyRepo copies the same, and Diff
// names what was added, removed or changed.
func TestSnapshot(t *testing.T) {
	src := t.TempDir()
	for path, text := range map[string]string{"a.txt": "a", "sub/b.txt": "b", ".git/HEAD": "ref", "node_modules/x.js": "x", "sub/node_modules/y.js": "y"} {
		os.MkdirAll(filepath.Join(src, filepath.Dir(path)), 0o755)
		os.WriteFile(filepath.Join(src, path), []byte(text), 0o644)
	}
	r := Repo{T: t, Dir: src}
	before := r.Snapshot("node_modules")
	if got := slices.Sorted(maps.Keys(before)); !slices.Equal(got, []string{"a.txt", "sub/b.txt"}) {
		t.Errorf("snapshot: %v", got)
	}
	dst := filepath.Join(t.TempDir(), "copy")
	if err := CopyRepo(src, dst, "node_modules"); err != nil {
		t.Fatal(err)
	}
	if d := Diff(before, (Repo{T: t, Dir: dst}).Snapshot()); len(d) > 0 {
		t.Errorf("the copy differs: %v", d)
	}
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("changed"), 0o644)
	os.Remove(filepath.Join(src, "sub", "b.txt"))
	os.WriteFile(filepath.Join(src, "c.txt"), []byte("c"), 0o644)
	if d := Diff(before, r.Snapshot("node_modules")); !slices.Equal(d, []string{"a.txt", "c.txt", "sub/b.txt"}) {
		t.Errorf("diff: %v", d)
	}
}

// TestSteps: the steps run in their order, as far as the stage asked; a later stage's, or one that says why it's
// skipped, doesn't run.
func TestSteps(t *testing.T) {
	var ran []string
	run := func(name string) func(*[]string) { return func(r *[]string) { *r = append(*r, name) } }
	steps := []Step[*[]string]{
		{Task: "greet:init", Check: run("greet:init")},
		{Task: "greet:check", Skip: "not here"},
		{Task: "greet:show", Check: run("greet:show")},
		{Task: "greet:deploy", Stage: 1, Check: run("greet:deploy")},
	}
	Steps(Repo{T: t}, &ran, steps, 0)
	if want := []string{"greet:init", "greet:show"}; !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
}
