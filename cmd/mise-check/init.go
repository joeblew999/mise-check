package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joeblew999/mise-check/templates"
)

// theirs are the template's files a repo may have of its own already: init writes them only when it has none.
// .mise/config.toml is mise-check's pin: written once, then the repo's (mise-check:update moves it, and so does a
// newer release's init, run by hand: forward only).
var theirs = map[string]bool{".gitignore": true, ".claude/settings.json": true, ".mise/config.toml": true}

// pinLine is mise-check's pin in .mise/config.toml.
var pinLine = regexp.MustCompile(regexp.QuoteMeta(`"`+self+`" = "`) + `[^"]*"`)

// pinned is the template's .mise/config.toml pinning this build: a release pins itself, a build of a checkout local
// (mise run tool:install there links it), so a repo runs exactly what set it up.
func pinned(b []byte) []byte {
	pin := "local"
	if version != "" {
		pin = strings.TrimPrefix(version, "v")
	}
	if !pinLine.Match(b) {
		fail(fmt.Errorf("the template's .mise/config.toml doesn't pin %s", self))
	}
	return pinLine.ReplaceAll(b, []byte(`"`+self+`" = "`+pin+`"`))
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
		if err == nil {
			if p == ".mise/config.toml" {
				b = pinned(b)
			}
			writeFile(to, b)
		}
		if strings.HasSuffix(p, ".sh") {
			must(os.Chmod(to, 0o755))
		}
		return err
	}))
}

// addPin adds mise-check's pin to the repo's own .mise/config.toml (b) when it has none, as one another tool's init
// wrote (mise-cloudflare's): under its [tools], or in a [tools] of its own at the end. The rest stays as it is.
func addPin(path string, b []byte) bool {
	if pinLine.Match(b) {
		return false
	}
	line := pinned([]byte(`"` + self + `" = "latest"`))
	s := string(b)
	if i := strings.Index(s, "[tools]\n"); i >= 0 {
		s = s[:i+len("[tools]\n")] + string(line) + "\n" + s[i+len("[tools]\n"):]
	} else {
		s = strings.TrimRight(s, "\n") + "\n\n[tools]\n" + string(line) + "\n"
	}
	must(os.WriteFile(path, []byte(s), 0o644))
	fmt.Printf("wrote %s: mise-check's pin added, %s\n", path, line)
	return true
}

// keepOwn brings mise-check's parts of one of the repo's own files (theirs), at to, its text b, to what this version
// writes, and leaves the rest of it as it is: in .mise/config.toml its pin (added, or moved forward), mise-check's
// comment at its top, and the owner's tools in minimum_release_age_excludes; in .gitignore the template's lines it
// lacks; in .claude/settings.json the hook's SessionStart entry. It says what it wrote, and reports whether it did.
func keepOwn(p, to string, b []byte) bool {
	switch p {
	case ".mise/config.toml":
		wrote := false
		for _, keep := range []func(string, []byte) bool{addPin, movePin, keepHeader, addExcludes, addVersionsHost} {
			if keep(to, b) {
				wrote = true
				b, _ = os.ReadFile(to)
			}
		}
		return wrote
	case ".gitignore":
		return addIgnores(to, b)
	case ".claude/settings.json":
		return addHook(to, b)
	}
	return false
}

// templateConfig is the template's .mise/config.toml, which says, once, the owner's tools (excludes) and what the file
// is (its header).
var templateConfig, _ = fs.ReadFile(templates.Tool, ".mise/config.toml")

// excludes are the owner's tools, which mise takes at their newest release the moment it's out (as latest, and for
// mise-check:update): the template's minimum_release_age_excludes, read from it. mise holds back a release younger than
// a day for every other tool, and matches no owner's tools by a pattern, so each is named there.
var excludes = excludesIn(templateConfig)

// excludesIn are the tools a mise file's text b lists in minimum_release_age_excludes.
func excludesIn(b []byte) []string {
	m := excludesLine.FindSubmatch(b)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range regexp.MustCompile(`"([^"]+)"`).FindAllSubmatch(m[1], -1) {
		out = append(out, string(q[1]))
	}
	return out
}

// configHeader is how mise-check's comment at the top of .mise/config.toml starts: a file whose comment starts so is
// mise-check's, kept as this version writes it; another tool's (mise-cloudflare's init writes its own) is left alone.
const configHeader = "# mise-check for this repo"

