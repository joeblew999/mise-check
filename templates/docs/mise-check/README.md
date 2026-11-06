# mise-check in this repo

This repo pins [mise-check](https://github.com/joeblew999/mise-check) in `.mise/config.toml`, from its GitHub
releases: `mise install` gets it. Every command is a task of `.mise/tasks/mise-check.toml`, numbered in the order a repo
meets them. Start with `mise run mise-check`: what's done, and what to run next.

## A new repo

An empty folder, its own git, then mise-check's files and the template's little app, exactly as the walk proves it on
macOS, Linux and Windows (mise run new-repo:e2e):

```sh
git init
mise exec github:joeblew999/mise-check@latest -- mise-check init   # mise-check's files, pinned at that release
mise run mise-check:template                                       # the app, its go.mod, mise.toml, package.json
mise install                                                       # node, aube, Go (from go.mod)
mise run setup                                                     # the packages (aube install)
mise run mise-check                                                # what's done, and what to run next
```

No Go is needed first: `mise-check:template` writes the go.mod, and `mise install` brings the Go it names. A repo with
code of its own skips `mise-check:template`, which never writes over a file anyway.

## The tasks

| Task | Run it | What it does |
|---|---|---|
| `mise run mise-check` | first, and whenever you're unsure | what's done for the repo, and what to run next |
| `mise run mise-check:init` | after upgrading mise-check | writes mise-check's files again: its tasks, the workflow, the skill, the docs, the hook |
| `mise run mise-check:check` | after any change to a mise file or a skill; when it fails, fix the side it names | the repo's rules: its mise files by misecheck, its skills against their tasks, mise-check's files current, its two pins (the binary in mise, the packages in go.mod) at one release |
| `mise run mise-check:show` | to see where the repo is | the same as mise run mise-check |
| `mise run mise-check:verify` | before every push | what GitHub runs: each pin consistent with what pins it, the repo's setup and test, then check |
| `mise run mise-check:update` | when the person asks to upgrade everything | every pin at its newest: the three below, in order, then mise-check's two pins at one release |
| `mise run mise-check:update-mise` | to upgrade the mise tools only | the tools in the repo's mise files |
| `mise run mise-check:update-packages` | to upgrade the Node packages only | package.json and its lockfile, through aube |
| `mise run mise-check:update-go` | to upgrade Go only | every go.mod's toolchain line and modules, majors too, their import paths rewritten |
| `mise run mise-check:show-users` | to see which of the owner's repos use this one, and which are behind | which of the owner's repos use it, at what version, the behind ones marked |
| `mise run mise-check:template` | only in a repo with no Go program yet | the template's little app, its go.mod and packages; it never writes over a file |
| `mise run mise-check:show-version` | to say which mise-check this is | its version |
| `mise run mise-check:show-help` | to list every command | every command and its flags |

The four updates take `--no-major` (no major moves) and `--json` (one JSON document on stdout: every pin, its file,
before and after, whether it moved and whether that was a major move; the tools' own output goes to stderr).
`mise run mise-check`, `show`, `check`, `show-users` and `show-version` take `--json` too.

`mise run mise-check:show-users` finds the repos that use this one by their `go.mod` (its Go module) and their mise
files (its binary). It reads them on GitHub, as Go's own index (pkg.go.dev's imported-by) counts no imports from tests, and mise pins aren't in
any index; private repos need a token (`GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth login`).

`mise run mise-check:verify` is what `.github/workflows/mise-check.yml` runs on GitHub: first each kind of pin checked
consistent with what pins it, as the updates leave them, then the repo's own `setup` and `test` tasks if it has them,
then check.

## What the updates run

`mise run mise-check:update` moves every pin the repo has of its own, each with its own tool (each step below is
also a task of its own: `update-mise`, `update-packages`, `update-go`), and mise reads no
parent folder's config, so nothing outside the repo moves:

1. mise's tools, in the repo's mise files: `mise upgrade --bump --local`
2. its packages, when it has a `package.json`: `aube update --latest`, which writes whichever lockfile the repo has
3. in every `go.mod` of the repo (none in `vendor/`, `node_modules/`, `testdata/` or a dot folder):
   - its Go: `go get toolchain@latest`, the toolchain line. The `go` line, the least Go the module needs, stays.
   - its modules: `go mod tidy`, then [go-mod-upgrade](https://github.com/oligot/go-mod-upgrade) `--force`, majors
     too (it rewrites their import paths), then `go mod tidy`. Releases younger than 7 days are skipped.

It ends with every pin, the moved ones before → after, majors marked. `git diff` shows them; `mise run
mise-check:verify` proves them, as a major move can break the repo's own code. mise-check's own pin moves with the
rest: then `mise run mise-check:init` writes its files again. In CI it doesn't run: CI only checks.

go-mod-upgrade skips `// indirect` lines, so the first `go mod tidy` is needed. mise can't write a `go.mod` (its
idiomatic version file for Go), so `update-mise` leaves Go to `update-go`.

What `verify` checks of each kind, so a pin file out of step with the code fails here and on GitHub, and an update
is what fixes it:

| Kind | Moved by | `verify` checks |
|---|---|---|
| mise tools | `update-mise` | nothing more: the mise files pin exact versions, their own lock |
| Node packages | `update-packages` | `aube install --frozen-lockfile`: `package.json` and the lockfile agree |
| Go | `update-go` | `go mod tidy -diff` in every go.mod: go.mod and go.sum are what the imports need |


go-mod-upgrade is pinned once, in mise-check's own root `mise.toml`, and run at that version: the repo needs nothing
of its own for it. A newer mise-check brings a newer go-mod-upgrade.

## Keeping tasks, Go, tests and skills in step: misecheck

A repo with mise-check's binary gets misecheck's rules on its own mise files with no code: `mise run
mise-check:check` runs them on its `mise.toml` and `.mise/tasks/*.toml` (each task one line, usage passed by name, no
POSIX shell syntax). The strict checks, for a repo with a binary of its own (its tasks against its Go, the tests that
run them, and its skill), are a test of your own for each mise file, saying in Go what to check:

```go
func TestRootMise(t *testing.T) { misecheck.Check(t, misecheck.Config{MiseFile: "mise.toml"}) }
```

and a task of your mise file runs that test by name:

```toml
[tasks."root:check-mise"]
run = 'go test . -run "^TestRootMise$" -count=1 -v'
```

| `Config` | What |
|---|---|
| `MiseFile` | The mise file: a `mise.toml`, or a file of tasks only (`.mise/tasks/*.toml`) |
| `GoCmd`, `GoLayout` | For tasks that run binaries of yours: their folder (`<GoCmd>/<binary>/`), and how each binary's commands are laid out, `file` (a file each) or `folder` (a folder each). Both, or neither |
| `GoCallers` | The folder of Go that runs the tasks by name, an end-to-end test say (optional) |
| `GoBinaries` | Which binaries of `GoCmd` to hold to the strict way (optional; none, all of them): for a repo taking it up one binary at a time |
| `Skills` | The agent skill of these tasks, a `SKILL.md`, and any page that must cover them too, your docs (optional): each must name every task and every flag the tasks take, and every task it names must be one, so the skill always covers everything the binary can do |

## The end-to-end tests: walk

Your test says only what's yours: the steps (each a task of your mise file, named in full, and what to check after it)
and how a person makes a new repo with your tool. `walk` does the rest: the repo cut off from the repo
around it (its own mise only), its tasks run by name, its setup, the steps in order, the files a step changed,
process groups and Windows' long paths.

```go
var myWalk = walk.Walk[*myRepo]{Steps: steps, New: newRepo, Of: func(r walk.Repo) *myRepo { return &myRepo{r} }}

func TestWalkTemplate(t *testing.T) { myWalk.Template(t, "templates") }
func TestWalkNewRepo(t *testing.T)   { myWalk.NewRepo(t) }
func TestSteps(t *testing.T)         { walk.CheckSteps(t, "templates/.mise/tasks/mytool.toml", steps) }
```

Which repo is the test's name: `Template` walks your template where it is, `NewRepo` a new repo made from nothing by
your `New`. A task of your mise file runs each by name, and a walk runs only when named (`walk.OnlyByName`, which
your own tests that change things can call too), so `go test ./...` never walks:

```toml
[tasks."new-repo:e2e"]
usage = 'flag "--version <version>" help="make it at this release"'
run = 'go test . -run "^TestWalkNewRepo$" -count=1 -v{% if usage.version %} -args -version "{{usage.version}}"{% endif %}'
```

| Test flag | What |
|---|---|
| `-version` | The release a new repo is made at (default this checkout's build) |
| `-stage` | How far to walk, when your steps have stages (default the first) |

For a tool whose steps are numbered in the order a repo takes them (mise-cloudflare's `cloudflare:03-init` to
`15-clean`), `CheckSteps` fails unless your steps are exactly your mise file's numbered tasks, in their order; a tool
with few steps (mise-check's `init`, `check`, `show`) needs no numbers, and no `TestSteps`. Either way, misecheck
(`GoCallers`) checks that every task name your steps use is a task. When the repo walked needs something its own mise
files can't give it, cut off from the repo around it, `Prepare` adds it (mise-check's own gives its template, a folder of
its module with no `go.mod` of its own, that module's Go).

## What misecheck checks, in full

Every task: its `run` and `usage` are each one line; it declares its flags and arguments in mise's `usage` and passes
each by name with mise's templates (`{% if usage.yes %} --yes{% endif %}`); its run has no POSIX shell syntax
(`${…}`, `$name`, `NAME=value cmd`), which Windows' cmd doesn't read.

The Go a task runs: from source (`go run|test|build ./path`, any folder, several) the folder must hold Go, and for
`./...` or `./path/...` Go anywhere below it, as `go test` finds it;
a binary of yours (`<binary> <command>`, built or run as `go tool <binary> <command>`) must have that command, every command must be one a task runs, and every flag
is declared on both sides. Anything else a task runs (`hugo …`, `wrangler …`) is held to the rules every task is, above, and logged as not
checked: a file of only such tool tasks passes on those rules alone, unless it's given `GoCmd` (then its tasks must
run its Go).

A task's `go test -run "^TestX$"` must name a test there is, in the folder it tests: a test renamed, and the task
that runs it says so. Go that runs tasks names each in full (`"namespace:task"`), and every name must be a task. Each
of `Skills` names every task and every flag, and every task it names is one.

Skills: `mise run mise-check:check` holds every skill of the repo, `.claude/skills/<namespace>/SKILL.md`, to the tasks
of that namespace, both ways: it names every task and every flag they take, and every task of the namespace it names
is one. A test of the repo's own gets the same with `misecheck.CheckSkills(t, ".")`.

The full rules are the [package documentation](https://pkg.go.dev/github.com/joeblew999/mise-check).
