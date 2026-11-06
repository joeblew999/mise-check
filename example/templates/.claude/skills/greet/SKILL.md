---
name: greet
description: Use when working on this repo's greet (the example tool of mise-check's reference) or the Go program that imports its module. greet comes from the repo's go.mod (go tool greet, one tool line pins it and its module); with mise, every command is a task too, mise run greet:<nn>-<command>.
---

# This repo's greet

greet comes from this repo's `go.mod`: one tool line (`tool github.com/joeblew999/mise-check/example/cmd/greet`) pins
the command, `go tool greet`, and the module the repo's Go imports, `github.com/joeblew999/mise-check/example/greet`, at
one version, checked by `go.sum` and the compiler. Everything about it is in this repo's
[docs/greet/README.md](../../../docs/greet/README.md): read it first.

Start with `go tool greet`, or `mise run greet`: it shows which steps are done, and what to run next.

The tasks are numbered in the order a repo meets them, and each runs the command of its name
(`mise run greet:06-hello` runs `go tool greet hello`): `03-init`, `04-check`, `05-show`, `06-hello`, `07-bye`. And
`greet:verify` (what GitHub runs), `greet:template` (the template's little app, in a repo with no Go program yet),
`greet:show-help`. To upgrade greet, upgrade its module as any Go module (`go get -tool …/cmd/greet@latest`), then
`go tool greet init`.

## Never

- Never install greet another way (`go install`, a binary on the PATH): it runs as `go tool greet`, at the version
  `go.mod` pins, so the command and the module are always the same release.
- Never edit what `go tool greet init` writes: `.mise/tasks/greet.toml`, `docs/greet/`, this skill,
  `.github/workflows/greet.yml` and the Claude Code hook. After upgrading greet, run `go tool greet init`,
  which writes them again.
- Never weaken a rule `go tool greet check` reports: fix the repo.
