package main

import (
	"os"
	"path/filepath"
	"regexp"
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
}

// TestShow: show's next step, as a repo meets them: init's files missing, then init done and no Go program, then a
// mise tool repo's binary (cmd/<name>/main.go), which is a Go program of its own, so the template isn't next.
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
}

// TestOwnFiles: init writes mise-check's parts of the repo's own files as this version has them, and nothing else of
// them: the comment at the top of .mise/config.toml, the owner's tools in its minimum_release_age_excludes beside the
// repo's own, the .gitignore lines, the hook in a .claude/settings.json with a hook of its own; check says to run init
// until it has.
func TestOwnFiles(t *testing.T) {
	r := repo{dir: t.TempDir()}
	r.initRepo()
	at := func(p string) string { return filepath.Join(r.dir, filepath.FromSlash(p)) }
	write := func(p, s string) { must(os.WriteFile(at(p), []byte(s), 0o644)) }
	read := func(p string) string { b, _ := os.ReadFile(at(p)); return string(b) }
	write(".mise/config.toml", "# mise-check for this repo: an older one's comment\n\n[settings]\nminimum_release_age_excludes = [\"npm:*\"]\n\n[tools]\n\"github:joeblew999/mise-check\" = \"local\"\n")
	write(".gitignore", "bin/\n")
	write(".claude/settings.json", `{"model": "opus", "hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "own.sh"}]}]}}`)
	if s := r.status(); s.Next != "mise run mise-check:init" || len(s.Problems) != 4 {
		t.Fatalf("next %q, want mise run mise-check:init, for 4 problems: %q", s.Next, s.Problems)
	}
	r.initRepo()
	if s := r.status(); len(s.Problems) > 0 {
		t.Fatalf("after init: %q", s.Problems)
	}
	cfg, ignore, settings := read(".mise/config.toml"), read(".gitignore"), read(".claude/settings.json")
	for _, want := range []string{header(string(templateConfig)), `"npm:*", "github:joeblew999/mise-check"`, `= "local"`} {
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
