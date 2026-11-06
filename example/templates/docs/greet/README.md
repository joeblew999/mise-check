# greet in this repo

This repo's [greet](https://github.com/joeblew999/mise-check/tree/main/example), the example tool of mise-check's
reference for a tool whose users are Go repos.

Written by `go tool greet init`, for the version this repo's `go.mod` pins: don't edit it. After upgrading,
`go tool greet init` writes it again, and `go tool greet check` fails until it has.

## What the repo needs: only its Go module

greet comes from a Go module, so a repo needs nothing but its `go.mod` to get it, run it and pin it. **mise is
optional.**

| | How | Checked by |
|---|---|---|
| The command | `go tool greet <command>` | `go.sum`: exactly the version `go.mod` pins |
| The module a Go program imports | `import "github.com/joeblew999/mise-check/example/greet"` | `go.sum` and the compiler |
| Both at one version | one line of `go.mod`: `tool github.com/joeblew999/mise-check/example/cmd/greet` | `go tool greet check` |
| Go itself | `go.mod`'s `toolchain` line (any Go from 1.21 downloads it) | Go |

With mise, the repo gets each command as a task too, numbered in the order a repo meets them, and mise installs the Go
that `go.mod` names.

## Set up and upgrade

At the repository's root (GitHub and Claude Code read `.github/` and `.claude/` only there):

1. **A Go module**, if the repo has none: `go mod init <module>`.
2. **greet, at the newest release:**

   ```sh
   go get -tool github.com/joeblew999/mise-check/example/cmd/greet@latest
   go tool greet init
   ```

   The first adds the tool line to `go.mod` (and the module to `require`). The second writes greet's files: its tasks
   (`.mise/tasks/greet.toml`), its GitHub workflow, its Claude Code hook and skill, and these docs.
3. **The template's little app, only in a repo with no Go program yet:** `go tool greet template`.
4. **Then** `go tool greet` (or `mise run greet`): what's done, and what to run next.

Commit what steps 1 to 3 wrote. **To upgrade,** upgrade the module as any Go module (`go get -tool
github.com/joeblew999/mise-check/example/cmd/greet@latest`, or the repo's own task for all its pins), then
`go tool greet init`: `go tool greet check` says when greet's files are out of date.

## The steps

| Task (with mise) | Command (without) | What it does |
|---|---|---|
| `mise run greet` | `go tool greet` | What's done, and what to run next |
| `greet:03-init` | `go tool greet init` | greet's files, as this version writes them |
| `greet:04-check` | `go tool greet check` | The repo's rules: greet's files current, `go.mod`'s tool line |
| `greet:05-show` | `go tool greet show` | What's done, and what to run next (`--json`: one JSON document) |
| `greet:06-hello` | `go tool greet hello` | Greet someone: `--name you --shout` |
| `greet:07-bye` | `go tool greet bye` | Goodbye |
| `greet:verify` | `go tool greet verify` | What GitHub runs: the repo's setup and test tasks, then check |
| `greet:template` | `go tool greet template` | The template's little app, in a repo with no Go program yet |
| `greet:show-help` | `go tool greet show-help` | Every command |

GitHub runs `.github/workflows/greet.yml` on every push: it checks out, installs mise, then runs
`mise run greet:verify`. Run that task here and you see what GitHub sees.
