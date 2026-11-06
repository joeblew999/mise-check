package main

import "fmt"

// showVersion prints this build's version (mise run greet:show-version): the module version go.mod pins.
func showVersion() { fmt.Println(buildVersion()) }
