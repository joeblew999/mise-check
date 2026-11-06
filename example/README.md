# The example: a mise tool for Go repos

`greet` is a tool built the way [mise-cloudflare](https://github.com/joeblew999/mise-cloudflare) is, for repos that
are Go: its template is a whole repo that runs where it is, its `init` writes the template's files into any repo, and
its walk runs the template's numbered tasks in their order, on the template and on a new repo made from nothing. It
shows mise-check's two packages working with a mise file: `misecheck` holds every task to the Go it runs, and `walk`
drives the template through its tasks. It isn't released or built on its own: copy this folder to start a real tool,
and change what's greet's.

## What a repo using the tool needs: only its Go module

**mise is optional.** greet is one Go module, and there's no binary: nothing is built ahead or published. The module
has a command in it (`cmd/greet`, a `main` package, which Go builds from the module when the repo runs `go tool greet`)
and a package a Go program imports (`greet/`). One line of a repo's `go.mod` pins the module, so both are always the
same version:

```sh
go get -tool github.com/joeblew999/mise-check/example/cmd/greet@latest   # the tool line: greet's module, at one version
go tool greet init                                                       # greet's files (tasks, workflow, hook, skill, docs)
go tool greet                                                            # what's done, and what to run next
```

| | How | Checked by |
|---|---|---|
| The command | `go tool greet <command>` | `go.sum`: exactly the version `go.mod` pins |
| The module a Go program imports | `import "github.com/joeblew999/mise-check/example/greet"` | `go.sum` and the compiler |
| Go itself | `go.mod`'s `toolchain` line | Go (any Go from 1.21 downloads it) |
| Upgrading | as any Go module: `go get -tool …/cmd/greet@latest`, then `go tool greet init` | `go tool greet check` says when greet's files are stale |
| With mise | the same commands as tasks, numbered: `mise run greet:06-hello` runs `go tool greet hello` | misecheck, here |

Never a release binary: a binary's command line isn't checked when it's used, and a module is, by `go.sum` and the
compiler. A repo never gets this folder's `mise.toml`: that is only for working on greet.

## Run it

Here, in `example/`:

```sh
mise run template:e2e           # the walk on the template, where it is: greet:03-init to 07-bye, each checked
mise run new-repo:e2e           # the same on a new repo, made from nothing as docs/greet/README.md says
mise run template:check-mise    # misecheck on the template's tasks against greet's Go and the walk, both ways
mise run root:check-mise        # misecheck on this folder's own mise.toml
```

And in the template itself, as a person in a repo with greet does: `cd templates`, then `go tool greet` (or
`mise run greet`).

## What each part is

| Path | What it is | What to change when you copy it |
|---|---|---|
| [mise.toml](mise.toml) | The tool repo's own tasks, only for working on the tool (noun:verb): `template:e2e`, `new-repo:e2e`, `template:check-mise`, `tool:test`, `root:check-mise` | greet's name in the descriptions |
| [cmd/greet/](cmd/greet/) | The command, `go tool greet`, one file per command, holding the function named for it: `init.go`, `check.go`, `show.go`, `hello.go`, `bye.go`, `template.go`, `verify.go`, `show_help.go`, `show_version.go` | Keep `init`, `check`, `show`, `template`, `verify`, `show-help`, `show-version`; replace `hello` and `bye` with the tool's own work |
| [greet/](greet/) | The package a Go program imports (`greet.Hello`), in the same module as the command | The tool's own package |
| [templates/](templates/) | The template: a whole repo that runs where it is, a folder of this module with no `go.mod` of its own (`go tool greet` runs here from this module's tool line) | Everything greet's |
| `templates/.mise/tasks/greet.toml` | The numbered tasks, each one line, `go tool greet <command>`: `greet` (start here), `greet:03-init` … `greet:07-bye`, `greet:verify`, `greet:template`, `greet:show-*` | The tool's own steps, in the order a repo meets them |
| `templates/.mise/config.toml` | Only has mise install the Go `go.mod` names | Nothing |
| `templates/.github/workflows/greet.yml`, `.claude/`, `docs/greet/` | The repo's workflow (`mise run greet:verify`), Claude Code's hook and skill, and its docs: all written by `greet init`, never edited in a repo | greet's name |
| `templates/app/` (`main.go`, `main_test.go`), `templates/mise.toml`, `templates/README.md` | The template's little app, which imports the module half: `greet template` writes it into a repo with no Go program yet | The tool's own little example |
| [templates/templates.go](templates/templates.go) | Embeds the template's two parts, `Tool` (the tool's files: init writes them) and `App` (the little app: template writes it). The template's only Go at its top, as mise-cloudflare's `templates.go` is; the app's Go is its own package, in `templates/app/` | The file lists |
| [walk_test.go](walk_test.go) | The walk: the template's numbered tasks as steps, each checked; how a person makes a new repo; `TestSteps` (the steps are exactly the numbered tasks) | The tool's own steps and checks |
| [mise_test.go](mise_test.go) | All a repo writes to use misecheck: one line | Nothing |

So it goes both ways, several levels deep: `mise run template:e2e` (this folder's `mise.toml`) runs `TestWalk`
(`walk_test.go`), which runs the template's tasks by name (`mise run greet:06-hello`, in `templates/`), which run
greet's Go (`go tool greet hello`, `cmd/greet/hello.go`), which is the package `greet` (`greet.Hello`). misecheck checks
every one of those links (`template:check-mise`, `root:check-mise`).

## Break it

misecheck's own tests (`TestBroken`, in `../misecheck_test.go`) break a copy of this folder one way at a time and check
that each is caught. Try one here and run `mise run template:check-mise`:

1. Rename `["greet:07-bye"]` in `templates/.mise/tasks/greet.toml`: `walk_test.go` still runs `greet:07-bye`.
2. Delete `cmd/greet/bye.go`: `greet:07-bye` still runs it.
3. Add `fs.Bool("loud", …)` to `hello.go`: no task declares `--loud`.
4. Add `flag "--color"` to `greet:06-hello`'s `usage`: greet's Go has no such flag.
5. Take `{% if usage.shout %} --shout{% endif %}` out of `greet:06-hello`'s run: `--shout` is declared and never passed.
6. Write `GREETING=1 go tool greet bye`: Windows' cmd doesn't read it.
7. Spread a run over several lines: logic that belongs in Go.
8. Add `cmd/greet/wave.go` with `func wave`: a command no task runs.
9. Write `"greet:" + step` in Go: a task name that can't be checked.
