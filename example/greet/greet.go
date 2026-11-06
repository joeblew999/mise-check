// Package greet is what a Go program imports of the example's tool. It's in the same module as the command
// (go tool greet), so one line of a repo's go.mod pins both at one version. The template's app imports it
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
