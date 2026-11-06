package main

import (
	"fmt"

	"github.com/joeblew999/mise-check/example/greet"
)

// bye is greet bye (07, mise run greet:07-bye). It takes no flags, so its task has no usage.
func bye(args []string) {
	fmt.Println(greet.Bye())
}
