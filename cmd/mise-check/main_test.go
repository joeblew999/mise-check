package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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

// TestExcludes: init lets mise take the owner's tools at their newest release, in a repo's own .mise/config.toml, however
// it has its [settings]; and check, and so show, say to run init while one isn't there.
func TestExcludes(t *testing.T) {
	all := `minimum_release_age_excludes = ["github:joeblew999/mise-check", "github:joeblew999/mise-intl", "github:joeblew999/mise-cloudflare"]`
	for _, c := range []struct{ name, in, want string }{
		{"no [settings]", "[tools]\nnode = \"26\"\n", "[settings]\n" + all + "\n\n[tools]\nnode = \"26\"\n"},
		{"a [settings] of its own", "[settings]\nexperimental = true\n\n[tools]\n", "[settings]\n" + all + "\nexperimental = true\n\n[tools]\n"},
		{"a list of its own, one of them there", "[settings]\nminimum_release_age_excludes = [\"npm:*\", \"github:joeblew999/mise-intl\"]\n",
			"[settings]\nminimum_release_age_excludes = [\"npm:*\", \"github:joeblew999/mise-intl\", \"github:joeblew999/mise-check\", \"github:joeblew999/mise-cloudflare\"]\n"},
		{"every one there", "[settings]\n" + all + "\n", "[settings]\n" + all + "\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(p, []byte(c.in), 0o644); err != nil {
				t.Fatal(err)
			}
			wrote := addExcludes(p, []byte(c.in))
			b, _ := os.ReadFile(p)
			if string(b) != c.want || wrote != (c.in != c.want) {
				t.Errorf("wrote %v:\n%s\nwant:\n%s", wrote, b, c.want)
			}
		})
	}
	r := repo{dir: t.TempDir()}
	r.initRepo()
	cfg := filepath.Join(r.dir, ".mise", "config.toml")
	b, _ := os.ReadFile(cfg)
	if err := os.WriteFile(cfg, excludesLine.ReplaceAll(b, nil), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := r.status(); s.Next != "mise run mise-check:init" {
		t.Fatalf("a config.toml without the excludes: next %q, want mise run mise-check:init (problems %q)", s.Next, s.Problems)
	}
	r.initRepo()
	if s := r.status(); len(excludesMissing(mustRead(t, cfg))) > 0 || slicesHas(s.Problems, "minimum_release_age_excludes") {
		t.Fatalf("init again didn't add them back: problems %q", s.Problems)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func slicesHas(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
