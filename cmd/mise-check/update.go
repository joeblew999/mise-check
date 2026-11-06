package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// self is mise-check as a repo's mise pins it (.mise/config.toml, which init writes).
const self = "github:joeblew999/mise-check"

// goModUpgrade is the Go module upgrader update runs (github.com/oligot/go-mod-upgrade), at the version mise-check's
// own go.mod pins on its tool line, by go run of it at that version: a repo needs no line of its own for it (one would
// raise the repo's go line and put the upgrader's own modules in its go.mod). TestPin fails when this and go.mod
// differ, so mise run root:update, which moves mise-check's own go.mod, says to move this with it.
const goModUpgrade = "github.com/oligot/go-mod-upgrade@v0.13.0"

// cooldown is how old a Go module's release must be before update takes it, as mise and aube skip a release younger
// than their minimum age.
const cooldown = "7d"

// update is mise-check update: the repo's own pins to their newest releases, each with its own tool (mise reads no parent folder's
// config, so nothing outside the repo moves): its mise tools (mise upgrade --bump), its packages (aube, which writes
// whichever lockfile the repo has) when it has them, and in every go.mod of the repo: its Go (the toolchain line; the go
// line, the least Go the module needs, stays) and its modules, majors too, with go-mod-upgrade, which rewrites a major's
// import paths (go mod tidy before, as it skips // indirect lines, and after). mise-check at local, a checkout's build,
// stays there; moved to a newer release, it says to write init's files again (03-init), which it doesn't do itself.
// It ends with what moved; git diff shows it, and mise run mise-check:verify proves it (a major move can break).
// A developer who wants a say moves one kind only (mise-update, packages-update, go-update), is asked for each one
// (--pick), or takes no majors (--no-major).
func (r repo) update(o updates) { r.updatePins(o, "mise", "packages", "go") }

// updatePins moves the pins of the kinds named, in that order, and ends with every pin: update moves all three,
// mise-update, packages-update and go-update one each.
func (r repo) updatePins(o updates, kinds ...string) {
	does := func(kind string) bool { return slices.Contains(kinds, kind) }
	if does("packages") && r.exists("package.json") && r.mise("which", "aube") == "" {
		fail(errors.New("the packages update with aube, which writes whichever lockfile the repo has, and the repo's mise doesn't pin it: mise use aube"))
	}
	mods := r.goModules()
	before := r.pins(mods)
	if does("mise") {
		upgrade := []string{"upgrade", "--bump", "--no-global"}
		if o.pick {
			upgrade = append(upgrade, "--interactive")
		}
		if selfPin(before) == "local" {
			upgrade = append(upgrade, "--exclude", self)
		}
		r.runOwnMise(r.dir, upgrade...)
	}
	if does("packages") && r.exists("package.json") {
		aube := []string{"exec", "--", "aube", "update"}
		if !o.noMajor {
			aube = append(aube, "--latest") // past each range: majors too
		}
		if o.pick {
			aube = append(aube, "--interactive")
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
		if o.noMajor {
			gmu = append(gmu, "--no-major")
		}
		if !o.pick {
			gmu = append(gmu, "--force") // every one; --pick asks for each
		}
		for _, m := range mods {
			r.runOwnMise(m, "exec", "--", "go", "mod", "tidy")
			r.runOwnMise(m, gmu...)
			r.runOwnMise(m, "exec", "--", "go", "mod", "tidy")
		}
	}
	after := r.pins(mods)
	if b, a := selfPin(before), selfPin(after); a != b {
		fmt.Printf("mise-check moved from %s to %s: mise run mise-check:03-init writes its files again (04-check fails until it has)\n", b, a)
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
	pick    bool // ask for each one, with the picker of each tool
	noMajor bool // no major moves: packages within their ranges, Go modules within their major
}

// goModules are the folders of every go.mod in the repo, its own first: none in vendor, node_modules, testdata or a
// folder whose name starts with a dot.
func (r repo) goModules() []string {
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
			Toolchain struct{ Name string }
			Require   []struct{ Path, Version string }
		}
		if json.Unmarshal([]byte(r.miseIn(m, "exec", "--", "go", "mod", "edit", "-json")), &mod) != nil {
			continue
		}
		file := filepath.ToSlash(filepath.Join(r.rel(m), "go.mod"))
		p[file+" toolchain"] = strings.TrimPrefix(cmp.Or(mod.Toolchain.Name, "go"+mod.Go), "go")
		for _, req := range mod.Require {
			p[file+" "+req.Path] = req.Version
		}
	}
	return p
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
	fmt.Println("$ mise " + strings.Join(args, " ") + at)
	cmd := exec.Command("mise", args...)
	cmd.Dir, cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = dir, r.ownMise(), os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fail(fmt.Errorf("mise %s%s: %w", strings.Join(args, " "), at, err))
	}
}

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
