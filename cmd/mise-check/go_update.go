package main

// goUpdate is mise-check go-update (mise run mise-check:go-update): the repo's Go, in every go.mod: its toolchain line and its modules, majors too (go-mod-upgrade), and nothing else.
func (r repo) goUpdate(o updates) { r.updatePins(o, "go") }
