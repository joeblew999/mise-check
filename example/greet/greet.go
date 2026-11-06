// Package greet is the module half of the example's tool: what a Go program imports, at the same version as the
// command it runs (go tool greet), both from one line of its go.mod. The template's app imports it
// (templates/main.go), and the command's hello and bye are this, on the command line.
package greet

import "strings"

// Hello greets name, in capitals when shout is true: hello, world.
func Hello(name string, shout bool) string {
	msg := "hello, " + name
	if shout {
		msg = strings.ToUpper(msg)
	}
	return msg
}

// Bye is goodbye.
func Bye() string { return "bye" }
