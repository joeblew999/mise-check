package main

import "fmt"

// showVersion prints this build's version (mise run mise-check:show-version).
func showVersion() { fmt.Println(buildVersion()) }