// header is the comment at the top of a file's text s: its lines from the first that start with #.
func header(s string) string {
	end := 0
	for end < len(s) && strings.HasPrefix(s[end:], "#") {
		nl := strings.IndexByte(s[end:], '\n')
		if nl < 0 {
			return s
		}
		end += nl + 1
	}
	return s[:end]
}

// keepHeader writes mise-check's comment at the top of the repo's .mise/config.toml (b) as this version has it, when
// the file's is mise-check's from an older one: one naming a task there isn't any more, say. The rest stays as it is.
func keepHeader(path string, b []byte) bool {
	s, want := string(b), header(string(templateConfig))
	have := header(s)
	if have == want || !strings.HasPrefix(have, configHeader) {
		return false
	}
	must(os.WriteFile(path, []byte(want+s[len(have):]), 0o644))
	fmt.Printf("wrote %s: mise-check's comment at its top, as this version writes it\n", path)
	return true
}

// ownersComment is how mise-check's comment on minimum_release_age_excludes starts, in this version and the ones
// before: one that starts so is mise-check's, written again as this version has it.
const ownersComment = "# The owner's tools"

// excludesComment is the template's comment on minimum_release_age_excludes, the lines just above it, which a repo's
// own .mise/config.toml gets with it.
func excludesComment() string {
	s := string(templateConfig)
	m := excludesLine.FindStringIndex(s)
	if m == nil {
		return ""
	}
	start := m[0]
	for {
		prev := strings.LastIndexByte(s[:max(start-1, 0)], '\n') + 1
		if start == 0 || !strings.HasPrefix(s[prev:start], "#") {
			return s[start:m[0]]
		}
		start = prev
	}
}

// versionsHostLine is use_versions_host in a mise file.
var versionsHostLine = regexp.MustCompile(`(?m)^use_versions_host\s*=`)

