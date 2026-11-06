package main

import (
	"testing"

	"github.com/joeblew999/mise-check/example/greet"
)

// TestGreeting: the app's greeting is greet's module half, as the app prints it.
func TestGreeting(t *testing.T) {
	if got := greet.Hello("app", false); got != "hello, app" {
		t.Errorf("greet.Hello: %q", got)
	}
}
