# mise-check

[![ci](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml/badge.svg)](https://github.com/joeblew999/mise-check/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/joeblew999/mise-check)](https://github.com/joeblew999/mise-check/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/joeblew999/mise-check)](go.mod)
[![license](https://img.shields.io/github/license/joeblew999/mise-check)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/joeblew999/mise-check.svg)](https://pkg.go.dev/github.com/joeblew999/mise-check)

**The problem.** Every repo pins things in three places, and each moves on its own: its **mise tools** (node, aube,
goreleaser, … in its mise files), its **Node packages** (`package.json` and its lockfile), and its **Go** (each
go.mod's toolchain line, and its modules, majors included, whose import paths change with them). Upgrading them is a
different command each, run a little differently in every repo, or not at all, and a Go major means rewriting imports
by hand. And a repo's mise tasks drift from the Go they run, and work on one OS but not another.

**What mise-check does.** Two things, through [mise](https://mise.jdx.dev):

1. **Upgrades everything a repo pins, in one command**, `mise run mise-check:update`: its mise tools
   (`mise upgrade --bump`), its Node packages ([aube](https://github.com/aubepkg/aube), whichever lockfile the repo
   has), and in every go.mod its Go and its modules, majors too, their import paths rewritten
   ([go-mod-upgrade](https://github.com/oligot/go-mod-upgrade)). It ends with every pin, the moved ones before → after.
   A developer who wants a say chooses: `--only mise|packages|go`, `--pick` (asked for each, with each tool's own
   picker), `--no-major`. That's the binary, `mise-check`, which one command adds to a repo, as
   [mise-cloudflare](https://github.com/joeblew999/mise-cloudflare) is added: from then on every step is a mise task,
   and `mise run mise-check` shows where the repo is.
2. **Checks a repo's mise tasks against the Go they run, both ways, and walks them end to end**: two Go packages a
   repo's own tests import, `misecheck` and `walk` (below). mise-check holds itself to them, and so does
   mise-cloudflare.

## Use it in your repo

Step by step, at the repository's root (GitHub and Claude Code read `.github/` and `.claude/` only there), in a repo
that's new or has code of its own:

1. **Install mise**, once on each machine: [mise's getting started](https://mise.jdx.dev/getting-started.html)
   (`curl https://mise.run | sh`, or `brew install mise`).
2. **mise-check's files, always:**

   ```sh
   mise exec github:joeblew999/mise-check@latest -- mise-check init
   ```

   It runs the newest release, or the one you name instead of `latest` (they're all on
   [its releases](https://github.com/joeblew999/mise-check/releases)); mise takes a new release as `latest` once it's
   a day old. It writes the repo's `.mise/` folder (its tasks, and mise-check pinned at exactly that release, which
   mise gets from GitHub), its GitHub workflow, its Claude Code hook and skill, its docs and its `.gitignore`.
3. **The template's app, only in a repo with no Go program yet:**

   ```sh
   mise run mise-check:template
   ```

   It writes the template's little app: a Go program and its go.mod, named for the repo's folder, the packages it
   pins (`package.json`, its lockfile), the `mise.toml` with node and aube, and a README. It never writes over a file.
4. **Then:**

   ```sh
   mise install
   mise run mise-check
   ```

   It shows what's done, and what to run next.

Commit what steps 2 and 3 wrote. Whenever you want the pins moved: `mise run mise-check:update`, then
`mise run mise-check:verify`, which runs what GitHub runs.

`init` writes everything your repo gets, its own docs among them: `docs/mise-check/README.md`. That page is
[templates/docs/mise-check/README.md](templates/docs/mise-check/README.md) here, in the template, just as a repo has it.

## What update moves

Each tool on the repo's own config only (`MISE_CEILING_PATHS`): nothing outside the repo moves. In CI it doesn't run:
CI only checks.

| Pins | How | `--pick` | `--no-major` |
|---|---|---|---|
| mise tools (`--only mise`) | `mise upgrade --bump --no-global` (mise-check's own pin among them; at `local` it stays) | `--interactive` | (mise's own ranges) |
| Node packages (`--only packages`) | `aube update --latest`, when there's a `package.json` (aube pinned in the repo's mise) | `--interactive` | without `--latest`: within each range |
| Go (`--only go`) | in every go.mod (none in `vendor/`, `node_modules/`, `testdata/` or a dot folder): `go get toolchain@latest` (the `go` line, the module's least Go, stays), then `go mod tidy`, go-mod-upgrade `--force --cooldown 7d`, `go mod tidy` | go-mod-upgrade's own picker | `--no-major` |

go-mod-upgrade skips `// indirect` lines, so the first `go mod tidy` is needed. It's pinned once, on mise-check's own
go.mod tool line, and run at that version (`go run …@version`): a repo needs no line of its own for it, which would
raise the repo's `go` line and add the upgrader's modules to its go.mod. A newer mise-check brings a newer
go-mod-upgrade.

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
files can't give it, cut off from the repo around it, `Prepare` adds it (mise-check's own gives its template, a folder of
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

## Working on mise-check

This repo is two things, kept apart:

| | What | Where |
|---|---|---|
| **For other repos** | The `mise-check` binary and the template, a whole repo you can drop anywhere and run, whose tool half `init` writes into a repo (its tasks, its check workflow, its Claude Code hook and skill, its docs, its `.gitignore`); and the two packages, `misecheck` and `walk` | [templates/](templates/), [cmd/mise-check/](cmd/mise-check/), [misecheck.go](misecheck.go), [walk/](walk/) |
| **For this repo** | The tasks for working on it, its tests, its CI. This README | [mise.toml](mise.toml), [e2e/](e2e/), the `*_test.go`, [.github/workflows/](.github/workflows/) |

Nothing for this repo is ever in `templates/`, and nothing in `templates/` is this repo's own. Everything is driven
from this repo's [mise.toml](mise.toml), run at the top, and each task runs what it needs first: run the furthest you
need, here, never waiting on GitHub. In this checkout the template's `.mise/config.toml` says `latest`, and
`mise run tool:install` overrides it on this machine only, with this checkout's build.

### Tasks

Every task is named noun:verb, what it acts on and then what it does. `mise tasks` lists each with what it does.

| Task | What | When |
|---|---|---|
| `lib:test` | `go vet` and the Go tests: the checker (the template broken nine ways), the walk, the binary (none of them walk) | After every change |
| `lib:check-mise` | `TestMise` on this repo's own `mise.toml`: one line each, its Go there, `usage` passed by name, no POSIX shell syntax | After a change to this repo's tasks |
| `template:check-mise` | `TestMise` on the template's tasks against the binary's Go and the walk that runs them, both ways | After a change to a task or a command |
| `template:e2e` | The walk on the template, where it is | After every change |
| `template:update` | The template's own pins at their newest, by its own `mise-check:update` | To move its pins; then the walks |
| `new-repo:e2e` | The walk on a new repo made the README's way (`--version` makes it at a release) | After every change |
| `new-repo:e2e-update` | A new repo given old pins of every kind: `mise-check:update` must move each, then `verify` pass | After a change to `update` |
| `tool:build` | This checkout's build, `bin/mise-check` | Any task needs it |
| `tool:install` | That build as the mise version `local`, for any repo on this machine (`mise use github:joeblew999/mise-check@local` there) and the template's (`templates/.mise/config.local.toml`, never committed) | To try a change in another repo, never by waiting on a release |
| `root:update` | This repo's own pins at their newest, by the binary's own update, from source | To move its pins; then `lib:test` and the walks |

### Tests

**The walk** ([e2e/walk_test.go](e2e/walk_test.go)), on this repo's own `walk`. A repo's steps are its numbered
tasks, `mise-check:03-init`, `04-check` and `05-show`
([templates/.mise/tasks/mise-check.toml](templates/.mise/tasks/mise-check.toml)), after steps 00 to 02 (the repo's
git, `mise install`, its own `setup`). One name all the way through: `mise-check:04-check` runs `mise-check check`,
which is `main.go`'s `case "check"` calling `r.check` in `check.go`. It walks:

- **The template**, [templates/](templates/), in place.
- **A new repo**, made as a person makes one: an empty folder (`mise-check-new`) with its own git, `mise-check init`,
  then its own `mise-check:template`. Its `init` runs at this checkout's build (`local`), or at a release with
  `--version`, and pins itself in `.mise/config.toml`.
- **A new repo with old pins** (`TestUpdate`): node 24.11.0 in `mise.toml`, wrangler 4.100.0 in `package.json`,
  go-chi/chi v1.5.4 (imported by the app) and google/uuid v1.3.0 in go.mod. `mise-check:update` must move every one,
  chi to `/v5` with the app's import rewritten, and `mise-check:verify` pass after. All real tools, nothing faked.

**The mise ↔ Go checker** on mise-check's own template ([misecheck_test.go](misecheck_test.go)): it passes, and a copy
of it broken nine ways, one at a time, is caught each time.

### Layout

| Path | Whose | What |
|---|---|---|
| [templates/](templates/) | other repos' | the template: the little app plus the tool's half, which `init` writes into a repo at the same paths; [templates.go](templates/templates.go) lists that half |
| [cmd/mise-check/](cmd/mise-check/) | other repos' | the command: a file per command, `x.go` holding `r.x`, which `main.go`'s `case "x"` calls |
| [misecheck.go](misecheck.go), [walk/](walk/) | other repos' | the two packages a repo's tests import |
| [e2e/](e2e/) | this repo's | the walk and `TestUpdate` |
| [.github/workflows/](.github/workflows/) | this repo's | its own workflows; other repos get theirs from `templates/` |

### CI and releases

Shell steps only, no JavaScript actions. Each workflow checks out and installs mise, then only runs the root mise, as
on any machine:

- [ci.yml](.github/workflows/ci.yml), on every push and pull request, on Linux, macOS and Windows: `lib:test`,
  `lib:check-mise`, `template:check-mise`, `template:e2e`, `new-repo:e2e`, `new-repo:e2e-update`.
- [new-repo.yml](.github/workflows/new-repo.yml), once a release run has passed (or by hand, for any release): a new
  repo made the README's way at that release, walked, on Linux, macOS and Windows.
- Releases, with goreleaser as mise-cloudflare's, aren't switched on yet: until then a repo runs this checkout's build
  through `mise run tool:install`.

This README is the one place for all of it, for people and agents alike: CLAUDE.md only loads it (`@README.md`).

## License

MIT
