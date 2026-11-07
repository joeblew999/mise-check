package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// self is mise-check as a repo's mise pins it (.mise/config.toml, which init writes).
const self = "github:joeblew999/mise-check"

// goModUpgrade is the Go module upgrader update-go runs (github.com/oligot/go-mod-upgrade), by go run of it at this
// version: a repo needs nothing of its own for it. Its pin is mise-check's root mise.toml ("go:<module>"), which
// update-mise moves; TestPin fails when this and that differ. Not a tool line of go.mod: it would raise mise-check's go
// line, and with it every importer's.
const goModUpgrade = "github.com/oligot/go-mod-upgrade@v0.13.0"

// cooldown is how old a Go module's release must be before update takes it, as mise and aube skip a release younger
// than their minimum age.
const cooldown = "7d"

// update is mise-check update: the repo's own pins to their newest releases, each with its own tool (mise reads no parent folder's
// config, so nothing outside the repo moves): its mise tools (mise upgrade --bump), its packages (aube, which writes
// whichever lockfile the repo has) when it has them, and in every go.mod of the repo: its Go (the toolchain line; the go
// line, the least Go the module needs, stays) and its modules, majors too, with go-mod-upgrade, which rewrites a major's
// import paths (go mod tidy before, as it skips // indirect lines, and after). mise-check at local, a checkout's build,
// stays there; moved to a newer release, it says to write init's files again (mise-check:init), which it doesn't do itself.
// It ends with what moved; git diff shows it, and mise run mise-check:verify proves it (a major move can break).
// A developer who wants a say moves one kind only (update-mise, update-packages, update-go), or takes no majors
// (--no-major).
func (r repo) update(o updates) { r.updatePins(o, "mise", "packages", "go") }

// updatePins moves the pins of the kinds named, in that order, and ends with every pin: update moves all three,
// update-mise, update-packages and update-go one each.
func (r repo) updatePins(o updates, kinds ...string) {
	does := func(kind string) bool { return slices.Contains(kinds, kind) }
	if does("packages") && r.exists("package.json") && r.mise("which", "aube") == "" {
		fail(errors.New("the packages update with aube, which writes whichever lockfile the repo has, and the repo's mise doesn't pin it: mise use aube"))
	}
	mods := r.goModules()
	before := r.pins(mods)
	if does("mise") {
		// The owner's tools' cached lists of releases, cleared first: mise keeps a tool's list an hour
		// (fetch_remote_versions_cache), and mise exec reads whatever it has, so a release the minute it's out is seen.
		r.runOwnMise(r.dir, append([]string{"cache", "clear"}, excludes...)...)
		upgrade := []string{"upgrade", "--bump", "--no-global"}
		for _, name := range r.atLocal() { // a checkout's own build (mise link, mise-check's or any tool's): it stays
			upgrade = append(upgrade, "--exclude", name)
		}
		for _, name := range r.fromGoMod() { // Go, which go.mod pins: update-go's, as mise can't write go.mod
			upgrade = append(upgrade, "--exclude", name)
		}
		r.runOwnMise(r.dir, upgrade...)
	}
	if does("packages") && r.exists("package.json") {
		aube := []string{"exec", "--", "aube", "update"}
		if !o.noMajor {
			aube = append(aube, "--latest") // past each range: majors too
		}
		r.runOwnMise(r.dir, aube...)
	}
	if does("go") {
		for _, m := range mods {
			r.runOwnMise(m, "exec", "--", "go", "get", "toolchain@latest")
		}
		if len(mods) > 0 {
			r.runOwnMise(r.dir, "install") // the Go go.mod's toolchain line pins now, which mise installs from it
		}
		gmu := []string{"exec", "--", "go", "run", goModUpgrade, "--cooldown", cooldown}
		own := r.ownModules()
		if own != "" { // the owner's own modules: no cooldown, below
			gmu = append(gmu, "--ignore", "^"+regexp.QuoteMeta(own))
		}
		if o.noMajor {
			gmu = append(gmu, "--no-major")
		}
		gmu = append(gmu, "--force") // every one, without asking
		for _, m := range mods {
			r.runOwnMise(m, "exec", "--", "go", "mod", "tidy")
			r.runOwnMise(m, gmu...)
			// The owner's own modules at their newest release, the moment it's out: the cooldown is for everyone else's,
			// a bad release of theirs; yours you want at once (show-users says which repos are behind them).
			if mine := r.requiresOf(m, own); len(mine) > 0 {
				r.runOwnMise(m, append([]string{"exec", "--", "go", "get"}, mine...)...)
			}
			r.runOwnMise(m, "exec", "--", "go", "mod", "tidy")
		}
	}
	// mise-check's two pins, the binary and the packages, at one release: update moves both to the newer; an update of
	// one kind moves only its own, and says when they're apart.
	if s := r.selves(mods); len(kinds) == 3 {
		r.together(s)
	} else if p := s.apart(); p != "" {
		fmt.Fprintln(os.Stderr, "! "+p)
	}
	after := r.pins(mods)
	if o.json {
		emit(pinsMoved(before, after))
		return
	}
	if b, a := selfPin(before), selfPin(after); a != b {
		fmt.Printf("mise-check moved from %s to %s: mise run mise-check:init writes its files again (mise-check:check fails until it has)\n", b, a)
	}
	lines, n := moved(before, after)
	if len(lines) == 0 {
		fail(errors.New("no pins read: mise ls --no-global --json, package.json and go.mod named none"))
	}
	if n == 0 {
		fmt.Println("✓ nothing moved: each pin is its newest release (mise, aube and go-mod-upgrade skip one younger than their minimum age)")
	} else {
		fmt.Printf("✓ %d of %d pins moved (git diff shows them; mise run mise-check:verify proves them; a major move can break):\n", n, len(lines))
	}
	for _, l := range lines {
		fmt.Println("  " + l)
	}
}

