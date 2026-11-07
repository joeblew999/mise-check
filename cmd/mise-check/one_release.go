package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// selfModule is mise-check as a repo's go.mod requires it: its packages, misecheck and walk.
const selfModule = "github.com/joeblew999/mise-check"

// selves is mise-check as the repo pins it twice: the binary in a mise file, and the packages in its go.mods. They're
// one release (the binary and the packages are one version, released from main), so a repo pins them alike.
type selves struct {
	mise     string   // the binary's version, as mise reads the pin ("" for none, or a checkout's own build)
	latest   bool     // the pin is latest, not a version: mise upgrade moves what it is, never mise use
	miseFile string   // the mise file that pins it
	mods     []string // the folders of the go.mods that require the packages
	module   string   // their version, "v0.4.20" (the newest, if they differ)
}

// selves reads both pins of mise-check from the repo's own mise and go.mods.
func (r repo) selves(mods []string) selves {
	var s selves
	var tools map[string][]struct {
		Version          string
		RequestedVersion string `json:"requested_version"`
		Source           struct{ Path string }
	}
	if json.Unmarshal([]byte(r.mise("ls", "--no-global", "--json")), &tools) == nil {
		for _, v := range tools[self] {
			if v.Source.Path != "" && v.RequestedVersion != "local" && release("v"+v.Version) != nil {
				s.mise, s.miseFile, s.latest = v.Version, v.Source.Path, v.RequestedVersion == "latest"
			}
		}
	}
	for _, m := range mods {
		var f struct {
			Require []struct{ Path, Version string }
		}
		if json.Unmarshal([]byte(r.miseIn(m, "exec", "--", "go", "mod", "edit", "-json")), &f) != nil {
			continue
		}
		for _, q := range f.Require {
			if q.Path == selfModule && release(q.Version) != nil {
				s.mods = append(s.mods, m)
				if s.module == "" || newer(q.Version, s.module) {
					s.module = q.Version
				}
			}
		}
	}
	return s
}

// release is a version's numbers, v0.4.20 → [0 4 20]; nil for anything else (a pseudo-version, a prerelease).
func release(v string) []int {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if !strings.HasPrefix(v, "v") || len(parts) != 3 {
		return nil
	}
	var n []int
	for _, p := range parts {
		i, err := strconv.Atoi(p)
		if err != nil || i < 0 {
			return nil
		}
		n = append(n, i)
	}
	return n
}

// newer reports whether release a is after release b.
func newer(a, b string) bool {
	x, y := release(a), release(b)
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

// apart is the problem with the repo's two pins of mise-check, "" when they agree or it pins only one.
func (s selves) apart() string {
	if s.mise == "" || s.module == "" || "v"+s.mise == s.module {
		return ""
	}
	return fmt.Sprintf("mise-check is one release, and the repo pins two: %s in %s, %s in go.mod (%s); mise run mise-check:update moves both to the newer",
		s.mise, filepath.Base(s.miseFile), s.module, selfModule)
}

// together moves the older of the two pins to the newer: the binary, with mise use, in the file that pins it; the
// packages, with go get, in each go.mod. mise takes a release a day after it's out, Go at once, so after an update the
// packages are usually the newer.
func (r repo) together(s selves) {
	if s.apart() == "" {
		return
	}
	switch {
	case newer(s.module, "v"+s.mise) && s.latest: // latest stays latest: the newest release, installed
		r.runOwnMise(r.dir, "upgrade", self)
		return
	case newer(s.module, "v"+s.mise):
		r.runOwnMise(r.dir, "use", "--path", s.miseFile, self+"@"+strings.TrimPrefix(s.module, "v"))
		return
	}
	for _, m := range s.mods {
		r.runOwnMise(m, "exec", "--", "go", "get", selfModule+"@v"+s.mise)
		r.runOwnMise(m, "exec", "--", "go", "mod", "tidy")
	}
}
