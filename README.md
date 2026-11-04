# mise-check

[![ci](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml/badge.svg)](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/joeblew999/mise-check.svg)](https://pkg.go.dev/github.com/joeblew999/mise-check)

A Go module that keeps a [mise](https://mise.jdx.dev) file and the Go its tasks run in sync, both ways, and every task
runs the same on macOS, Linux and Windows. It forces one structure, and checks the shape, never what the Go does.

## Why

In a repo run by [mise](https://mise.jdx.dev), the mise file is the interface: people, CI and AI agents all do
things with `mise run <task>`. Behind each task is Go (or a tool). And Go calls back: an end-to-end test runs
`mise run <task>`, a tool prints "now run `mise run <task>`". So names, commands and flags cross between the mise
file and the Go in both directions, often several levels deep:

```
mise.toml task ──runs──▶ Go (a command, or a test) ──runs by name──▶ mise.toml task ──runs──▶ Go …
```

Nothing in mise or in Go checks those crossings, so they drift apart without a sound:

- a task is renamed, and a test or a printed hint still names the old one;
- a Go command is removed or renamed, and its task still runs it; or one is added, and no task runs it;
- a flag is added in Go and never declared in the task's `usage` (mise then refuses it), or the other way round;
- a run line picks up `$VAR`, `${…}` or `NAME=value cmd`: fine on a Mac, broken in Windows' cmd;
- a task grows into a shell script inside the TOML, where Go's compiler, vet and tests never see it.

Each is a one-line slip, easy for a person and easier still for an AI agent that edits one side without the other,
and none fails until someone runs that task, on that OS. misecheck turns the crossing into a contract and fails the
build when it breaks: one structure, checked both ways, the same on every OS.

## Real-world use

[mise-cloudflare](https://github.com/joeblew999/mise-cloudflare) is built this way: its tasks run its Go, its Go and its
end-to-end tests run its tasks back by name, and a task of each of its mise files checks that very file with misecheck
(`mise run root:check-mise`, `mise run template:check-mise`). Look there to see it in real use.

## The example

[example/](example/) is a small repo that uses all of it: a mise file, a binary whose commands are a file each, and
a Go test that runs the tasks back by name. It passes the strict check, and misecheck's own tests break a copy of it
nine ways, one at a time, to prove each is caught. Start there.

## Use it

```sh
go get github.com/joeblew999/mise-check@latest
```

A test of your own calls it:

```go
func TestMise(t *testing.T) { misecheck.Check(t, misecheck.FromEnv()) }
```

and a task of your mise file runs that test, saying what to check:

```toml
[tasks."root:check-mise"]
env = { MISE_FILE = "mise.toml" }
run = 'go test . -run "^TestMise$" -count=1 -v'
```

| Variable | What |
|---|---|
| `MISE_FILE` | The mise file: a `mise.toml`, or a file of tasks only (`.mise/tasks/*.toml`). Without it, the check skips |
| `GO_CMD`, `GO_LAYOUT` | For tasks that run built binaries: their folder (`<GO_CMD>/<binary>/`), and how each binary's commands are laid out, `file` (a file each) or `folder` (a folder each). Both, or neither |
| `GO_CALLERS` | The folder of Go that runs the tasks by name, an end-to-end test say (optional) |

## What it checks

Every task: its `run` and `usage` are each one line; it declares its flags and arguments in mise's `usage` and passes
each by name with mise's templates (`{% if usage.yes %} --yes{% endif %}`); its run has no POSIX shell syntax
(`${…}`, `$name`, `NAME=value cmd`), which Windows' cmd doesn't read.

The Go a task runs: from source (`go run|test|build ./path`, any folder, several, `./...`) the folder must hold Go;
a built binary (`<binary> <command>`) must have that command, every command must be one a task runs, and every flag
is declared on both sides. Anything else a task runs (`hugo …`, `wrangler …`) is logged as not checked.

Go that runs tasks names each in full (`"namespace:task"`), and every name must be a task.

The full rules are the [package documentation](https://pkg.go.dev/github.com/joeblew999/mise-check).

## This repo

It checks itself as any repo does: `mise run root:check-mise`. `mise run lib:test` runs its own tests, the example's
breakages among them, and `mise run example:check` runs the example: its strict check and its end-to-end test.

## License

MIT
