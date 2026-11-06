# mise-check in this repo

This repo pins [mise-check](https://github.com/joeblew999/mise-check) in `.mise/config.toml`, from its GitHub
releases: `mise install` gets it. Every command is a task of `.mise/tasks/mise-check.toml`, numbered in the order a repo
meets them. Start with `mise run mise-check`: what's done, and what to run next.

## A new repo

An empty folder, its own git, then mise-check's files and the template's little app:

```sh
git init
go mod init <module>
mise exec github:joeblew999/mise-check@latest -- mise-check init
mise install
mise run mise-check:template
mise run mise-check
```

A repo with Go of its own already skips `go mod init` and `mise-check:template` (it never writes over a file).

## The steps

| Task | What |
|---|---|
| `mise run mise-check:03-init` | mise-check's files written into the repo again, after upgrading it |
| `mise run mise-check:04-check` | the repo's rules: init's files current, `.mise/config.toml` pinning mise-check |
| `mise run mise-check:05-show` | what's done, and what to run next (`mise run mise-check` too) |


Then, whenever you want pins moved, each task named for what it acts on:

| Task | Acts on |
|---|---|
| `mise run mise-check:update` | all three below, in order |
| `mise run mise-check:update-mise` | the mise tools, in the repo's mise files |
| `mise run mise-check:update-packages` | the Node packages: `package.json` and its lockfile, through aube |
| `mise run mise-check:update-go` | Go, in every `go.mod`: its toolchain line and modules, majors too |

Each takes `--pick` (asked for each one, with the picker of each tool), `--no-major` (no major moves) and `--json`
(one JSON document on stdout: every pin, its file, before and after, whether it moved and whether that was a major
move; the tools' own output goes to stderr). `show`, `check` and `show-users` take `--json` too.

And `mise run mise-check:show-users`: which of the owner's repos on GitHub use this one (its Go module in their
`go.mod`, its binary in their mise files), at what version, the ones behind its newest release marked. It reads the
repos on GitHub, as Go's own index (pkg.go.dev's imported-by) counts no imports from tests, and mise pins aren't in
any index; private repos need a token (`GITHUB_TOKEN`, `GH_TOKEN`, or `gh auth login`).

And `mise run mise-check:verify` (what `.github/workflows/mise-check.yml` runs on GitHub: first each kind of pin checked
consistent with what pins it, as the updates leave them, then the repo's own `setup` and `test` tasks if it has them,
then check), `mise run mise-check:template`, `mise run mise-check:show-version`,
`mise run mise-check:show-help`.

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
rest: then `mise run mise-check:03-init` writes its files again. In CI it doesn't run: CI only checks.

go-mod-upgrade skips `// indirect` lines, so the first `go mod tidy` is needed. mise can't write a `go.mod` (its
idiomatic version file for Go), so `update-mise` leaves Go to `update-go`.

What `verify` checks of each kind, so a pin file out of step with the code fails here and on GitHub, and an update
is what fixes it:

| Kind | Moved by | `verify` checks |
|---|---|---|
| mise tools | `update-mise` | nothing more: the mise files pin exact versions, their own lock |
| Node packages | `update-packages` | `aube install --frozen-lockfile`: `package.json` and the lockfile agree |
| Go | `update-go` | `go mod tidy -diff` in every go.mod: go.mod and go.sum are what the imports need |

| Task | `--pick` | `--no-major` |
|---|---|---|
| `update-mise` | `mise upgrade --interactive` | (mise's own ranges) |
| `update-packages` | `aube update --interactive` | without `--latest`: each package within its range |
| `update-go` | go-mod-upgrade's own picker | go-mod-upgrade `--no-major` |

go-mod-upgrade is pinned once, in mise-check's own `go.mod`, and run at that version: the repo needs no line of its
own for it. A newer mise-check brings a newer go-mod-upgrade.
