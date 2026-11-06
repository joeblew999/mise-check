package walk

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestNumbered: a mise file's numbered tasks, in their order, in a file of tasks only and in a mise.toml; the others,
// and a numbered name in a run line, aren't.
func TestNumbered(t *testing.T) {
	dir := t.TempDir()
	for name, text := range map[string]string{
		"tasks.toml": "[\"greet\"]\nrun = 'greet'\n\n[\"greet:03-init\"]\nrun = 'greet init'\n\n[\"greet:verify\"]\nrun = 'mise run greet:04-check'\n\n[\"greet:04-check\"]\r\nrun = 'greet check'\r\n",
		"mise.toml":  "[tools]\ngo = \"1.27\"\n\n[tasks.\"greet:03-init\"]\nrun = 'greet init'\n\n[tasks.\"greet:04-check\"]\nrun = 'greet check'\n",
	} {
		file := filepath.Join(dir, name)
		os.WriteFile(file, []byte(text), 0o644)
		got, err := Numbered(file)
		if want := []string{"greet:03-init", "greet:04-check"}; err != nil || !slices.Equal(got, want) {
			t.Errorf("%s: %v, %v; want %v", name, got, err, want)
		}
	}
	if _, err := Numbered(filepath.Join(dir, "none.toml")); err == nil {
		t.Error("a file that isn't there: no error")
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
		{Task: "greet:03-init", Check: run("greet:03-init")},
		{Task: "greet:04-check", Skip: "not here"},
		{Task: "greet:05-show", Check: run("greet:05-show")},
		{Task: "greet:06-deploy", Stage: 1, Check: run("greet:06-deploy")},
	}
	Steps(Repo{T: t}, &ran, steps, 0)
	if want := []string{"greet:03-init", "greet:05-show"}; !slices.Equal(ran, want) {
		t.Errorf("ran %v, want %v", ran, want)
	}
}
