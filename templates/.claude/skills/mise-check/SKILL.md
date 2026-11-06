---
name: mise-check
description: Use whenever you add, rename or change a mise task or the Go command it runs, upgrade anything the repo pins (mise tools, Node packages, Go and Go modules, majors included), or check what its GitHub workflow runs. The repo uses mise-check, at the version its .mise/config.toml pins, and every command is a task, mise run mise-check:<command>. It keeps the mise tasks, the Go they run, the end-to-end tests that run them and this skill in step, both ways. Never upgrade a pin by hand, and never let a task and its Go drift apart.
---

# This repo's mise tasks, their Go, and its pins

This repo uses mise-check (github.com/joeblew999/mise-check) at the version its `.mise/config.toml` pins. It does two
jobs: it keeps the repo's mise tasks and the Go they run in step, and it moves every pin the repo has. Everything, with
each command's details, is in [docs/mise-check/README.md](../../../docs/mise-check/README.md).

Start with `mise run mise-check`: what's done, and what to run next.

## The tasks

Each runs the mise-check command of its name (`mise run mise-check:check` runs `mise-check check`).

| Task | Run it | What it does |
|---|---|---|
| `mise run mise-check` | first, and whenever you're unsure | what's done for the repo, and what to run next |
| `mise run mise-check:init` | after upgrading mise-check | writes mise-check's files again: its tasks, the workflow, the skill, the docs, the hook |
| `mise run mise-check:check` | after any change to a mise file or a skill | the repo's rules: its mise files by misecheck, its skills against their tasks, mise-check's files current, its pin |
| `mise run mise-check:show` | to see where the repo is | the same as mise run mise-check |
| `mise run mise-check:verify` | before every push | what GitHub runs: each pin consistent with what pins it, the repo's setup and test, then check |
| `mise run mise-check:update` | when the person asks to upgrade everything | every pin at its newest: the three below, in order |
| `mise run mise-check:update-mise` | to upgrade the mise tools only | the tools in the repo's mise files |
| `mise run mise-check:update-packages` | to upgrade the Node packages only | package.json and its lockfile, through aube |
| `mise run mise-check:update-go` | to upgrade Go only | every go.mod's toolchain line and modules, majors too, their import paths rewritten |
| `mise run mise-check:show-users` | after a release of this repo | which of the owner's repos use it, at what version, the behind ones marked |
| `mise run mise-check:template` | only in a repo with no Go program yet | the template's little app, its go.mod and packages; it never writes over a file |
| `mise run mise-check:show-version` | to say which mise-check this is | its version |
| `mise run mise-check:show-help` | to list every command | every command and its flags |

Flags, each declared in the task's usage and passed by name:

- `--json`: one JSON document on stdout instead of text, on `mise run mise-check`, `show`, `check`, the four updates,
  `show-users` and `show-version`. Read that, never the text: the text may change, the JSON keeps its fields. The
  updates' JSON is every pin with its file, `before`, `after`, `moved` and `major`.
- `--no-major`: on the four updates, no major moves: packages within their ranges, Go modules within their major.

A command given a flag it doesn't take refuses it. The updates never run in CI: CI only checks.

After any update, run `mise run mise-check:verify`: a major move can break the repo's own code, and that's where it
shows. Fix the code; never move the pin back by hand. Then `git diff` shows exactly what moved, and the update's last
lines (or its `--json`) list every pin, before → after.

## Keeping tasks, Go, tests and this skill in step

misecheck is mise-check's checker, a Go package (`github.com/joeblew999/mise-check`). It holds one structure on a mise
file and everything around it, both ways, so nothing drifts:

- **Every task:** its `run` and `usage` are each one line; every flag and argument it takes is declared in mise's
  `usage` and passed by name (`{% if usage.json %} --json{% endif %}`, `"{{usage.name}}"`); no POSIX shell syntax
  (`$name`, `${…}`, `NAME=value cmd`), which Windows' cmd doesn't read: variables go in the task's `env`. Longer logic
  belongs in Go, never in the mise file.
- **Tasks ↔ the Go they run (`GO_CMD`, `GO_LAYOUT`):** every task's command is a command file of the binary
  (`cmd/<binary>/<command>.go`, dashes as underscores, holding the function named for it), every command file is a
  task's, and every flag is declared on both sides: in the task's `usage` and in the Go (`flag.Bool("json", …)`).
- **Tasks ↔ the end-to-end tests (`GO_CALLERS`):** every task the tests name, in full (`"mise-check:check"`, never
  glued from pieces), is a task.
- **Tasks ↔ this skill (`SKILL_FILE`):** every task and every flag is named in the skill, and every task the skill
  names is a task.

`mise run mise-check:check` runs the first of these on the repo's own `mise.toml` and `.mise/tasks/*.toml`, and the
last on every skill of the repo (`.claude/skills/<namespace>/SKILL.md` against the tasks of `<namespace>`), with no
code of the repo's own. The strict ones, for a repo with a binary of its own, are a test of the repo's:

```go
func TestMise(t *testing.T) { misecheck.Check(t, misecheck.FromEnv()) }
```

run by a task whose `env` says what to check (`MISE_FILE`, and `GO_CMD`, `GO_LAYOUT`, `GO_CALLERS`, `GO_BINARIES`,
`SKILL_FILE` as it has them; `misecheck.CheckSkills(t, ".")` holds every skill, as `check` does):

```toml
[tasks."template:check-mise"]
env = { MISE_FILE = "templates/.mise/tasks/mytool.toml", GO_CMD = "cmd", GO_LAYOUT = "file", GO_CALLERS = ".", SKILL_FILE = "templates/.claude/skills/mytool/SKILL.md" }
run = 'go test . -run "^TestMise$" -count=1 -v'
```

So when you add, rename or remove a command: change its Go, its task, the end-to-end test that runs it, and its
skill, together, then run the check. When it fails, it names the side that's out of step: fix that side. Never weaken
the check, skip it, or work around a rule (a shell script, a second line).

The end-to-end tests are mise-check's other package, `walk` (`github.com/joeblew999/mise-check/walk`): the repo's
tasks run by name, in turn, through its own mise file only, as a person runs them, on the template where it is and
on a new repo made from nothing, each step checked by what a person would see it do. Real tools, never mocks.

## Never

- Never upgrade a pin by hand (`go get -u`, `npm install x@latest`, `aube add`, `mise use`, editing a version): run
  the update for its kind, so every pin moves the same way, past each tool's minimum age, and the run lists what moved.
- Never run `npm`, `npx`, `pnpm` or `yarn`: the packages are aube's, at the version the repo's mise pins.
- Never add a task without its Go, its test and its line in the skill, nor a command without its task.
- Never edit what `mise run mise-check:init` writes: `.mise/tasks/mise-check.toml`, `docs/mise-check/`, this skill,
  `.github/workflows/mise-check.yml` and the Claude Code hook. After upgrading mise-check, run
  `mise run mise-check:init`, which writes them again.
- Never weaken a rule `mise run mise-check:check` reports: fix the repo.
