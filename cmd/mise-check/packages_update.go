package main

// packagesUpdate is mise-check packages-update (mise run mise-check:packages-update): the repo's Node packages, package.json and its lockfile (aube update), and nothing else.
func (r repo) packagesUpdate(o updates) { r.updatePins(o, "packages") }
