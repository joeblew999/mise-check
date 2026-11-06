// Package templates is mise-check's template: a whole repo with mise-check, which runs wherever it is put, as
// mise-cloudflare's is. It has two halves. The tool's half, listed below, is what mise-check init writes into any repo,
// at the same paths: the .mise folder (the tasks, and mise-check's pin, which mise install gets from its GitHub
// releases), the GitHub workflow, the Claude Code hook and its settings, the agent skill, the docs and the .gitignore.
// The rest is the template's little app: a Go program (app/; mise-check template gives a repo its go.mod), the
// packages it pins (package.json and its lockfile, which aube installs), the mise.toml with node and aube and its
// tasks, and README.md, there so the template
// runs: init never writes it, and mise-check template writes it into a repo with no Go program yet. What is for
// mise-check itself (its own mise.toml, tests, CI and README) is never here, and neither is the machine's own
// .mise/config.local.toml.
package templates

import (
	"embed"
	"io/fs"
)

//go:embed .mise/tasks/mise-check.toml .mise/config.toml
//go:embed .github/workflows/mise-check.yml
//go:embed .claude/hooks/mise-check-session-start.sh .claude/settings.json .claude/skills
//go:embed docs/mise-check
//go:embed .gitignore
var tool embed.FS

// Tool is the template's tool half: the files init writes into a repo, at their paths there.
var Tool fs.FS = tool

//go:embed app/main.go app/main_test.go mise.toml package.json pnpm-lock.yaml README.md
var app embed.FS

// App is the template's little app: the files mise-check template writes into a repo with no Go program yet, at their
// paths there. Its mise.toml runs and tests it, and README.md is the repo's front page.
var App fs.FS = app

// AppName is the template's name in its README.md: a repo gets its own instead, its folder's.
const AppName = "mise-check-template"

// Tasks is the repo's mise tasks, .mise/tasks/mise-check.toml.
var Tasks, _ = fs.ReadFile(Tool, ".mise/tasks/mise-check.toml")

// HookFile is the Claude Code hook's name in a repo's .claude/hooks.
const HookFile = "mise-check-session-start.sh"
