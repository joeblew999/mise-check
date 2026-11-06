---
name: mise-check
description: Use when upgrading this repo's pins (mise tools, packages, Go, Go modules, majors included), checking its mise tasks, or what its GitHub workflow runs. The repo uses mise-check, at the version its .mise/config.toml pins; every command is a task, mise run mise-check:<command>.
---

# This repo's pins and mise tasks

This repo uses mise-check, at the version its `.mise/config.toml` pins. Every command, and what each does, is in this
repo's [docs/mise-check/README.md](../../../docs/mise-check/README.md): read it first.

Start with `mise run mise-check`: it shows the steps, which are done, and what to run next.

The tasks are numbered in the order a repo meets them, and each runs the command of its name
(`mise run mise-check:06-update` runs `mise-check update`): `03-init`, `04-check`, `05-show`, `06-update`, each
`mise run mise-check:<it>`. Steps 00 to 02 come before them: the repo's git, `mise install`, its own `setup`. And
`mise run mise-check:verify` (what GitHub runs), `mise run mise-check:template` (the template's little app, in a repo
with no Go program yet), `mise run mise-check:show-help`.

To upgrade anything the repo pins, run `mise run mise-check:06-update`, then `mise run mise-check:verify`. It moves
every pin in one go: mise's tools in the repo's mise files, its packages (aube), and in every go.mod its Go (the
toolchain line) and modules, majors too, their import paths rewritten.

## Never

- Never upgrade a pin by hand (`go get -u`, `mise use`, editing versions): run `mise run mise-check:06-update`, so
  every pin moves the same way, newest releases past their minimum age, and the run lists what moved.
- Never edit the files init writes (`.mise/tasks/mise-check.toml`, the workflow, this skill, the docs, the hook):
  `mise run mise-check:03-init` writes them again, and `04-check` fails when they differ.
