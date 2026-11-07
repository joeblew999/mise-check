package main

import (
	"archive/zip"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// selfRepo is this tool's own module: what new-tool copies, at this binary's release.
const selfRepo = "github.com/joeblew999/mise-check"

// packagesModule is the module of the packages a new tool repo imports, misecheck, walk and release for its tests,
// toolfiles and miseconfig for its init: mise-check's, in mise-check and in every repo new-tool makes (this line is
// kept as it is when it's renamed).
const packagesModule = "github.com/joeblew999/mise-check"

// dropped are what a new tool repo imports from mise-check instead of copying: the packages and their tests.
var dropped = []string{"misecheck.go", "misecheck_test.go", "walk/", "miseconfig/", "toolfiles/", "release/"}

// kept are the imports of mise-check that stay mise-check's in a new tool repo: the packages it imports.
var kept = []string{`misecheck "github.com/joeblew999/mise-check"`, `"github.com/joeblew999/mise-check/walk"`, `"github.com/joeblew999/mise-check/miseconfig"`, `"github.com/joeblew999/mise-check/toolfiles"`, `"github.com/joeblew999/mise-check/release"`, `packagesModule = "github.com/joeblew999/mise-check"`}

// newTool makes a new mise tool repo in the folder it runs in: mise-check at this binary's release (from Go's module
// proxy, as go get has it), or the checkout from names, copied; mise-check renamed to the module's name in every
// file's name and contents, but for the imports of the packages; the packages dropped, and required instead, at that
// release.
func (r repo) newTool(module, from string) {
	m := regexp.MustCompile(`^github\.com/([\w.-]+)/([\w.-]+)$`).FindStringSubmatch(module)
	if m == nil {
		fail(fmt.Errorf("new-tool takes the new repo's module, github.com/<owner>/<name>: mise gets its releases from GitHub"))
	}
	owner, name := m[1], m[2]
	if es, _ := os.ReadDir(r.dir); len(es) > 1 || len(es) == 1 && es[0].Name() != ".git" {
		fail(fmt.Errorf("new-tool makes the repo in an empty folder (its own git may be there already), and %s isn't one", r.dir))
	}
	// Its GitHub repo first: mise gets the tool's releases there, and runs even this machine's build (local) only once
	// it can list them.
	switch _, status := (github{githubToken()}).get("repos/"+owner+"/"+name, ""); status {
	case http.StatusOK:
	case http.StatusNotFound:
		fail(fmt.Errorf("GitHub has no %s/%s: mise gets the tool's releases there, so make it first (empty, no files): gh repo create %s/%s --public", owner, name, owner, name))
	default:
		fail(fmt.Errorf("GitHub didn't say whether %s/%s is there (HTTP %d): a token that can read it is GITHUB_TOKEN, GH_TOKEN or gh auth login", owner, name, status))
	}
	files := map[string][]byte{}
	at := version
	switch {
	case from != "":
		files = checkout(from)
		at = ""
	case version == "":
		fail(fmt.Errorf("this mise-check is a checkout's build, not a release: new-tool copies its release, or with --from <folder> a checkout"))
	default:
		at = "v" + strings.TrimPrefix(at, "v") // a release's version is 0.4.22; Go's is v0.4.22
		files = fromProxy(at)
	}
	rename := strings.NewReplacer("joeblew999/mise-check", owner+"/"+name, "mise-check", name)
	for p, b := range files {
		if droppedFile(p) {
			continue
		}
		for i, k := range kept { // the imports of the packages stay mise-check's
			b = bytes.ReplaceAll(b, []byte(k), []byte(fmt.Sprintf("\x00%d\x00", i)))
		}
		b = []byte(rename.Replace(string(b)))
		for i, k := range kept {
			b = bytes.ReplaceAll(b, []byte(fmt.Sprintf("\x00%d\x00", i)), []byte(k))
		}
		if p == "README.md" {
			b = []byte(readmeStub(module, name, cmp.Or(at, "a checkout")))
		}
		to := filepath.Join(r.dir, filepath.FromSlash(rename.Replace(p)))
		must(os.MkdirAll(filepath.Dir(to), 0o755))
		must(os.WriteFile(to, b, 0o644))
	}
	fmt.Printf("✓ %s: mise-check %s copied and renamed, its packages (misecheck, walk, miseconfig, toolfiles, release) imported instead\n", module, cmp.Or(at, "(a checkout, "+from+")"))
	// Every mise file it wrote, trusted, as init trusts its own: the root's, and the template's.
	for p := range files {
		if p = rename.Replace(p); !droppedFile(p) && (path.Base(p) == "mise.toml" || path.Base(p) == "config.toml" && path.Base(path.Dir(p)) == ".mise") {
			trustConfig(r.dir, p)
		}
	}
	// The packages: at this release when this is mise-check (both pins at one release); the newest for a checkout, or
	// for a tool new-tool made, whose releases are its own.
	pkgs := "latest"
	if selfRepo == packagesModule && at != "" {
		pkgs = at
	}
	r.runOwnMise(r.dir, "exec", "--", "go", "get", packagesModule+"@"+pkgs)
	if from != "" && selfRepo == packagesModule { // a checkout of mise-check: its packages too, as they are there
		abs, err := filepath.Abs(from)
		must(err)
		r.runOwnMise(r.dir, "exec", "--", "go", "mod", "edit", "-replace", packagesModule+"="+abs)
	}
	r.runOwnMise(r.dir, "exec", "--", "go", "mod", "tidy")
	// mise-check itself, through mise, as every repo that imports its packages uses it (their misecheck.Check fails
	// without it): init's files at the new repo's top, the binary pinned at this release (local, a checkout's build),
	// beside go.mod's packages. A tool new-tool made has its own template, not mise-check's: it says what to run.
	if selfRepo == packagesModule {
		r.initRepo()
		trustConfig(r.dir, ".mise/config.toml")
		fmt.Printf("✓ mise-check's own tasks, through mise: mise run mise-check says what's done and what's next\n")
	} else {
		fmt.Printf("· its packages are mise-check's, used through mise: there, mise exec \"github:joeblew999/mise-check[minimum_release_age=0s]@latest\" -- mise-check init\n")
	}
	fmt.Printf("✓ next: mise install, then mise run root:ci (what its CI runs); README.md says what's next after that\n")
}

// readmeStub is a new tool repo's README: what it is, where it came from, and what to do next.
func readmeStub(module, name, from string) string {
	return "# " + name + `

**The problem.** Write here, in a paragraph, what's hard without ` + name + `, for a person or an agent who has never
seen it.

**What ` + name + ` does.** Write here, in a paragraph, how it solves that, through mise: one command adds it to a repo,
and from then on every step is a mise task.

A mise tool repo, made by ` + "`mise-check new-tool " + module + "`" + ` from mise-check ` + from + `: the ` + "`" + name + "`" + ` binary
(` + "`cmd/" + name + "/`" + `), its template (` + "`templates/`" + `), its walks, its release and its CI, as mise-check's are.

## Next

1. ` + "`mise install`, then `mise run root:ci`" + `: what its CI runs, here the same. It passes as it comes.
2. Its commands are mise-check's, renamed: keep those the tool needs. To remove one, delete its file,
   ` + "`cmd/" + name + "/<command>.go`" + `, and run ` + "`mise run root:ci`" + `: misecheck names its task, its skill row and its docs row
   still left.
3. Write the tool's own: a file per command in ` + "`cmd/" + name + "/`" + `, its row in ` + "`var commands`" + ` in ` + "`main.go`" + `, its task in
   ` + "`templates/.mise/tasks/" + name + ".toml`" + `, and its rows in the template's skill and docs. misecheck names each one missing.
4. Write this README's two openings above (misecheck's CheckToolRepo holds a tool repo's README to them), then the rest:
   how to use it, and how to work on it.

mise-check is used here as in any repo, through its own tasks, which new-tool wrote with ` + "`mise-check init`" + `:
` + "`mise run mise-check`" + ` (what's done, what's next), ` + "`mise-check:check`" + `, ` + "`mise-check:update`" + ` (every pin, mise-check's two at
one release). ` + "`docs/mise-check/README.md`" + ` has all of it. ` + "`mise tasks`" + ` lists every task here. CLAUDE.md loads this README.
`
}

// droppedFile reports whether the file p is one new-tool drops.
func droppedFile(p string) bool {
	for _, d := range dropped {
		if p == d || strings.HasSuffix(d, "/") && strings.HasPrefix(p, d) {
			return true
		}
	}
	return false
}

// fromProxy is mise-check's files at version, from Go's module proxy, as go get has them: its module, the files git
// holds, no more.
func fromProxy(version string) map[string][]byte {
	url := "https://proxy.golang.org/" + selfRepo + "/@v/" + version + ".zip"
	resp, err := http.Get(url)
	must(err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail(fmt.Errorf("the module proxy has no %s@%s (HTTP %d): a release is there a minute after it's out", selfRepo, version, resp.StatusCode))
	}
	b, err := io.ReadAll(resp.Body)
	must(err)
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	must(err)
	files := map[string][]byte{}
	prefix := selfRepo + "@" + version + "/"
	for _, f := range z.File {
		if f.FileInfo().IsDir() || !strings.HasPrefix(f.Name, prefix) {
			continue
		}
		rc, err := f.Open()
		must(err)
		files[strings.TrimPrefix(f.Name, prefix)], err = io.ReadAll(rc)
		must(err)
		rc.Close()
	}
	return files
}

// checkout is a checkout's files, as its git has them (committed or not, but not ignored): what its release would be.
func checkout(dir string) map[string][]byte {
	out, err := exec.Command("git", "-C", dir, "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		fail(fmt.Errorf("--from %s: not a git checkout of mise-check (%v)", dir, err))
	}
	files := map[string][]byte{}
	for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p))); err == nil {
			files[path.Clean(p)] = b
		}
	}
	return files
}
