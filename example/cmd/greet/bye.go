package main

import "fmt"

// bye is greet bye, run by the task greet:bye. It takes no flags, so that task has no usage.
func bye(args []string) {
	fmt.Println("bye")
}
