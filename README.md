# mise-check

[![ci](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml/badge.svg)](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/joeblew999/mise-check.svg)](https://pkg.go.dev/github.com/joeblew999/mise-check)

mise-check is for any repo whose [mise](https://mise.jdx.dev) tasks run Go. It's a Go module you import from your own
tests, with two packages:

| Package | Import | What it does |
|---|---|---|
| `misecheck` | `github.com/joeblew999/mise-check` | Checks a [mise](https://mise.jdx.dev) file against the Go its tasks run, both ways, and that every task runs the same on macOS, Linux and Windows |
| `walk` | `github.com/joeblew999/mise-check/walk` | The end-to-end test that runs a repo's numbered mise tasks in their order, the way a person there does, each checked |

Used by [mise-cloudflare](https://github.com/joeblew999/mise-cloudflare) and ubuntu-website (misecheck), and by any tool
built the same way.

**All your repo needs is the Go module:** `go get github.com/joeblew999/mise-check@latest`. There is no binary to
install, nothing to copy, and nothing from this repo's mise file: `mise.toml` here is only for working on mise-check
itself.

## Use misecheck

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
| `GO_CMD`, `GO_LAYOUT` | For tasks that run binaries of yours: their folder (`<GO_CMD>/<binary>/`), and how each binary's commands are laid out, `file` (a file each) or `folder` (a folder each). Both, or neither |
| `GO_CALLERS` | The folder of Go that runs the tasks by name, an end-to-end test say (optional) |
| `GO_BINARIES` | Which binaries of `GO_CMD` to hold to the strict way, comma-separated (optional; empty, all of them): for a repo taking it up one binary at a time |

## Use walk

Your test says only what's yours: the steps (each a numbered task of your mise file, named in full, and what to check
after it) and how a person makes a new repo with your tool. `walk` does the rest: the repo cut off from the repo
around it (its own mise only), its tasks run by name, its setup, the steps in order, the files a step changed,
process groups and Windows' long paths.

```go
var myWalk = walk.Walk[*myRepo]{Steps: steps, New: newRepo, Of: func(r walk.Repo) *myRepo { return &myRepo{r} }}

func TestWalk(t *testing.T)  { myWalk.Run(t) }
func TestSteps(t *testing.T) { walk.CheckSteps(t, "templates/.mise/tasks/mytool.toml", steps) }
```

A task of your mise file runs it, saying which repo:

| Variable | What |
|---|---|
| `REPO_PATH` | The folder to walk, from where the test runs: `templates`, your template where it is. Without it, the walk skips |
| `E2E_STAMPED=true` | Walk a new repo instead, made from nothing by your `New` |
| `E2E_STAGE` | How far to walk, when your steps have stages |

`CheckSteps` fails unless your steps are exactly your mise file's numbered tasks, in their order, and misecheck
(`GO_CALLERS`) checks that every task name your steps use is a task. When the repo walked needs something its own mise
files can't give it, cut off from the repo around it, `Prepare` adds it (the example's gives its template, a folder of
its module with no `go.mod` of its own, that module's Go).

## What misecheck checks

Every task: its `run` and `usage` are each one line; it declares its flags and arguments in mise's `usage` and passes
each by name with mise's templates (`{% if usage.yes %} --yes{% endif %}`); its run has no POSIX shell syntax
(`${…}`, `$name`, `NAME=value cmd`), which Windows' cmd doesn't read.

The Go a task runs: from source (`go run|test|build ./path`, any folder, several, `./...`) the folder must hold Go;
a binary of yours (`<binary> <command>`, built or run as `go tool <binary> <command>`) must have that command, every command must be one a task runs, and every flag
is declared on both sides. Anything else a task runs (`hugo …`, `wrangler …`) is held to the rules every task is, above, and logged as not
checked: a file of only such tool tasks passes on those rules alone, unless it's given `GO_CMD` (then its tasks must
run its Go).

Go that runs tasks names each in full (`"namespace:task"`), and every name must be a task.

The full rules are the [package documentation](https://pkg.go.dev/github.com/joeblew999/mise-check).

## The example

[example/](example/) is a worked example of both packages together: a small made-up tool, `greet`, with a template
that misecheck checks and `walk` walks, as mise-cloudflare does with its own. It isn't part of mise-check: nothing in
it is released, and you never import or install it. Its [README](example/README.md) says how it's built, for anyone
writing a tool of the same kind. misecheck's own tests also break a copy of it nine ways, one at a time, to prove each
break is caught.

## This repo

`mise.toml` here is only for working on mise-check. `mise run lib:test` runs its tests, `mise run lib:check-mise`
checks that file with misecheck, and `mise run example:check` runs the example: misecheck on its mise files, then its
walk on its template and on a new repo. CI runs all three on Linux, macOS and Windows.

## License

MIT
