package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/joeblew999/mise-check/templates"
)

// TestPin: the go-mod-upgrade update-go runs is the one mise-check's root mise.toml pins ("go:<module>" = "<version>"),
// so moving it there (mise run root:update) moves what every repo's update-go runs. When they differ, it says what to
// change. The pin isn't a tool line of go.mod: that would raise mise-check's go line, and every importer's with it.
func TestPin(t *testing.T) {
	b, err := os.ReadFile("../../mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	path, want, _ := strings.Cut(goModUpgrade, "@")
	m := regexp.MustCompile(`"go:` + regexp.QuoteMeta(path) + `"\s*=\s*"([^"]*)"`).FindSubmatch(b)
	switch {
	case m == nil:
		t.Fatalf("mise.toml doesn't pin \"go:%s\": it's the pin of update.go's goModUpgrade", path)
	case "v"+string(m[1]) != want:
		t.Fatalf("mise.toml pins go:%s %s, and update.go's goModUpgrade is @%s: make it %s@v%s", path, m[1], want, path, m[1])
	}
}

// TestOneRelease: mise-check's two pins, the binary's and the packages', are apart only when both are there and differ.
func TestOneRelease(t *testing.T) {
	for _, c := range []struct {
		s    selves
		want bool
	}{
		{selves{mise: "0.4.20", module: "v0.4.20"}, false},
		{selves{mise: "0.4.19", module: "v0.4.20"}, true},
		{selves{mise: "0.4.20", module: "v0.4.9"}, true},
		{selves{mise: "", module: "v0.4.20"}, false},
		{selves{mise: "0.4.20", module: ""}, false},
	} {
		if got := c.s.apart() != ""; got != c.want {
			t.Errorf("%+v: apart %v, want %v", c.s, got, c.want)
		}
	}
	if !newer("v0.4.20", "v0.4.9") || newer("v0.4.9", "v0.4.20") || newer("v0.4.20", "v0.4.20") {
		t.Error("newer compares releases by number")
	}
	// The binary the older: the way out is init at the packages' release, which needs nothing of the old binary.
	if p := (selves{mise: "0.4.19", module: "v0.4.20"}).apart(); !strings.Contains(p, initAt("0.4.20")) {
		t.Errorf("the binary behind the packages: %q, want %q", p, initAt("0.4.20"))
	}
}

// TestShow: show's next step, as a repo meets them: init's files missing, then init done and no Go program, then a
// mise tool repo's binary (cmd/<name>/main.go), which is a Go program of its own, so the template isn't next; nor in a
// repo with only a go.mod.
func TestShow(t *testing.T) {
	r := repo{dir: t.TempDir()}
	if s := r.status(); s.Next != "mise run mise-check:init" {
		t.Fatalf("no init's files: next %q, want mise run mise-check:init (problems %q)", s.Next, s.Problems)
	}
	r.initRepo()
	if s := r.status(); s.Next != "mise run mise-check:template" || len(s.Problems) > 0 {
		t.Fatalf("init's files, no Go program: next %q, want mise run mise-check:template (problems %q)", s.Next, s.Problems)
	}
	if err := os.MkdirAll(filepath.Join(r.dir, "cmd", "tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.dir, "cmd", "tool", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := r.status(); s.Next != "mise run mise-check:update" || !s.App {
		t.Fatalf("a binary at cmd/tool/main.go: next %q, app %v, want mise run mise-check:update, the template not next", s.Next, s.App)
	}
	// A go.mod and no program, a library's or a tool's tests: code of its own, so the template isn't next either.
	lib := repo{dir: t.TempDir()}
	lib.initRepo()
	must(os.WriteFile(filepath.Join(lib.dir, "go.mod"), []byte("module example.com/lib\n"), 0o644))
	if s := lib.status(); s.Next != "mise run mise-check:update" || !s.App {
		t.Fatalf("a go.mod, no program: next %q, app %v, want mise run mise-check:update, the template not next", s.Next, s.App)
	}
}

// TestOwnFiles: init writes mise-check's parts of the repo's own files as this version has them, and nothing else of
// them: in .mise/config.toml (miseconfig's, which its own test holds to each part) its own name beside the repo's
// entry, the .gitignore lines, the hook in a .claude/settings.json with a hook of its own; check says to run init until
// it has.
func TestOwnFiles(t *testing.T) {
	r := repo{dir: t.TempDir()}
	r.initRepo()
	at := func(p string) string { return filepath.Join(r.dir, filepath.FromSlash(p)) }
	write := func(p, s string) { must(os.WriteFile(at(p), []byte(s), 0o644)) }
	read := func(p string) string { b, _ := os.ReadFile(at(p)); return string(b) }
	write(".mise/config.toml", "# mise-check for this repo: an older one's comment\n\n[settings]\n# The owner's tools: an older one's comment\nminimum_release_age_excludes = [\"npm:*\"]\n\n[tools]\n\"github:joeblew999/mise-check\" = \"local\"\n")
	write(".gitignore", "bin/\n")
	write(".claude/settings.json", `{"model": "opus", "hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "own.sh"}]}]}}`)
	if s := r.status(); s.Next != "mise run mise-check:init" || len(s.Problems) != 5 {
		t.Fatalf("next %q, want mise run mise-check:init, for 5 problems: %q", s.Next, s.Problems)
	}
	r.initRepo()
	if s := r.status(); len(s.Problems) > 0 {
		t.Fatalf("after init: %q", s.Problems)
	}
	// .mise/config.toml's parts are miseconfig's, its own test's: here, only that the repo's own stay.
	cfg, ignore, settings := read(".mise/config.toml"), read(".gitignore"), read(".claude/settings.json")
	for _, want := range []string{`minimum_release_age_excludes = ["npm:*", "github:joeblew999/mise-check"]`, `= "local"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config.toml has no %q:\n%s", want, cfg)
		}
	}
	if !strings.HasPrefix(ignore, "bin/\n") || !strings.Contains(ignore, ".mise/config.local.toml") {
		t.Errorf(".gitignore:\n%s", ignore)
	}
	if !strings.Contains(settings, `"opus"`) || !strings.Contains(settings, "own.sh") || !strings.Contains(settings, templates.HookFile) {
		t.Errorf("settings.json:\n%s", settings)
	}
}

// TestHeld: update keeps a tool within a range the repo declares outside mise, of one minor or one major, and no other
// range holds it.
func TestHeld(t *testing.T) {
	for v, want := range map[string]string{"==3.13.*": "3.13", "== 3.13": "3.13", "~=3.13.1": "3.13", ">=3.13,<3.14": "3.13", ">=3.11": "", ">=3.13,<4": ""} {
		if got := pythonPrefix(v); got != want {
			t.Errorf("requires-python %q: %q, want %q", v, got, want)
		}
	}
	for v, want := range map[string]string{"^24": "24", "24.x": "24", "~24.11.0": "24.11", ">=24": "", "": ""} {
		if got := nodePrefix(v); got != want {
			t.Errorf("engines.node %q: %q, want %q", v, got, want)
		}
	}
	r := repo{dir: t.TempDir()}
	must(os.WriteFile(filepath.Join(r.dir, "pyproject.toml"), []byte("[project]\nname = \"x\"\nrequires-python = \"==3.13.*\"\n"), 0o644))
	if h := r.held(); len(h) != 1 || h[0].tool != "python" || h[0].prefix != "3.13" {
		t.Errorf("held: %+v", h)
	}
}

// An EmDash or Astro site keeps its packages in site/, a test harness in tests/: each folder with a package.json and a
// lockfile beside it is a root the updates and verify act on, the top's first; a package.json with no lockfile below
// the top (a workspace member, a fixture) and one in node_modules are not.
func TestPackageRoots(t *testing.T) {
	r := repo{dir: t.TempDir()}
	write := func(rel, text string) {
		t.Helper()
		f := filepath.Join(r.dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command("git", "-C", r.dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	write("site/package.json", "{}")
	write("site/pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	write("tests/package.json", "{}")
	write("tests/package-lock.json", "{}")
	write("packages/member/package.json", "{}")
	write("site/node_modules/x/package.json", "{}")
	write("site/node_modules/x/pnpm-lock.yaml", "")
	rel := func(roots []string) (out []string) {
		for _, d := range roots {
			out = append(out, filepath.ToSlash(r.rel(d)))
		}
		return out
	}
	if got, want := rel(r.packageRoots()), []string{"site", "tests"}; !slices.Equal(got, want) {
		t.Fatalf("package roots: %v, want %v", got, want)
	}
	write("package.json", "{}")
	if got := rel(r.packageRoots()); len(got) != 3 || got[0] != "." {
		t.Fatalf("with a package.json at the top, it first: %v", got)
	}
}
