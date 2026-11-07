// Package miseconfig keeps a mise tool's parts of a repo's own .mise/config.toml as the tool's init writes them, and
// leaves the rest of the file as it is: the tool's pin (added, or moved forward to this release; local and latest
// stay), its own name in minimum_release_age_excludes (each tool adds only its own, beside the repo's entries and
// other tools'), use_versions_host = false, and the comment at the file's top when it's the tool's. Every tool's init
// imports it, mise-check's among them, so a repo with several tools gets each one's parts from each one's init, and no
// list of tools is kept anywhere. Each part, with its comment, is the tool's template .mise/config.toml's, the file init
// writes whole into a repo with none (Pinned).
//
//	t := miseconfig.Tool{Name: "github:joeblew999/mise-check", Release: "0.4.32", Template: tmpl, Init: "mise run mise-check:init"}
//	out, wrote := t.Keep(b)   // the repo's own file, its text b, with the tool's parts as this release writes them
//	problems := t.Problems(b) // what the tool's check says of it, each naming t.Init
package miseconfig

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Tool is a mise tool as its init keeps its parts of a repo's .mise/config.toml.
type Tool struct {
	Name     string // the tool as mise names it: "github:joeblew999/mise-check"
	Release  string // this build's release, "0.4.32"; "" for a checkout's build, which pins local
	Template []byte // the tool's template .mise/config.toml: its comment at the top, its settings, its pin
	Init     string // what writes the tool's files again, which each problem names: "mise run mise-check:init"
}

// Pinned is the template pinning this release (local for a checkout's build): what init writes into a repo with no
// .mise/config.toml of its own. It fails when the template doesn't pin the tool.
func (t Tool) Pinned() ([]byte, error) {
	if !t.pinLine().Match(t.Template) {
		return nil, fmt.Errorf("the template's .mise/config.toml doesn't pin %s", t.Name)
	}
	return t.pinLine().ReplaceAll(t.Template, []byte(t.pinAt(cmp.Or(t.Release, "local")))), nil
}

// Keep is the repo's own .mise/config.toml, its text b, with the tool's parts as this release writes them, and what it
// wrote, one line each (none, and b was so already). The rest of the file stays as it is.
func (t Tool) Keep(b []byte) ([]byte, []string) {
	s := string(b)
	var wrote []string
	for _, keep := range []func(string) (string, string){t.addPin, t.movePin, t.keepHeader, t.addExclude, t.addVersionsHost} {
		var did string
		if s, did = keep(s); did != "" {
			wrote = append(wrote, did)
		}
	}
	return []byte(s), wrote
}

// Problems are the tool's parts the repo's own .mise/config.toml (its text b) lacks, or has as an older release wrote
// them, each with what fixes it (t.Init): what the tool's check reports.
func (t Tool) Problems(b []byte) []string {
	s, fix := string(b), ": run "+t.Init
	var out []string
	if !t.pinLine().MatchString(s) {
		out = append(out, ".mise/config.toml doesn't pin "+t.Name+fix)
	}
	if have := header(s); have != header(string(t.Template)) && strings.HasPrefix(have, t.mark()) {
		out = append(out, ".mise/config.toml's comment at its top is an older "+t.short()+"'s"+fix)
	}
	if !slices.Contains(Excludes(b), t.Name) {
		out = append(out, ".mise/config.toml doesn't let mise take "+t.short()+" at its newest release the minute it's out ("+t.Name+" not in minimum_release_age_excludes)"+fix)
	}
	if !versionsHostLine.MatchString(s) {
		out = append(out, ".mise/config.toml doesn't set use_versions_host, so latest waits for mise's versions host to list a release"+fix)
	}
	return out
}

// Excludes are the tools a mise file's text b lists in minimum_release_age_excludes: the ones mise takes the minute
// they're out, never held back a day.
func Excludes(b []byte) []string {
	m := excludesLine.FindSubmatch(b)
	if m == nil {
		return nil
	}
	var out []string
	for _, q := range quoted.FindAllSubmatch(m[1], -1) {
		out = append(out, string(q[1]))
	}
	return out
}

var (
	excludesLine     = regexp.MustCompile(`(?m)^minimum_release_age_excludes\s*=\s*\[([^\]]*)\]`)
	versionsHostLine = regexp.MustCompile(`(?m)^use_versions_host\s*=`)
	quoted           = regexp.MustCompile(`"([^"]+)"`)
)

// legacyComment is how the comment on minimum_release_age_excludes started before this package, in every tool that
// wrote it: one that starts so, or none at all, gets the template's. Any other is the repo's, or another tool's, and
// stays.
const legacyComment = "# The owner's tools"

// short is the tool's name without its backend and owner: mise-check.
func (t Tool) short() string { return t.Name[strings.LastIndexAny(t.Name, ":/")+1:] }

// mark is how the comment at the top of the tool's .mise/config.toml starts ("# mise-check "): a file whose comment
// starts so is the tool's, kept as this release writes it; another tool's stays as it is.
func (t Tool) mark() string { return "# " + t.short() + " " }

// pinLine is the tool's pin in a mise file.
func (t Tool) pinLine() *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(`"`+t.Name+`" = "`) + `[^"]*"`)
}

// pinAt is the tool's pin line at version v.
func (t Tool) pinAt(v string) string { return `"` + t.Name + `" = "` + v + `"` }

// addPin adds the tool's pin when the file has none, as another tool's init wrote the file: under its [tools], or in a
// [tools] of its own at the end.
func (t Tool) addPin(s string) (string, string) {
	if t.pinLine().MatchString(s) {
		return s, ""
	}
	line := t.pinAt(cmp.Or(t.Release, "local"))
	if i := strings.Index(s, "[tools]\n"); i >= 0 {
		i += len("[tools]\n")
		return s[:i] + line + "\n" + s[i:], t.short() + "'s pin added, " + line
	}
	return strings.TrimRight(s, "\n") + "\n\n[tools]\n" + line + "\n", t.short() + "'s pin added, " + line
}

