# mise-check

[![ci](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml/badge.svg)](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/joeblew999/mise-check.svg)](https://pkg.go.dev/github.com/joeblew999/mise-check)

A Go module that keeps a [mise](https://mise.jdx.dev) file and the Go its tasks run in sync, both ways, and every task
runs the same on macOS, Linux and Windows. It forces one structure, and checks the shape, never what the Go does. Two
packages:

- **`misecheck`** (`github.com/joeblew999/mise-check`): the checker, below.
- **`walk`** (`github.com/joeblew999/mise-check/walk`): the end-to-end test that drives a repo through its own mise
  file, its numbered tasks in their order, each checked ([The walk](#the-walk)).

> **A repo that uses mise-check needs only the Go module.** `go get github.com/joeblew999/mise-check`, and a test of
> its own imports it. Nothing else: no mise from this repo, no binary, no files copied. This repo's own mise file
> (`mise.toml`) is only for working on mise-check itself.

## Why: a control plane that tests other repos through their own mise files

A repo's root mise file can be a control plane, and not only for that repo: for the other repos it works on. Its tasks
run end-to-end tests, and those tests drive each of the other repos **through that repo's own mise file**, the way a
person working there does, by running its tasks by name. [mise-cloudflare](https://github.com/joeblew999/mise-cloudflare)
is the real one, and the place to look. Its root mise.toml drives three kinds of repo, each through the repo's own tasks:

- the template it ships (`templates/`): `mise run template:e2e-serve`, `template:e2e-deploy`;
- a new repo, made from nothing as a person makes one (`mise-cloudflare init` in an empty folder):
  `mise run new-repo:e2e-serve`, `new-repo:e2e-deploy`;
- a third-party Worker, adopted as its owner would: `mise run repos:e2e-adopt`.

Each of those tasks runs the same end-to-end test, `TestWalk`, on its repo. It runs that repo's numbered tasks in
order with `mise run`, and each of those runs the tool's own Go:

```
mise run template:e2e-serve                    the control plane: mise-cloudflare's root mise.toml
  └─ go test . -run TestWalk                   e2e_walk_test.go: the walk
       └─ mise run cloudflare:03-init          the repo walked, through its own mise file
          mise run cloudflare:04-check           (templates/.mise/tasks/mise-cloudflare.toml)
          …
          mise run cloudflare:08-deploy
            └─ mise-cloudflare init | check | … | deploy     the tool: cmd/mise-cloudflare/<command>.go
```

So a mise file isn't glue around the product: it *is* the product's interface, the thing the end-to-end test tests,
in the control plane and in every repo it drives. Names, commands and flags cross between the mise files and the Go
in both directions, several levels deep (a task runs Go, which runs a task, which runs Go), and nothing in mise or in
Go checks those crossings. They drift apart without a sound:

- a task is renamed, and the end-to-end test, or a hint the tool prints, still names the old one;
- a Go command is removed or renamed, and its task still runs it; or one is added, and no task runs it;
- a flag is added in Go and never declared in the task's `usage` (mise then refuses it), or the other way round;
- a run line picks up `$VAR`, `${…}` or `NAME=value cmd`: fine on a Mac, broken in Windows' cmd;
- a task grows into a shell script inside the TOML, where Go's compiler, vet and tests never see it.

Each is a one-line slip, easy for a person and easier still for an AI agent that edits one side without the other.
And the end-to-end walk is the slowest test there is (minutes, on a live Cloudflare account): a slip found there costs
a walk, and fails for a reason that hides the real one. misecheck checks every one of those links, both ways, in a
second, before the walk runs: each task the test names is a task; each task's command is in the Go, and each Go
command is a task's; each flag is declared on both sides; each run line runs on Windows too. In mise-cloudflare a task
of each mise file runs it on that very file (`mise run root:check-mise`, `mise run template:check-mise`), so the walk
only ever fails for what it's there to find.

A repo without such a test gets the same contract: its tasks and its Go stay one structure, checked both ways.

## The example

[example/](example/) is the same loop, whole and small: a tool, `greet`, built the way mise-cloudflare is, and walked
by `walk` as mise-cloudflare's is. It's also the reference for a mise tool whose users are Go repos: copy it to start
one. It passes the strict check, and misecheck's own tests break a copy of it nine ways, one at a time, to prove each
is caught. Start there.

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
| `GO_BINARIES` | Which binaries of `GO_CMD` to hold to the strict way, comma-separated (optional; empty, all of them): for a repo taking it up one binary at a time |

## What it checks

Every task: its `run` and `usage` are each one line; it declares its flags and arguments in mise's `usage` and passes
each by name with mise's templates (`{% if usage.yes %} --yes{% endif %}`); its run has no POSIX shell syntax
(`${…}`, `$name`, `NAME=value cmd`), which Windows' cmd doesn't read.

The Go a task runs: from source (`go run|test|build ./path`, any folder, several, `./...`) the folder must hold Go;
a built binary (`<binary> <command>`) must have that command, every command must be one a task runs, and every flag
is declared on both sides. Anything else a task runs (`hugo …`, `wrangler …`) is held to the rules every task is, above, and logged as not
checked: a file of only such tool tasks passes on those rules alone, unless it's given `GO_CMD` (then its tasks must
run its Go).

Go that runs tasks names each in full (`"namespace:task"`), and every name must be a task.

The full rules are the [package documentation](https://pkg.go.dev/github.com/joeblew999/mise-check).

## The walk

`walk` is mise-cloudflare's end-to-end test with everything of Cloudflare's taken out: a repo's numbered tasks
(`mise run greet:03-init`, `greet:04-check`, …) run in their order, through the repo's own mise file, as a person there
runs them, each checked by what it should have done. A tool's repo walks its template where it is, and a new repo made
from nothing as its README says, so every step a user takes is known to work.

The tool's own test says only what's the tool's: its steps, what each checks, and how a person makes a new repo with
it. `walk` does the rest: the repo cut off from the repo around it (its own mise only), its tasks run by name, its
setup, the steps in order, the files a step changed, process groups and Windows' long paths. A whole walk is one value:

```go
var greetWalk = walk.Walk[*greeter]{Steps: steps, New: newRepo, Of: func(r walk.Repo) *greeter { return &greeter{r} }}

func TestWalk(t *testing.T)  { greetWalk.Run(t) }
func TestSteps(t *testing.T) { walk.CheckSteps(t, "templates/.mise/tasks/greet.toml", steps) }
```

and a task of the tool's root mise file runs it, saying which repo: `REPO_PATH=templates` (the template where it is),
`E2E_STAMPED=true` (a new repo made from nothing), `E2E_STAGE` (how far). The steps name their tasks in full, so
misecheck checks every name against the tasks file (`GO_CALLERS`), and `CheckSteps` fails unless the steps are exactly
its numbered tasks, in their order. [example/walk_test.go](example/walk_test.go) is a whole one.

## The reference: a mise tool for Go repos

[example/](example/) is how to build a tool that Go repos use, so that a repo using it has as little of its own to
manage as possible. A tool built this way has two halves at one version, a command and a module, and **a repo that
uses it needs only its Go module: mise is optional.**

| A repo using the tool needs | How | Checked by |
|---|---|---|
| The command | `go tool greet <command>`, from one line of its `go.mod` (`go get -tool …/cmd/greet@latest`) | `go.sum`: exactly the version `go.mod` pins |
| The module its Go imports | `import "…/example/greet"` | `go.sum` and the compiler |
| Go itself | `go.mod`'s `toolchain` line (any Go from 1.21 downloads it) | Go |
| Upgrading | as any Go module: `go get -tool …/cmd/greet@latest` | `go tool greet check` says when the tool's files are stale |
| mise (optional) | its tasks, `mise run greet:<nn>-<command>`, one line each: `go tool greet <command>` | misecheck, in the tool's repo |

The command comes from the module, never a release binary: a binary's command line isn't checked when it's used, and
a module is, by `go.sum` and the compiler. The tool's repo has its template (`templates/`), a whole repo that runs
where it is: the tool's half (`go tool greet init` writes it into a repo) and a little app that imports the module
(`go tool greet template` writes it into a repo with no Go program yet). [example/README.md](example/README.md) says
what each file is, and what to change when you copy it.

## This repo

`mise.toml` here is only for working on mise-check: a repo that uses mise-check never needs it. mise-check checks
itself as any repo does, `mise run lib:check-mise`. `mise run lib:test` runs its own tests, the example's breakages
among them, and `mise run example:check` runs the example through its own mise file: misecheck on both its mise files,
then its walk on the template and on a new repo. CI runs all three on Linux, macOS and Windows.

## License

MIT
