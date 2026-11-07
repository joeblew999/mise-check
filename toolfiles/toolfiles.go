// Package toolfiles writes a mise tool's half of its template into a repo, as the tool's init does, and says what of it
// a repo lacks, as the tool's check does: every file of the half at its path in the repo, a .sh made runnable; and of
// the three files a repo may have of its own already, only the tool's parts, the rest left as it is: .mise/config.toml
// (miseconfig: its pin, its own name in minimum_release_age_excludes, use_versions_host), the template's .gitignore
// lines, and the Claude Code hook's SessionStart entry in .claude/settings.json. Every tool's init imports it,
// mise-check's among them, and a tool's init runs mise-check's own init too (MiseCheckInit), so the tool's users get
// mise-check's tasks themselves, never a tool's wrappers of them.
//
//	t := toolfiles.Tool{Files: templates.Tool, Hook: "mytool-session-start.sh", Config: miseconfig.Tool{…}}
//	err := t.Write(dir)        // init
//	problems := t.Problems(dir) // check, each naming t.Config.Init
package toolfiles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/joeblew999/mise-check/miseconfig"
)

// Tool is a mise tool as its init writes its files into a repo.
type Tool struct {
	Files  fs.FS           // the template's tool half, at the paths init writes them in a repo
	Hook   string          // the Claude Code hook's file name in .claude/hooks ("mise-check-session-start.sh"); "" for none
	Config miseconfig.Tool // the tool's parts of .mise/config.toml, and its init task (Config.Init), which each problem names
	Out    io.Writer       // where Write says what it wrote; os.Stdout when nil
}

// ownFiles are the files of a tool's half a repo may have of its own already: Write writes each whole only when the repo
// has none, and otherwise only the tool's parts of it.
var ownFiles = []string{".mise/config.toml", ".gitignore", ".claude/settings.json"}

// Write writes the tool's half into the repo at dir, each file at its path there, saying what it wrote: the repo's own
// .mise/config.toml, .gitignore and .claude/settings.json get only the tool's parts, and the rest of each stays.
func (t Tool) Write(dir string) error {
	return fs.WalkDir(t.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		to := filepath.Join(dir, filepath.FromSlash(p))
		if b, err := os.ReadFile(to); err == nil && theirs(p) {
			wrote, err := t.keepOwn(p, to, b)
			if err == nil && !wrote {
				fmt.Fprintf(t.out(), "✓ %s is the repo's own, with %s's parts as this release writes them\n", p, t.name())
			}
			return err
		}
		b, err := fs.ReadFile(t.Files, p)
		if err == nil && p == ".mise/config.toml" {
			b, err = t.Config.Pinned()
		}
		if err == nil {
			err = t.writeChanged(to, b)
		}
		if err == nil && strings.HasSuffix(p, ".sh") {
			err = os.Chmod(to, 0o755)
		}
		return err
	})
}

// Problems are what Write would write again in the repo at dir, each with what fixes it (t.Config.Init): a file of the
// half missing or out of date, a .claude/settings.json that doesn't run the hook, a .gitignore without the template's
// lines, and the tool's parts of .mise/config.toml (miseconfig). None, and the repo has the tool's files as this
// release writes them.
func (t Tool) Problems(dir string) []string {
	fix := ": run " + t.Config.Init
	var out []string
	fs.WalkDir(t.Files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || theirs(p) {
			return err
		}
		want, _ := fs.ReadFile(t.Files, p)
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		switch {
		case err != nil:
			out = append(out, p+" is missing"+fix)
		case !bytes.Equal(lf(b), lf(want)): // a Windows checkout may have \r\n where init wrote \n
			out = append(out, p+" is out of date"+fix)
		}
		return nil
	})
	if b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json")); err == nil && t.Hook != "" && !bytes.Contains(b, []byte(t.Hook)) {
		out = append(out, ".claude/settings.json doesn't run .claude/hooks/"+t.Hook+fix)
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".gitignore")); err == nil {
		if missing := t.ignoresMissing(b); len(missing) > 0 {
			out = append(out, ".gitignore lacks "+t.name()+"'s lines ("+strings.Join(missing, "; ")+")"+fix)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".mise", "config.toml")); err != nil {
		out = append(out, ".mise/config.toml is missing: it pins "+t.Config.Name+fix)
	} else {
		out = append(out, t.Config.Problems(b)...)
	}
	return out
}

// Trust has mise trust a config file in dir (".mise/config.toml", "mise.toml") that a tool wrote, so its tasks run
// straight away, and says so; when mise can't, it says what to run.
func Trust(dir, file string, out io.Writer) {
	if out == nil {
		out = os.Stdout
	}
	path := filepath.Join(dir, filepath.FromSlash(file))
	if _, err := os.Stat(path); err != nil {
		return
	}
	if _, err := exec.LookPath("mise"); err != nil {
		return
	}
	if b, err := exec.Command("mise", "trust", "-q", path).CombinedOutput(); err != nil {
		fmt.Fprintf(out, "mise didn't trust %s (%v %s): run mise trust %s\n", file, err, strings.TrimSpace(string(b)), file)
		return
	}
	fmt.Fprintf(out, "✓ %s trusted by mise\n", file)
}

