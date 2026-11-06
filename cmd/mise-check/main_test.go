package main

import (
	"os"
	"strings"
	"testing"
)

// TestPin: the go-mod-upgrade update runs is the one mise-check's go.mod pins on its tool line, so moving the module
// (mise run root:update) moves what every repo's update runs. When they differ, it says what to change.
func TestPin(t *testing.T) {
	b, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	path, want, _ := strings.Cut(goModUpgrade, "@")
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "require ")); len(f) >= 2 && f[0] == path {
			if f[1] != want {
				t.Fatalf("go.mod requires %s %s, and update.go's goModUpgrade is @%s: make it %s@%s", path, f[1], want, path, f[1])
			}
			return
		}
	}
	t.Fatalf("go.mod doesn't require %s: its tool line is the pin (go get -tool %s)", path, goModUpgrade)
}
