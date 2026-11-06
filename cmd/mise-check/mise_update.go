package main

// miseUpdate is mise-check mise-update (mise run mise-check:mise-update): the repo's mise tools, in its mise files (mise upgrade --bump), and nothing else.
func (r repo) miseUpdate(o updates) { r.updatePins(o, "mise") }
