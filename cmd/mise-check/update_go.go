package main

// updateGo is mise-check update-go (mise run mise-check:update-go): the repo's Go, in every go.mod: its toolchain line and its modules, majors too (go-mod-upgrade), and nothing else.
func (r repo) updateGo(o updates) { r.updatePins(o, "go") }