// updates are the update commands' choices, their flags.
type updates struct {
	json    bool // one JSON document on stdout, every pin before → after; the tools' own output on stderr
	noMajor bool // no major moves: packages within their ranges, Go modules within their major
}

// goModules are the folders of every go.mod of the repo's own, its top's first: the ones git tracks or would (git
// ls-files), so nothing git ignores (another repo's clone, say: mise-cloudflare's repos/) and no other repo inside it
// is ever updated. In a folder that isn't a git repo, every go.mod under it but those in vendor, node_modules, testdata
// or a folder whose name starts with a dot.
func (r repo) goModules() []string {
	ls := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "--", "go.mod", "**/go.mod")
	ls.Dir = r.dir
	if out, err := ls.Output(); err == nil {
		var mods []string
		for _, f := range strings.Fields(string(out)) {
			if !strings.Contains("/"+f, "/vendor/") && !strings.Contains("/"+f, "/testdata/") {
				mods = append(mods, filepath.Join(r.dir, filepath.Dir(filepath.FromSlash(f))))
			}
		}
		slices.SortFunc(mods, func(a, b string) int { return cmp.Compare(len(a), len(b)) })
		return mods
	}
	var mods []string
	must(filepath.WalkDir(r.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != r.dir {
			switch name := d.Name(); {
			case name == "vendor", name == "node_modules", name == "testdata", strings.HasPrefix(name, "."):
				return filepath.SkipDir
			}
		}
		if !d.IsDir() && d.Name() == "go.mod" {
			mods = append(mods, filepath.Dir(path))
		}
		return nil
	}))
	return mods
}

// pins is every version the repo pins of its own, by file and name: its mise tools by the file mise read each from
// ("mise.toml node"), its packages ("package.json wrangler"), and each go.mod's Go ("go.mod toolchain", the toolchain
// line, or the go line with none) and modules ("go.mod golang.org/x/term"; a go.mod in a folder by its path,
// "tools/go.mod …"), each as that file's tool reports it.
func (r repo) pins(mods []string) map[string]string {
	p := map[string]string{}
	var tools map[string][]struct {
		Version string
		Source  struct{ Path string }
	}
	if json.Unmarshal([]byte(r.mise("ls", "--no-global", "--json")), &tools) == nil {
		for name, vs := range tools {
			for _, v := range vs {
				if v.Source.Path != "" && filepath.Base(v.Source.Path) != "go.mod" { // a file names it, installed yet or not; go.mod's Go is below
					p[filepath.Base(v.Source.Path)+" "+name] = v.Version
				}
			}
		}
	}
	var pkg struct{ Dependencies, DevDependencies map[string]string }
	if b, err := os.ReadFile(filepath.Join(r.dir, "package.json")); err == nil && json.Unmarshal(b, &pkg) == nil {
		for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
			for name, v := range deps {
				p["package.json "+name] = v
			}
		}
	}
	for _, m := range mods {
		var mod struct {
			Go        string
			Toolchain string // go mod edit -json writes it as a string: "go1.27.3"
			Require   []struct{ Path, Version string }
		}
		if json.Unmarshal([]byte(r.miseIn(m, "exec", "--", "go", "mod", "edit", "-json")), &mod) != nil {
			continue
		}
		file := filepath.ToSlash(filepath.Join(r.rel(m), "go.mod"))
		p[file+" toolchain"] = strings.TrimPrefix(cmp.Or(mod.Toolchain, "go"+mod.Go), "go")
		for _, req := range mod.Require {
			p[file+" "+req.Path] = req.Version
		}
	}
	return p
}

