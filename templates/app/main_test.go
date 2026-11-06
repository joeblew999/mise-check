package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting("app"); got != "hello, app" {
		t.Fatalf("greeting(app) = %q", got)
	}
}