// miseCheck is mise-check, as mise names it and as a go.mod requires its packages.
const (
	miseCheckTool   = "github:joeblew999/mise-check"
	miseCheckModule = "github.com/joeblew999/mise-check"
)

// MiseCheckInit runs mise-check's own init in dir, the repo's top, so a tool's users get mise-check's own tasks, its
// workflow, skill and docs, never a tool's wrappers of them: through mise exec, at the release of mise-check the tool is
// built with (the one its go.mod requires, so the packages it imports and the binary it sets up are one release), which
// mise gets at once as an exact release; at the newest for a build with none (a checkout's, its packages replaced).
func MiseCheckInit(dir string) error {
	cmd := exec.Command("mise", "exec", miseCheckTool+miseCheckAt(), "--", "mise-check", "init")
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
	}
	return nil
}

// miseCheckAt is the release of mise-check this binary is built with, as mise exec takes it ("@0.4.33"), or its newest
// release for a build with none.
func miseCheckAt() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		mods := append([]*debug.Module{&bi.Main}, bi.Deps...)
		for _, m := range mods {
			if m.Path == miseCheckModule && m.Replace == nil && strings.HasPrefix(m.Version, "v") && !strings.Contains(m.Version, "-") {
				return "@" + strings.TrimPrefix(m.Version, "v")
			}
		}
	}
	return "[minimum_release_age=0s]@latest"
}

// theirs reports whether the half's file p is one a repo may have of its own.
func theirs(p string) bool {
	for _, f := range ownFiles {
		if p == f {
			return true
		}
	}
	return false
}

// name is the tool's short name, as it says what it wrote: mise-check.
func (t Tool) name() string { return t.Config.Name[strings.LastIndexAny(t.Config.Name, ":/")+1:] }

func (t Tool) out() io.Writer {
	if t.Out == nil {
		return os.Stdout
	}
	return t.Out
}

// keepOwn brings the tool's parts of one of the repo's own files (ownFiles), at to, its text b, to what this release
// writes, and leaves the rest of it as it is, saying what it wrote; it reports whether it did.
func (t Tool) keepOwn(p, to string, b []byte) (bool, error) {
	switch p {
	case ".mise/config.toml":
		out, wrote := t.Config.Keep(b)
		if len(wrote) == 0 {
			return false, nil
		}
		for _, w := range wrote {
			fmt.Fprintf(t.out(), "wrote %s: %s\n", to, w)
		}
		return true, os.WriteFile(to, out, 0o644)
	case ".gitignore":
		return t.addIgnores(to, b)
	case ".claude/settings.json":
		return t.addHook(to, b)
	}
	return false, nil
}

// ignoresMissing are the template's .gitignore lines that a repo's own .gitignore (b) lacks, each with the template's
// comment just above it.
func (t Tool) ignoresMissing(b []byte) []string {
	want, _ := fs.ReadFile(t.Files, ".gitignore")
	have := map[string]bool{}
	for _, l := range strings.Split(string(lf(b)), "\n") {
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
func (t Tool) addIgnores(path string, b []byte) (bool, error) {
	missing := t.ignoresMissing(b)
	if len(missing) == 0 {
		return false, nil
	}
	s := strings.TrimRight(string(b), "\n") + "\n" + strings.Join(missing, "\n") + "\n"
	fmt.Fprintf(t.out(), "wrote %s: %s's lines it lacked, %d\n", path, t.name(), len(missing))
	return true, os.WriteFile(path, []byte(s), 0o644)
}

// addHook adds the hook's SessionStart entry, as the template's .claude/settings.json has it, to the repo's own (b)
// when it doesn't run the hook, keeping every other setting and hook. One it can't read as JSON is left as it is, and
// Problems goes on saying so.
func (t Tool) addHook(path string, b []byte) (bool, error) {
	if t.Hook == "" || bytes.Contains(b, []byte(t.Hook)) {
		return false, nil
	}
	var own, want map[string]any
	tb, _ := fs.ReadFile(t.Files, ".claude/settings.json")
	if json.Unmarshal(b, &own) != nil || json.Unmarshal(tb, &want) != nil {
		fmt.Fprintf(t.out(), "· %s isn't JSON %s can read: add the template's SessionStart entry, .claude/hooks/%s, to it\n", path, t.name(), t.Hook)
		return false, nil
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
	if err != nil {
		return false, err
	}
	fmt.Fprintf(t.out(), "wrote %s: the hook's SessionStart entry, .claude/hooks/%s\n", path, t.Hook)
	return true, os.WriteFile(path, append(out, '\n'), 0o644)
}

// writeChanged writes b to path, saying so, unless it's there already as it is.
func (t Tool) writeChanged(path string, b []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		fmt.Fprintf(t.out(), "✓ %s is up to date\n", path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fmt.Fprintln(t.out(), "wrote", path)
	return os.WriteFile(path, b, 0o644)
}

// lf is b with \r\n as \n.
func lf(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