// atLocal are the tools the repo's mise pins at local, a checkout's own build linked with mise link (mise run
// tool:install): mise upgrade --bump would move them to a release.
func (r repo) atLocal() []string {
	var tools map[string][]struct{ Version string }
	var names []string
	if json.Unmarshal([]byte(r.mise("ls", "--no-global", "--json")), &tools) == nil {
		for name, vs := range tools {
			if slices.ContainsFunc(vs, func(v struct{ Version string }) bool { return v.Version == "local" }) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return names
}

// ownModules is the module path prefix of the repo's owner, github.com/<owner>/, from its go.mod's module path; ""
// when it isn't on GitHub.
func (r repo) ownModules() string {
	var f struct{ Module struct{ Path string } }
	for _, m := range r.goModules() {
		if json.Unmarshal([]byte(r.miseIn(m, "exec", "--", "go", "mod", "edit", "-json")), &f) == nil {
			if p := strings.Split(f.Module.Path, "/"); len(p) >= 2 && p[0] == "github.com" {
				return "github.com/" + p[1] + "/"
			}
		}
	}
	return ""
}

// requiresOf are the modules a go.mod requires under the prefix, each at @latest, for go get: none for "" or none there.
func (r repo) requiresOf(mod, prefix string) []string {
	if prefix == "" {
		return nil
	}
	var f struct {
		Require []struct{ Path string }
	}
	var out []string
	if json.Unmarshal([]byte(r.miseIn(mod, "exec", "--", "go", "mod", "edit", "-json")), &f) == nil {
		for _, q := range f.Require {
			if strings.HasPrefix(q.Path, prefix) {
				out = append(out, q.Path+"@latest")
			}
		}
	}
	return out
}

// fromGoMod are the tools mise reads from a go.mod (go, with idiomatic_version_file_enable_tools), which mise upgrade
// --bump can't write: update-go moves them.
func (r repo) fromGoMod() []string {
	var tools map[string][]struct{ Source struct{ Path string } }
	var names []string
	if json.Unmarshal([]byte(r.mise("ls", "--no-global", "--json")), &tools) == nil {
		for name, vs := range tools {
			for _, v := range vs {
				if filepath.Base(v.Source.Path) == "go.mod" && !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

// selfPin is the version of mise-check the repo's mise pins, from whichever file it does: "" if none.
func selfPin(p map[string]string) string {
	for k, v := range p {
		if strings.HasSuffix(k, " "+self) {
			return v
		}
	}
	return ""
}

// pin is one pin in update --json: the file that pins it, its name, its version before and after the update ("" for
// none: a module a major move renamed), whether it moved, and whether that was a major move.
type pin struct {
	File   string `json:"file"`
	Name   string `json:"name"`
	Before string `json:"before"`
	After  string `json:"after"`
	Moved  bool   `json:"moved"`
	Major  bool   `json:"major,omitempty"`
}

// pinsMoved is update --json: every pin, in the order of their file and name, and how many moved.
func pinsMoved(before, after map[string]string) any {
	all := maps.Clone(before)
	maps.Copy(all, after)
	pins := []pin{}
	n := 0
	for _, k := range slices.Sorted(maps.Keys(all)) {
		file, name, _ := strings.Cut(k, " ")
		b, a := before[k], after[k]
		p := pin{File: file, Name: name, Before: b, After: a, Moved: a != b}
		p.Major = p.Moved && b != "" && a != "" && breaking(b, a)
		if p.Moved {
			n++
		}
		pins = append(pins, p)
	}
	return struct {
		Moved int   `json:"moved"`
		Pins  []pin `json:"pins"`
	}{n, pins}
}

// moved is every pin, one line each in the order of their file and name, and how many moved: one that stayed is
// "aube 2.8.4 (mise.toml)", one that moved "node 26.12.1 → 26.13.0 (mise.toml)"; one only before or after has
// "none" for the other (a module a major move renamed: github.com/go-chi/chi → none, github.com/go-chi/chi/v5 none → …).
func moved(before, after map[string]string) ([]string, int) {
	all := maps.Clone(before)
	maps.Copy(all, after)
	var lines []string
	n := 0
	for _, k := range slices.Sorted(maps.Keys(all)) {
		file, name, _ := strings.Cut(k, " ")
		b, a := before[k], after[k]
		if a == b {
			lines = append(lines, fmt.Sprintf("%s %s (%s)", name, a, file))
			continue
		}
		major := ""
		if b != "" && a != "" && breaking(b, a) {
			major = ", a major move"
		}
		lines = append(lines, fmt.Sprintf("%s %s → %s (%s)%s", name, cmp.Or(b, "none"), cmp.Or(a, "none"), file, major))
		n++
	}
	return lines, n
}

// breaking reports whether moving from version b to a is a major move, as semver counts one: its major number, or for a
// 0.x version its minor number (esbuild 0.19 → 0.28). The versions are as their files write them (^5.5.2, v1.2.3).
func breaking(b, a string) bool {
	parts := func(v string) []string {
		return strings.SplitN(strings.TrimLeft(v, "^~=<>v "), ".", 3)
	}
	pb, pa := parts(b), parts(a)
	if pb[0] != pa[0] {
		return true
	}
	return pb[0] == "0" && len(pb) > 1 && len(pa) > 1 && pb[1] != pa[1]
}

// ownMise is the environment that keeps mise to the repo's own config, as in a repo on its own. mise stops at a ceiling
// equal to a parent of its working folder exactly as the OS reports it (mise's src/config/miserc.rs): on macOS and Linux
// the real path, links resolved (/tmp is /private/tmp on macOS); on Windows the path as given, links not resolved, in
// long or short names. So the ceiling is each of those forms, and mise stops at whichever it sees.
func (r repo) ownMise() []string {
	forms := []string{r.dir, longPath(r.dir)}
	if real, err := filepath.EvalSymlinks(r.dir); err == nil {
		forms = append(forms, real, longPath(real))
	}
	var ceilings []string
	for _, f := range forms {
		if d := filepath.Dir(f); !slices.Contains(ceilings, d) {
			ceilings = append(ceilings, d)
		}
	}
	return append(os.Environ(), "MISE_CEILING_PATHS="+strings.Join(ceilings, string(filepath.ListSeparator)))
}

// mise is what mise prints for args on the repo's own config, "" when it fails.
func (r repo) mise(args ...string) string { return r.miseIn(r.dir, args...) }

// miseIn is mise's output for args in dir, a folder of the repo, on the repo's own config: "" when it fails.
func (r repo) miseIn(dir string, args ...string) string {
	cmd := exec.Command("mise", args...)
	cmd.Dir, cmd.Env = dir, r.ownMise()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runOwnMise runs mise with args in dir, a folder of the repo, on the repo's own config, as the person would, and stops
// at a failure.
func (r repo) runOwnMise(dir string, args ...string) {
	at := ""
	if dir != r.dir {
		at = " (in " + r.rel(dir) + ")"
	}
	fmt.Fprintln(out, "$ mise "+strings.Join(args, " ")+at)
	cmd := exec.Command("mise", args...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, r.ownMise(), os.Stdin, out, os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("mise %s%s: %w", strings.Join(args, " "), at, err))
	}
}

// out is where the tools' own output goes: stdout, or stderr with --json, which keeps stdout to its one JSON document.
var out io.Writer = os.Stdout

// exists reports whether the repo has the file f.
func (r repo) exists(f string) bool {
	_, err := os.Stat(filepath.Join(r.dir, f))
	return err == nil
}

// rel is path, a folder of the repo, from the repo's top, with slashes: "." for the top itself.
func (r repo) rel(path string) string {
	if p, err := filepath.Rel(r.dir, path); err == nil {
		return filepath.ToSlash(p)
	}
	return path
}
