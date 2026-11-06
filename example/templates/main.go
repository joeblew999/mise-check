// Command app is the template's little app: a Go program that imports greet's package, from the module the repo's go.mod
// pins for greet's command too. go run . (mise run start) prints its greeting.
package main

import (
	"fmt"

	"github.com/joeblew999/mise-check/example/greet"
)

func main() { fmt.Println(greet.Hello("app", false)) }
