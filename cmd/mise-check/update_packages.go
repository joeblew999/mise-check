package main

// updatePackages is mise-check update-packages (mise run mise-check:update-packages): the repo's Node packages, every package root's package.json and lockfile (aube update), and nothing else.
func (r repo) updatePackages(o updates) { r.updatePins(o, "packages") }
