package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/mise-check/miseconfig"
	"github.com/joeblew999/mise-check/templates"
)

// theirs are the template's files a repo may have of its own already: init writes them only when it has none.
// .mise/config.toml is mise-check's pin: written once, then the repo's (mise-check:update moves it, and so does a
// newer release's init, run by hand: forward only).
var theirs = map[string]bool{".gitignore": true, ".claude/settings.json": true, ".mise/config.toml": true}

// templateConfig is the template's .mise/config.toml: mise-check's pin, its settings and their comments.
var templateConfig, _ = fs.ReadFile(templates.Tool, ".mise/config.toml")

// configTool is mise-check as its init keeps its parts of a repo's .mise/config.toml (miseconfig, which every tool's
// init imports): its pin at this release (local, a checkout's build, so a repo runs exactly what set it up), its own
// name in minimum_release_age_excludes, use_versions_host = false, and its comment at the file's top.
func configTool() miseconfig.Tool {
	release := ""
	if version != "" {
		release = buildVersion()
	}
	return miseconfig.Tool{Name: self, Release: release, Template: templateConfig, Init: initTask}
}

// initRepo is mise-check init (mise run mise-check:init): the template's tool half written into the repo, each file at its path there.
func (r repo) initRepo() {
	must(fs.WalkDir(templates.Tool, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		to := filepath.Join(r.dir, filepath.FromSlash(p))
		if b, err := os.ReadFile(to); err == nil && theirs[p] {
			if !keepOwn(p, to, b) {
				fmt.Printf("✓ %s is the repo's own, with mise-check's parts as this version writes them\n", p)
			}
			return nil
		}
		b, err := fs.ReadFile(templates.Tool, p)
		if err == nil && p == ".mise/config.toml" {
			b, err = configTool().Pinned()
		}
		if err == nil {
			writeFile(to, b)
		}
		if strings.HasSuffix(p, ".sh") {
			must(os.Chmod(to, 0o755))
		}
		return err
	}))
}

// keepOwn brings mise-check's parts of one of the repo's own files (theirs), at to, its text b, to what this version
// writes, and leaves the rest of it as it is: in .mise/config.toml its pin, its comment at the top and its settings
// (miseconfig); in .gitignore the template's lines it lacks; in .claude/settings.json the hook's SessionStart entry.
// It says what it wrote, and reports whether it did.
func keepOwn(p, to string, b []byte) bool {
	switch p {
	case ".mise/config.toml":
		out, wrote := configTool().Keep(b)
		if len(wrote) > 0 {
			must(os.WriteFile(to, out, 0o644))
		}
		for _, w := range wrote {
			fmt.Printf("wrote %s: %s\n", to, w)
		}
		return len(wrote) > 0
	case ".gitignore":
		return addIgnores(to, b)
	case ".claude/settings.json":
		return addHook(to, b)
	}
	return false
}

// ignores are the template's .gitignore lines that a repo's own .gitignore (b) lacks, each with the template's comment
// just above it.
func ignoresMissing(b []byte) []string {
	want, _ := fs.ReadFile(templates.Tool, ".gitignore")
	have := map[string]bool{}
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	var out []string
	lines := strings.Split(string(want), "\n")
	for i, l := range lines {
		if l = strings.TrimSpace(l); l == "" || strings.HasPrefix(l, "#") || have[l] {
			continue
		}
		if i > 0 && strings.HasPrefix(lines[i-1], "#") {
			l = lines[i-1] + "\n" + l
		}
		out = append(out, l)
	}
	return out
}

// addIgnores adds the template's .gitignore lines the repo's own lacks, at its end. The rest stays as it is.
func addIgnores(path string, b []byte) bool {
	missing := ignoresMissing(b)
	if len(missing) == 0 {
		return false
	}
	s := strings.TrimRight(string(b), "\n") + "\n" + strings.Join(missing, "\n") + "\n"
	must(os.WriteFile(path, []byte(s), 0o644))
	fmt.Printf("wrote %s: mise-check's lines it lacked, %d\n", path, len(missing))
	return true
}

// addHook adds the hook's SessionStart entry, as the template's .claude/settings.json has it, to the repo's own (b)
// when it doesn't run the hook, keeping every other setting and hook. One it can't read as JSON is left as it is, and
// check goes on saying so.
func addHook(path string, b []byte) bool {
	if bytes.Contains(b, []byte(templates.HookFile)) {
		return false
	}
	var own, want map[string]any
	tb, _ := fs.ReadFile(templates.Tool, ".claude/settings.json")
	if json.Unmarshal(b, &own) != nil || json.Unmarshal(tb, &want) != nil {
		fmt.Printf("· %s isn't JSON mise-check can read: add the SessionStart entry of docs/mise-check/README.md to it\n", path)
		return false
	}
	hooks, _ := own["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	ours, _ := want["hooks"].(map[string]any)["SessionStart"].([]any)
	have, _ := hooks["SessionStart"].([]any)
	hooks["SessionStart"] = append(have, ours...)
	own["hooks"] = hooks
	out, err := json.MarshalIndent(own, "", "  ")
	must(err)
	must(os.WriteFile(path, append(out, '\n'), 0o644))
	fmt.Printf("wrote %s: the hook's SessionStart entry, .claude/hooks/%s\n", path, templates.HookFile)
	return true
}

// older reports whether release a is older than release b, both x.y.z.
func older(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, errx := strconv.Atoi(pa[i])
		y, erry := strconv.Atoi(pb[i])
		if errx != nil || erry != nil {
			return false
		}
		if x != y {
			return x < y
		}
	}
	return len(pa) < len(pb)
}

