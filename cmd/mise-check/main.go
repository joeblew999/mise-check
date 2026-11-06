// Command mise-check keeps a repo whose mise tasks run Go the way mise-check's packages check it: its tasks written
// in (init), its rules (check), and every pin it has moved to its newest release in one go (update: mise's tools, its
// packages, Go and every Go module, majors too). A repo pins it in .mise/config.toml, from its GitHub releases, as
// mise-cloudflare is pinned. `mise-check show-help` lists the commands.
//
// Each command is a file of its own, holding the function named for it (update.go holds update), so misecheck holds
// every task of the template to the Go it runs, both ways.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
)

const help = `This repo's pins and mise tasks, with mise-check. Each command is a task, numbered in the order a repo meets
them: mise run mise-check:<nn>-<command> runs mise-check <command>.

The steps, in order (mise run mise-check says which is next):
  mise run mise-check:03-init     mise-check's files written into the repo (again, after upgrading)
  mise run mise-check:04-check    the repo's rules: mise-check's files current, its pin
  mise run mise-check:05-show     what's done, and what to run next (mise run mise-check, too)
  mise run mise-check:06-update   every pin at its newest release: mise's tools, packages, Go, Go modules (majors too);
                                  --only mise|packages|go, --pick (ask for each), --no-major

And:
  mise run mise-check:verify         what the repo's GitHub workflow runs: its setup and test tasks, then check
  mise run mise-check:template       the template's little app, in a repo with no Go program yet: it never writes over a file
  mise run mise-check:show-version   this build's version
  mise run mise-check:show-help      this
`

func main() {
	args := os.Args[1:]
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("mise-check "+command, flag.ExitOnError)
	jsonOut := flags.Bool("json", false, "print one JSON document on stdout instead of text")
	only := flags.String("only", "", "update: only these pins, mise (its tools), packages (aube) or go (Go and its modules)")
	pick := flags.Bool("pick", false, "update: ask for each one, with each tool's own picker")
	noMajor := flags.Bool("no-major", false, "update: no major moves (packages within their ranges, Go modules within their major)")
	flags.Usage = func() { fmt.Fprint(os.Stderr, help) }
	flags.Parse(args)
	if flags.NArg() > 0 {
		flags.Usage()
		os.Exit(2)
	}
	dir, err := os.Getwd()
	must(err)
	r := repo{dir: dir}
	switch command {
	case "", "show":
		r.show(*jsonOut)
	case "init":
		r.initRepo()
		trustConfig(dir, ".mise/config.toml")
		sayNewest()
	case "check":
		r.check(*jsonOut)
	case "update":
		if inCI() {
			fail(errors.New("in CI update doesn't run: CI only checks the repo, and update changes its pins; run mise run mise-check:06-update on your machine, then mise run mise-check:verify"))
		}
		r.update(updates{only: *only, pick: *pick, noMajor: *noMajor})
	case "verify":
		r.verify()
	case "template":
		r.template()
	case "show-version":
		showVersion()
	case "show-help":
		showHelp()
	default:
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}
}

// repo is the repo in the folder mise-check runs in.
type repo struct{ dir string }

// inCI reports whether this runs in CI (CI=true, as GitHub Actions and every other CI set it).
func inCI() bool {
	return os.Getenv("CI") != "" && os.Getenv("CI") != "false" && os.Getenv("CI") != "0"
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	must(enc.Encode(v))
}

func must(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mise-check:", err)
	os.Exit(1)
}

// version is set by the release build (goreleaser's -X main.version); otherwise Go's module version, or "(devel)".
var version = ""

func buildVersion() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "(devel)"
}
