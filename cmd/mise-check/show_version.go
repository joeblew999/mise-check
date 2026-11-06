package main

import "fmt"

// showVersion prints this build's version (mise run mise-check:show-version), or with --json one JSON document of it.
func showVersion(jsonOut bool) {
	if jsonOut {
		emit(struct {
			Version string `json:"version"`
		}{buildVersion()})
		return
	}
	fmt.Println(buildVersion())
}