// movePin moves the pin forward to this release when it pins an older one: init at a newer release, run by hand, is
// the move to it. A pin of local or latest, or of this release or a newer one, stays, as does every pin for a
// checkout's build.
func (t Tool) movePin(s string) (string, string) {
	m := t.pinLine().FindString(s)
	at := strings.TrimSuffix(strings.TrimPrefix(m, `"`+t.Name+`" = "`), `"`)
	if t.Release == "" || m == "" || !older(at, t.Release) {
		return s, ""
	}
	return strings.Replace(s, m, t.pinAt(t.Release), 1), t.short() + " " + at + " → " + t.Release
}

// keepHeader writes the comment at the file's top as the template has it, when the file's is the tool's from an older
// release (one naming a task there isn't any more, say).
func (t Tool) keepHeader(s string) (string, string) {
	have, want := header(s), header(string(t.Template))
	if have == want || !strings.HasPrefix(have, t.mark()) {
		return s, ""
	}
	return want + s[len(have):], t.short() + "'s comment at its top, as this release writes it"
}

// addExclude adds the tool's own name to minimum_release_age_excludes: in its list, beside the entries there, or a
// line of its own, with the template's comment, under [settings]. A comment on the list of an older tool's
// (legacyComment), or none, gets the template's.
func (t Tool) addExclude(s string) (string, string) {
	tmpl := string(t.Template)
	comment := ""
	if tm := excludesLine.FindStringIndex(tmpl); tm != nil {
		comment = tmpl[commentAbove(tmpl, tm[0]):tm[0]]
	}
	m := excludesLine.FindStringSubmatchIndex(s)
	if m == nil {
		s, i := settings(s)
		return s[:i] + comment + `minimum_release_age_excludes = ["` + t.Name + "\"]\n" + s[i:], t.Name + " in minimum_release_age_excludes, so latest is its newest release the minute it's out"
	}
	above := commentAbove(s, m[0])
	stale := s[above:m[0]] != comment && (above == m[0] || strings.HasPrefix(s[above:m[0]], legacyComment))
	listed := slices.Contains(Excludes([]byte(s)), t.Name)
	if listed && !stale {
		return s, ""
	}
	list, did := s[m[2]:m[3]], "the comment on minimum_release_age_excludes, as this release writes it"
	if !listed {
		list = `"` + t.Name + `"`
		if have := strings.TrimRight(strings.TrimSpace(s[m[2]:m[3]]), ","); have != "" {
			list = have + ", " + list
		}
		did = t.Name + " in minimum_release_age_excludes, so latest is its newest release the minute it's out"
	}
	if !stale {
		above, comment = m[0], ""
	}
	return s[:above] + comment + s[m[0]:m[2]] + list + s[m[3]:], did
}

// addVersionsHost adds the template's use_versions_host = false, with its comment, under [settings] when the file
// doesn't set use_versions_host at all: latest from GitHub's own list of releases. A value of the repo's own stays.
func (t Tool) addVersionsHost(s string) (string, string) {
	tmpl := string(t.Template)
	tm := versionsHostLine.FindStringIndex(tmpl)
	if versionsHostLine.MatchString(s) || tm == nil {
		return s, ""
	}
	s, i := settings(s)
	if e := excludesLine.FindStringIndex(s); e != nil && e[0] >= i { // after the excludes, as the template has it
		i = lineEnd(s, e[1])
	}
	return s[:i] + tmpl[commentAbove(tmpl, tm[0]):lineEnd(tmpl, tm[0])] + s[i:], "use_versions_host = false, so latest is the newest release on GitHub the minute it's out"
}

// settings is the file's text s with a [settings] (one of its own before [tools], or at the end, when it has none),
// and where in it a line under [settings] goes.
func settings(s string) (string, int) {
	if i := strings.Index(s, "[settings]\n"); i >= 0 {
		return s, i + len("[settings]\n")
	}
	if i := strings.Index(s, "[tools]\n"); i >= 0 {
		return s[:i] + "[settings]\n\n" + s[i:], i + len("[settings]\n")
	}
	if s = strings.TrimRight(s, "\n"); s != "" {
		s += "\n\n"
	}
	s += "[settings]\n"
	return s, len(s)
}

// header is the comment at the top of a file's text s: its lines from the first that start with #.
func header(s string) string {
	end := 0
	for end < len(s) && strings.HasPrefix(s[end:], "#") {
		end = lineEnd(s, end)
	}
	return s[:end]
}

// commentAbove is where the comment just above the line at in s starts: its lines that start with #, at for none.
func commentAbove(s string, at int) int {
	for at > 0 {
		prev := strings.LastIndexByte(s[:at-1], '\n') + 1
		if !strings.HasPrefix(s[prev:at], "#") {
			break
		}
		at = prev
	}
	return at
}

// lineEnd is where the line with i in s ends, after its newline.
func lineEnd(s string, i int) int {
	if nl := strings.IndexByte(s[i:], '\n'); nl >= 0 {
		return i + nl + 1
	}
	return len(s)
}

// older reports whether release a is older than release b, both x.y.z; false for anything else (local, latest).
func older(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	if len(pa) != 3 || len(pb) != 3 {
		return false
	}
	for i := range pa {
		x, errx := strconv.Atoi(pa[i])
		y, erry := strconv.Atoi(pb[i])
		if errx != nil || erry != nil {
			return false
		}
		if x != y {
			return x < y
		}
	}
	return false
}
