package main

import (
	"flag"
	"fmt"
	"strings"
)

// hello is greet hello, run by the task greet:hello, whose usage declares the same two flags.
func hello(args []string) {
	fs := flag.NewFlagSet("hello", flag.ExitOnError)
	name := fs.String("name", "world", "who to greet")
	shout := fs.Bool("shout", false, "in capitals")
	fs.Parse(args)
	msg := "hello, " + *name
	if *shout {
		msg = strings.ToUpper(msg)
	}
	fmt.Println(msg)
}
