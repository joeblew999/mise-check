---
name: mise-check
description: Use when upgrading anything this repo pins (mise tools, Node packages in package.json, Go and Go modules, majors included), checking its mise tasks, or what its GitHub workflow runs. The repo uses mise-check, at the version its .mise/config.toml pins; every command is a task, mise run mise-check:<command>. Never upgrade a pin by hand.
---

# This repo's pins and mise tasks

This repo keeps its pins and its mise tasks with mise-check, at the version its `.mise/config.toml` pins. Every
command, what each update moves and how, and its flags, is in this repo's
[docs/mise-check/README.md](../../../docs/mise-check/README.md): read it first.

Start with `mise run mise-check`: it shows the steps, which are done, and what to run next.

The tasks are numbered in the order a repo meets them, and each runs the command of its name
(`mise run mise-check:04-check` runs `mise-check check`): `03-init`, `04-check`, `05-show`, each
`mise run mise-check:<it>`. Steps 00 to 02 come before them: the repo's git, `mise install`, its own `setup`. And
`mise run mise-check:verify` (what GitHub runs), `mise run mise-check:template` (the template's little app, in a repo
with no Go program yet), `mise run mise-check:show-help`.

The updates, each named for what it acts on:

- `mise run mise-check:update`: all three below, in order. The one to run when the person says "upgrade everything".
- `mise run mise-check:mise-update`: the mise tools, in the repo's mise files.
- `mise run mise-check:packages-update`: the Node packages, `package.json` and its lockfile, through aube.
- `mise run mise-check:go-update`: Go in every `go.mod`, its toolchain line and its modules, majors too, their import
  paths rewritten.

Each ends with every pin, the moved ones before → after, majors marked. `--no-major` takes no majors; `--pick` asks
for each one, which needs a person at a terminal: never pass it yourself. After any update, run
`mise run mise-check:verify`: a major move can break the repo's own code, and that's where it shows. Fix the code;
don't move the pin back by hand.

What GitHub runs on every push is `.github/workflows/mise-check.yml`, which checks out, installs mise, then runs
`mise run mise-check:verify`. Run that task here and you see what GitHub sees. The updates don't run in CI: CI only
checks.

## Never

- Never upgrade a pin by hand (`go get -u`, `npm install x@latest`, `aube add`, `mise use`, editing a version): run
  the update for its kind, so every pin moves the same way, past each tool's minimum age, and the run lists what moved.
- Never run `npm`, `npx`, `pnpm` or `yarn`: the packages are aube's, at the version the repo's mise pins.
- Never add a `tool` line for go-mod-upgrade to the repo's `go.mod`: mise-check runs it at the version its own release
  pins.
- Never edit what `mise run mise-check:03-init` writes: `.mise/tasks/mise-check.toml`, `docs/mise-check/`, this skill,
  `.github/workflows/mise-check.yml` and the Claude Code hook. After upgrading mise-check, run
  `mise run mise-check:03-init`, which writes them again.
- Never weaken a rule `mise run mise-check:04-check` reports: fix the repo.
