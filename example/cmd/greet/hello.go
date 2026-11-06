package main

import (
	"flag"
	"fmt"

	"github.com/joeblew999/mise-check/example/greet"
)

// hello is greet hello (06, mise run greet:06-hello), whose task's usage declares the same two flags: the package's
// greet.Hello, on the command line.
func hello(args []string) {
	fs := flag.NewFlagSet("hello", flag.ExitOnError)
	name := fs.String("name", "world", "who to greet")
	shout := fs.Bool("shout", false, "in capitals")
	fs.Parse(args)
	fmt.Println(greet.Hello(*name, *shout))
}
