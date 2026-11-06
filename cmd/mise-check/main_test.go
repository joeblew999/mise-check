package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestPin: the go-mod-upgrade update-go runs is the one mise-check's root mise.toml pins ("go:<module>" = "<version>"),
// so moving it there (mise run root:update) moves what every repo's update-go runs. When they differ, it says what to
// change. The pin isn't a tool line of go.mod: that would raise mise-check's go line, and every importer's with it.
func TestPin(t *testing.T) {
	b, err := os.ReadFile("../../mise.toml")
	if err != nil {
		t.Fatal(err)
	}
	path, want, _ := strings.Cut(goModUpgrade, "@")
	m := regexp.MustCompile(`"go:` + regexp.QuoteMeta(path) + `"\s*=\s*"([^"]*)"`).FindSubmatch(b)
	switch {
	case m == nil:
		t.Fatalf("mise.toml doesn't pin \"go:%s\": it's the pin of update.go's goModUpgrade", path)
	case "v"+string(m[1]) != want:
		t.Fatalf("mise.toml pins go:%s %s, and update.go's goModUpgrade is @%s: make it %s@v%s", path, m[1], want, path, m[1])
	}
}