// addVersionsHost adds the template's use_versions_host = false, and its comment, under the [settings] of the repo's own
// .mise/config.toml (b) when it doesn't set use_versions_host at all: latest from GitHub's own list of releases. A
// value of the repo's own stays.
func addVersionsHost(path string, b []byte) bool {
	s := string(b)
	if versionsHostLine.MatchString(s) || !strings.Contains(s, "[settings]\n") {
		return false
	}
	t := string(templateConfig)
	m := versionsHostLine.FindStringIndex(t)
	end := m[0] + strings.IndexByte(t[m[0]:], '\n') + 1
	start := m[0]
	for start > 0 {
		prev := strings.LastIndexByte(t[:start-1], '\n') + 1
		if !strings.HasPrefix(t[prev:start], "#") {
			break
		}
		start = prev
	}
	i := strings.Index(s, "[settings]\n") + len("[settings]\n")
	if e := excludesLine.FindStringIndex(s); e != nil && e[0] >= i { // after the excludes, as the template has it
		i = e[1] + 1
	}
	must(os.WriteFile(path, []byte(s[:i]+t[start:end]+s[i:]), 0o644))
	fmt.Printf("wrote %s: use_versions_host = false, so latest is the newest release on GitHub the minute it's out\n", path)
	return true
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

// excludesLine is minimum_release_age_excludes in a mise file, its list in the first group.
var excludesLine = regexp.MustCompile(`(?m)^minimum_release_age_excludes\s*=\s*\[([^\]]*)\]`)

// excludesMissing are the owner's tools the mise file's text b doesn't exclude from minimum_release_age.
func excludesMissing(b []byte) []string {
	have := ""
	if m := excludesLine.FindSubmatch(b); m != nil {
		have = string(m[1])
	}
	var missing []string
	for _, e := range excludes {
		if !strings.Contains(have, `"`+e+`"`) {
			missing = append(missing, e)
		}
	}
	return missing
}

// addExcludes adds the owner's tools to minimum_release_age_excludes in the repo's own .mise/config.toml (b), the
// ones it doesn't have: in its list, or a line of its own under its [settings], or a [settings] of its own before
// [tools]. The rest of the file stays as it is.
func addExcludes(path string, b []byte) bool {
	missing := excludesMissing(b)
	s := string(b)
	m := excludesLine.FindStringSubmatchIndex(s)
	// The comment above the line: none (an older init wrote it bare), or an older mise-check's, gets the template's;
	// the repo's own comment stays.
	above, comment := 0, excludesComment()
	if m != nil {
		above = m[0]
		for above > 0 {
			prev := strings.LastIndexByte(s[:above-1], '\n') + 1
			if !strings.HasPrefix(s[prev:above], "#") {
				break
			}
			above = prev
		}
	}
	ours := m != nil && s[above:m[0]] != comment && (above == m[0] || strings.HasPrefix(s[above:m[0]], ownersComment))
	if len(missing) == 0 && !ours {
		return false
	}
	quoted := `"` + strings.Join(missing, `", "`) + `"`
	switch {
	case m != nil:
		have := strings.TrimRight(strings.TrimSpace(s[m[2]:m[3]]), ",")
		if have != "" && len(missing) > 0 {
			quoted = have + ", " + quoted
		} else if have != "" {
			quoted = have
		}
		if !ours {
			above, comment = m[0], ""
		}
		s = s[:above] + comment + s[m[0]:m[2]] + quoted + s[m[3]:]
	case strings.Contains(s, "[settings]\n"):
		i := strings.Index(s, "[settings]\n") + len("[settings]\n")
		s = s[:i] + excludesComment() + "minimum_release_age_excludes = [" + quoted + "]\n" + s[i:]
	case strings.Contains(s, "[tools]\n"):
		i := strings.Index(s, "[tools]\n")
		s = s[:i] + "[settings]\n" + excludesComment() + "minimum_release_age_excludes = [" + quoted + "]\n\n" + s[i:]
	default:
		s = strings.TrimRight(s, "\n") + "\n\n[settings]\n" + excludesComment() + "minimum_release_age_excludes = [" + quoted + "]\n"
	}
	must(os.WriteFile(path, []byte(s), 0o644))
	fmt.Printf("wrote %s: the owner's tools at their newest release, as latest (minimum_release_age_excludes), with the template's comment: %s\n", path, cmp.Or(strings.Join(missing, ", "), "none missing"))
	return true
}

// movePin moves the repo's pin of mise-check in its .mise/config.toml (b) forward to this release, when it pins an
// older one: init at a newer release, run by hand, is the move to it. The rest of the file stays as it is; a pin of
// local, or of this release or a newer one, stays too, as does every pin when this is a checkout's build.
func movePin(path string, b []byte) bool {
	m := pinLine.Find(b)
	at := strings.TrimSuffix(strings.TrimPrefix(string(m), `"`+self+`" = "`), `"`)
	if version == "" || m == nil || at == "local" || !older(at, buildVersion()) {
		return false
	}
	must(os.WriteFile(path, pinLine.ReplaceAll(b, []byte(`"`+self+`" = "`+buildVersion()+`"`)), 0o644))
	fmt.Printf("wrote %s: mise-check %s → %s\n", path, at, buildVersion())
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

// sayNewest says, of a release, when GitHub has a newer one, and how to set the repo up at it: mise takes a release as
// latest only once it's a day old, so @latest can run an older one than the newest.
func sayNewest() {
	if version == "" {
		return
	}
	switch n := newestRelease(); {
	case n == "":
		fmt.Printf("· this is mise-check %s; GitHub didn't say what its newest release is\n", buildVersion())
	case older(buildVersion(), n):
		fmt.Printf("· this is mise-check %s, and the newest release is %s: mise takes a release as latest only once it's a day old. For it, which moves the repo's pin to it:\n  mise exec %s@%s -- mise-check init\n", buildVersion(), n, self, n)
	default:
		fmt.Printf("✓ mise-check %s, the newest release\n", buildVersion())
	}
}

// stale names the files init wrote that this version would write differently, and the repo's own files whose
// mise-check parts init would write again (keepOwn): an older comment at the top of .mise/config.toml, a .gitignore
// without the template's lines, a .claude/settings.json that doesn't run the hook.
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
	if b, err := os.ReadFile(filepath.Join(r.dir, ".mise", "config.toml")); err == nil {
		if have := header(string(b)); strings.HasPrefix(have, configHeader) && have != header(string(templateConfig)) {
			out = append(out, ".mise/config.toml's comment at its top is an older mise-check's: run mise run mise-check:init")
		}
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
