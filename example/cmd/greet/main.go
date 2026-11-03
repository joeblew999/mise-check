// Command greet is the example's binary. Each of its commands is a file of its own, holding the function named for it
// (hello.go, bye.go), and each is run by one task of ../../mise.toml, which declares its flags too.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: greet hello|bye [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "hello":
		hello(os.Args[2:])
	case "bye":
		bye(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "greet: no command %q\n", os.Args[1])
		os.Exit(2)
	}
}
