// Package templates is greet's template, embedded for go tool greet: a whole repo that runs where it is, in two parts:
// the tool's files, which greet init writes into any repo (the numbered tasks, the GitHub workflow, the Claude Code hook
// and skill, the docs), and the template's little app, which greet template writes into a repo with no Go program yet
// (app/, which imports greet's package; its tasks; its README). This file is the template's only Go at its top, as
// mise-cloudflare's templates.go is; the app's is a package of its own, in app/.
package templates

import (
	"embed"
	"io/fs"
)

//go:embed .mise/tasks/greet.toml .mise/config.toml
//go:embed .github/workflows/greet.yml
//go:embed .claude/hooks/greet-session-start.sh .claude/settings.json .claude/skills
//go:embed docs/greet
//go:embed .gitignore
var tool embed.FS

// Tool is the template's tool files: what greet init writes into a repo, at their paths there.
var Tool fs.FS = tool

//go:embed app/main.go app/main_test.go mise.toml README.md
var app embed.FS

// App is the template's little app: what greet template writes into a repo with no Go program yet, at its paths
// there. Its mise.toml runs and tests it, and README.md is the repo's front page.
var App fs.FS = app

// AppName is the template's name in its README.md: a repo gets its own instead, its folder's.
const AppName = "greet-template"

// HookFile is the Claude Code hook's name in a repo's .claude/hooks.
const HookFile = "greet-session-start.sh"
