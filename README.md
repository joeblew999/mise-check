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
   A developer who wants a say moves one kind only, each its own task named for what it acts on
   (`mise-check:update-mise`, `mise-check:update-packages`, `mise-check:update-go`), or takes no majors (`--no-major`). That's the binary, `mise-check`, which one command adds to a repo, as
   [mise-cloudflare](https://github.com/joeblew999/mise-cloudflare) is added: from then on every step is a mise task,
   and `mise run mise-check` shows where the repo is.
2. **Checks a repo's mise tasks against the Go they run, both ways, and walks them end to end**: two Go packages a
   repo's own tests import, `misecheck` and `walk`, in a repo that has run `init` as any other (below):
   `misecheck.Check` fails in one that only requires them, and says what to run. mise-check holds itself to them, and
   so does mise-cloudflare.

## Use it in your repo

How to use mise-check, from a new repo to every command and its flags, is one page:
[templates/docs/mise-check/README.md](templates/docs/mise-check/README.md), which `init` writes into your repo as
`docs/mise-check/README.md`. To start, at the repository's root, with [mise](https://mise.jdx.dev/getting-started.html)
installed:

```sh
mise exec github:joeblew999/mise-check@latest -- mise-check init
mise run mise-check   # what's done, and what to run next
```

## Working on mise-check

This repo is two things, kept apart:

| | What | Where |
|---|---|---|
| **For other repos** | The `mise-check` binary and the template, a whole repo you can drop anywhere and run, whose tool half `init` writes into a repo (its tasks, its check workflow, its Claude Code hook and skill, its docs, its `.gitignore`); and the two packages, `misecheck` and `walk` | [templates/](templates/), [cmd/mise-check/](cmd/mise-check/), [misecheck.go](misecheck.go), [walk/](walk/) |
| **For this repo** | The tasks for working on it, its tests, its CI. This README | [mise.toml](mise.toml), the `*_test.go` at the top, [.github/workflows/](.github/workflows/) |

Nothing for this repo is ever in `templates/`, and nothing in `templates/` is this repo's own. Everything is driven
from this repo's [mise.toml](mise.toml), run at the top, and each task runs what it needs first: run the furthest you
need, here, never waiting on GitHub. In this checkout the template's `.mise/config.toml` says `latest`, and
`mise run tool:install` overrides it on this machine only, with this checkout's build.

### Tasks

Every task is named noun:verb, what it acts on and then what it does. `mise tasks` lists each with what it does.

| Task | What | When |
|---|---|---|
| `root:ci` | What GitHub's CI runs, the same command: `root:verify`, then `new-tool:e2e` | Before every push |
| `root:verify` | The checks at once, then the walks one after the other (they share the machine's aube store); all but `new-tool:e2e`, which runs it in the repo it makes | After every change |
| `root:check-mise` | `TestRootMise` on this repo's own `mise.toml`: one line each, its Go there, `usage` passed by name, no POSIX shell syntax | After a change to this repo's tasks |
| `root:update` | This repo's own pins (`mise.toml`'s tools, `go.mod`'s modules) at their newest, by the binary's own update, from source | To move its pins; then `tool:test` and the walks |
| `root:show-users` | Which of joeblew999's repos use mise-check (go.mod, mise pins), at what version, the behind ones marked | `tool:release` runs it after every release |
| `tool:test` | `go vet`, the Go tests (the checker, the template broken nine ways, the walk, the binary; none of them walk), the release config | After every change |
| `tool:build` | This checkout's build, `bin/mise-check` | Any task needs it |
| `tool:install` | That build as the mise version `local`, for any repo on this machine (`mise use github:joeblew999/mise-check@local` there) and the template's (`templates/.mise/config.local.toml`, never committed) | To try a change in another repo, never by waiting on a release |
| `tool:release` | A release from the commit checked out, as `release.yml` runs it; `--dry-run` tags and publishes nothing | Never by hand: a merge to main runs it |
| `template:check-mise` | `TestTemplateMise` on the template's tasks against the binary's Go, the walk that runs them and the skill, both ways | After a change to a task, a command or the skill |
| `template:e2e` | The walk on the template, where it is | After every change |
| `template:e2e-update` | A copy of the template as it is, in a folder of its own, with the go.mod `mise-check:template` writes, given old pins of every kind: each update must move its own and nothing else, then `update` has nothing left, and `verify` passes. The template itself never changes | After a change to an update |
| `template:update` | The template's own pins at their newest, by its own `mise-check:update` | To move its pins; then the walks |
| `new-repo:e2e` | The walk on a new repo made as the docs page says (`--version` makes it at a release) | After every change |
| `new-repo:e2e-update` | A new repo given old pins of every kind: `update-mise`, `update-packages` and `update-go` must each move its own and nothing else, then `update` has nothing left, and `verify` passes | After a change to how a new repo is made (CI runs the template's) |
| `new-tool:e2e` | A new mise tool repo, made by `mise-check new-tool` in an empty folder (`--version` makes it at a release), then its own `root:verify` must pass. Its GitHub repo is [mise-new-tool](https://github.com/joeblew999/mise-new-tool), empty on purpose. Not in `root:verify`, which it runs in the repo it makes, where it would make one again | `root:ci` runs it, and `new-repo.yml` at every release |

### Tests

**The walk** ([e2e_walk_test.go](e2e_walk_test.go)), on this repo's own `walk`. A repo meets
`mise-check:init`, `mise-check:check` and `mise-check:show` in turn
([templates/.mise/tasks/mise-check.toml](templates/.mise/tasks/mise-check.toml)), after its git, `mise install` and
its own `setup`. One name all the way through: `mise-check:check` runs `mise-check check`,
which is `main.go`'s `case "check"` calling `r.check` in `check.go`. It walks:

- **The template**, [templates/](templates/), in place.
- **A new repo**, made as a person makes one: an empty folder (`mise-check-new`) with its own git, `mise-check init`,
  then its own `mise-check:template`. Its `init` runs at this checkout's build (`local`), or at a release with
  `--version`, and pins itself in `.mise/config.toml`.
- **The template and a new repo, each with old pins** (`TestUpdateTemplate` and `TestUpdateNewRepo`; `template:e2e-update` runs on a copy of the template
  as it is, so the template itself never changes): node 24.11.0 in `mise.toml`, wrangler 4.100.0 in `package.json`,
  go-chi/chi v1.5.4 (imported by the app) and google/uuid v1.3.0 in go.mod. `mise-check:update-mise`,
  `update-packages` and `update-go` must each move its own kind and nothing else, chi to `/v5` with the app's import
  rewritten; then `mise-check:update` has nothing left to move, and `mise-check:verify` passes. All real tools, nothing faked.

**The mise ↔ Go checker** on mise-check's own template ([misecheck_test.go](misecheck_test.go)): it passes, and a copy
of it broken eleven ways, one at a time (its skill among them), is caught each time.

### Layout

| Path | Whose | What |
|---|---|---|
| [templates/](templates/) | other repos' | the template: the little app plus the tool's half, which `init` writes into a repo at the same paths; [templates.go](templates/templates.go) lists that half |
| [cmd/mise-check/](cmd/mise-check/) | other repos' | the command: a file per command, `x.go` holding `r.x`, which `main.go`'s `case "x"` calls |
| [misecheck.go](misecheck.go), [walk/](walk/) | other repos' | the two packages a repo's tests import |
| [e2e_walk_test.go](e2e_walk_test.go), [e2e_tool_test.go](e2e_tool_test.go) | this repo's | at the top, as the end-to-end runs on everything else: the walks and the updates; `TestToolRelease`, the release, which only `mise run tool:release` runs |
| [.github/workflows/](.github/workflows/) | this repo's | its own workflows; other repos get theirs from `templates/` |

### CI and releases

Shell steps only, no JavaScript actions. Each workflow checks out and installs mise, then only runs the root mise, as
on any machine:

- [ci.yml](.github/workflows/ci.yml), on every push and pull request, on Linux, macOS and Windows: `mise run
  root:ci`, the same command as on your machine: `tool:test`, `root:check-mise`, `template:check-mise`,
  then the walks one after the other (`template:e2e`, `template:e2e-update`, `new-repo:e2e`, `new-tool:e2e`), as they share the machine's one aube store. `new-repo:e2e-update` is the same update walk on a new repo: run it after a change to how a new repo is made.
- [new-repo.yml](.github/workflows/new-repo.yml), once a release run has passed (or by hand, for any release): a new
  repo made as the docs page says at that release, walked, and a new mise tool repo made by `new-tool` at that release, its
  own `root:verify` run (`new-tool:e2e --version`), on Linux, macOS and Windows.
- [release.yml](.github/workflows/release.yml), on every change to what mise-check is (its binary, its packages, the
  template) merged to main: `tool:release`. It takes the commit's `v*` tag or tags the next patch version,
  [goreleaser](.goreleaser.yaml) puts the `mise-check` binary for macOS, Linux and Windows on the release, a new repo
  made at that release is walked (`new-repo:e2e --version`), and the version goes on proxy.golang.org, so `go get`
  and pkg.go.dev have it at once; then `root:show-users` lists the repos that could move to it. So the release is always main: the binary and the packages are one version.

This README is the one place for working on mise-check, for people and agents alike: CLAUDE.md only loads it (`@README.md`). How to use mise-check is the docs page's, [templates/docs/mise-check/README.md](templates/docs/mise-check/README.md), never this README's: here, only how to start, and the link there.

## License

MIT
