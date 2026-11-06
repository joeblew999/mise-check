// Command app is the template's little app: a Go program, so the repo has Go for mise-check to keep current. go run
// ./app (mise run start) prints its greeting.
package main

import "fmt"

// greeting is what the app prints.
func greeting(name string) string { return "hello, " + name }

func main() { fmt.Println(greeting("app")) }