// newestRelease is mise-check's newest release, as GitHub's page for it names it (…/releases/latest redirects to its
// tag), or "" if GitHub can't say.
func newestRelease() string {
	c := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get("https://github.com/joeblew999/mise-check/releases/latest")
	if err != nil {
		return ""
	}
	resp.Body.Close()
	_, tag, _ := strings.Cut(resp.Header.Get("Location"), "/releases/tag/")
	return strings.TrimPrefix(tag, "v")
}

// sayNewest says, of a release, when GitHub has a newer one, and the one line that sets the repo up at it, on the same
// line: @latest runs the release mise lists as its newest, and mise lists one some minutes after it's out (its
// versions host, and GitHub's own API, which caches a minute or more). An exact release mise gets at once, and a
// latest pin then runs it, the newest installed.
func sayNewest() {
	if version == "" {
		return
	}
	switch n := newestRelease(); {
	case n == "":
		fmt.Printf("· this is mise-check %s; GitHub didn't say what its newest release is\n", buildVersion())
	case older(buildVersion(), n):
		fmt.Printf("· this is mise-check %s, and its newest release is %s (mise lists one as latest some minutes after it's out). To set the repo up at %s: %s\n", buildVersion(), n, n, initAt(n))
	default:
		fmt.Printf("✓ mise-check %s, the newest release\n", buildVersion())
	}
}

// initAt is the one line that sets a repo up at a release of mise-check (0.4.30), whatever runs now: mise gets an exact
// release at once, as no list of releases is read for it; init then writes its files as that release does and moves
// an older pin to it, and a pin of latest runs it, the newest installed.
func initAt(release string) string {
	return "mise exec " + self + "@" + release + " -- mise-check init"
}

// stale names the files init wrote that this version would write differently, and the repo's own files whose
// mise-check parts init would write again (keepOwn): a .gitignore without the template's lines, a .claude/settings.json
// that doesn't run the hook. Those of .mise/config.toml are miseconfig's (rules).
func (r repo) stale() []string {
	var out []string
	fs.WalkDir(templates.Tool, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || theirs[p] {
			return err
		}
		want, _ := fs.ReadFile(templates.Tool, p)
		// Line endings aside: a Windows checkout of the repo may have \r\n where init wrote \n.
		crlf := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
		b, err := os.ReadFile(filepath.Join(r.dir, filepath.FromSlash(p)))
		switch {
		case err != nil:
			out = append(out, p+" is missing: run mise run mise-check:init")
		case !bytes.Equal(crlf(b), crlf(want)):
			out = append(out, p+" is out of date: run mise run mise-check:init")
		}
		return nil
	})
	if b, err := os.ReadFile(filepath.Join(r.dir, ".claude", "settings.json")); err == nil && !bytes.Contains(b, []byte(templates.HookFile)) {
		out = append(out, ".claude/settings.json doesn't run .claude/hooks/"+templates.HookFile+": run mise run mise-check:init")
	}
	if b, err := os.ReadFile(filepath.Join(r.dir, ".gitignore")); err == nil && len(ignoresMissing(b)) > 0 {
		out = append(out, ".gitignore lacks mise-check's lines ("+strings.Join(ignoresMissing(b), "; ")+"): run mise run mise-check:init")
	}
	return out
}

func writeFile(path string, b []byte) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		fmt.Printf("✓ %s is up to date\n", path)
		return
	}
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	must(os.WriteFile(path, b, 0o644))
	fmt.Println("wrote", path)
}

// trustConfig has mise trust a config file mise-check wrote, so its tasks run straight away.
func trustConfig(dir, file string) {
	path := filepath.Join(dir, filepath.FromSlash(file))
	if _, err := os.Stat(path); err != nil {
		return
	}
	if _, err := exec.LookPath("mise"); err != nil {
		return
	}
	if out, err := exec.Command("mise", "trust", "-q", path).CombinedOutput(); err != nil {
		fmt.Printf("mise didn't trust %s (%v %s): run mise trust %s\n", file, err, strings.TrimSpace(string(out)), file)
		return
	}
	fmt.Printf("✓ %s trusted by mise\n", file)
}
