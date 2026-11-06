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

// selfRepo is mise-check's own module: what new-tool copies, at this binary's release.
const selfRepo = "github.com/joeblew999/mise-check"

// dropped are what a new tool repo imports from mise-check instead of copying: the two packages and their tests.
var dropped = []string{"misecheck.go", "misecheck_test.go", "walk/"}

// kept are the imports of mise-check that stay mise-check's in a new tool repo: the packages it imports.
var kept = []string{`misecheck "` + selfRepo + `"`, `"` + selfRepo + `/walk"`}

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
	files := map[string][]byte{}
	at := version
	switch {
	case from != "":
		files = checkout(from)
		at = ""
	case version == "":
		fail(fmt.Errorf("this mise-check is a checkout's build, not a release: new-tool copies its release, or with --from <folder> a checkout"))
	default:
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
		to := filepath.Join(r.dir, filepath.FromSlash(rename.Replace(p)))
		must(os.MkdirAll(filepath.Dir(to), 0o755))
		must(os.WriteFile(to, b, 0o644))
	}
	fmt.Printf("✓ %s: mise-check %s copied and renamed, its packages (misecheck, walk) imported instead\n", module, cmp.Or(at, "(a checkout, "+from+")"))
	trustConfig(r.dir, "mise.toml")
	// The packages, at this release (both pins at one release), or the newest, for a checkout.
	r.runOwnMise(r.dir, "exec", "--", "go", "get", selfRepo+"@"+cmp.Or(at, "latest"))
	r.runOwnMise(r.dir, "exec", "--", "go", "mod", "tidy")
	fmt.Printf("✓ next: mise install, then mise run root:verify (what its CI runs); the tool's commands are in cmd/%s\n", name)
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
