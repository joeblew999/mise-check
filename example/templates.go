// Package example is mise-check's reference for a mise tool whose users are Go repos: copy this folder to start one,
// and change what's greet's. Read README.md here.
//
// The tool, greet, is one Go module with no binary: its command (cmd/greet), which Go builds from the module when a
// repo runs go tool greet, and its package (greet/), which a Go program imports. One line of the repo's go.mod pins the
// module, checked by go.sum and the compiler. Its template (templates/) is a whole repo that runs where it is, in two
// parts: the tool's files, which greet init writes into any repo (the numbered tasks, the GitHub workflow, the Claude
// Code hook and skill, the docs), and the template's little app, which greet template writes into a repo with no Go
// program yet. The template is a
// folder of this module, with no go.mod of its own, so this file, which embeds it, is here, one level up.
package example

import (
	"embed"
	"io/fs"
)

//go:embed templates/.mise/tasks/greet.toml templates/.mise/config.toml
//go:embed templates/.github/workflows/greet.yml
//go:embed templates/.claude/hooks/greet-session-start.sh templates/.claude/settings.json templates/.claude/skills
//go:embed templates/docs/greet
//go:embed templates/.gitignore
var tool embed.FS

// Tool is the template's tool half: the files greet init writes into a repo, at their paths there.
var Tool = sub(tool)

//go:embed templates/main.go templates/main_test.go templates/mise.toml templates/README.md
var app embed.FS

// App is the template's little app: the files greet template writes into a repo with no Go program yet, at their
// paths there. Its mise.toml runs and tests it, and README.md is the repo's front page.
var App = sub(app)

// AppName is the template's name in its README.md: a repo gets its own instead, its folder's.
const AppName = "greet-template"

// HookFile is the Claude Code hook's name in a repo's .claude/hooks.
const HookFile = "greet-session-start.sh"

func sub(f embed.FS) fs.FS {
	s, err := fs.Sub(f, "templates")
	if err != nil {
		panic(err)
	}
	return s
}
