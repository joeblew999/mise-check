package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/mise-check/miseconfig"
	"github.com/joeblew999/mise-check/templates"
	"github.com/joeblew999/mise-check/toolfiles"
)

// templateConfig is the template's .mise/config.toml: mise-check's pin, its settings and their comments.
var templateConfig, _ = fs.ReadFile(templates.Tool, ".mise/config.toml")

// toolFiles is mise-check as its init writes its files into a repo (toolfiles, which every tool's init imports): the
// template's tool half, the hook, and its parts of .mise/config.toml (miseconfig): its pin at this release (local, a
// checkout's build, so a repo runs exactly what set it up), its own name in minimum_release_age_excludes,
// use_versions_host = false, and its comment at the file's top.
func toolFiles() toolfiles.Tool {
	release := ""
	if version != "" {
		release = buildVersion()
	}
	return toolfiles.Tool{Files: templates.Tool, Hook: templates.HookFile,
		Config: miseconfig.Tool{Name: self, Release: release, Template: templateConfig, Init: initTask}}
}

// initRepo is mise-check init (mise run mise-check:init): the template's tool half written into the repo, each file at
// its path there; of the repo's own .mise/config.toml, .gitignore and .claude/settings.json, mise-check's parts only.
func (r repo) initRepo() { must(toolFiles().Write(r.dir)) }

// trustConfig has mise trust a config file mise-check wrote, so its tasks run straight away.
func trustConfig(dir, file string) { toolfiles.Trust(dir, file, nil) }

func writeFile(path string, b []byte) {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		fmt.Printf("✓ %s is up to date\n", path)
		return
	}
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	must(os.WriteFile(path, b, 0o644))
	fmt.Println("wrote", path)
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
