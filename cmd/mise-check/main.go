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
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"slices"
	"strings"
)

const help = `This repo's pins and mise tasks, with mise-check. Each command is a task: mise run mise-check:<command>
runs mise-check <command>.

The repo (mise run mise-check says what to run next):
  mise run mise-check:init               mise-check's files written into the repo (again, after upgrading)
  mise run mise-check:check              the repo's rules: its mise files by misecheck, mise-check's files current, its pin
  mise run mise-check:show               what's done, and what to run next (mise run mise-check, too)

The updates, each pin at its newest release (--no-major: no major moves):
  mise run mise-check:update             all three below, in order
  mise run mise-check:update-mise        the mise tools, in the repo's mise files
  mise run mise-check:update-packages    the Node packages, package.json and its lockfile (aube)
  mise run mise-check:update-go          Go in every go.mod: its toolchain line and modules, majors too (go-mod-upgrade)

And:
  mise run mise-check:verify             what the repo's GitHub workflow runs: each pin consistent, setup, test, check
  mise run mise-check:show-users         which of the owner's repos use this one, at what version
  mise run mise-check:template           the template's little app, in a repo with no Go program yet: it never writes over a file
  mise run mise-check:show-version       this build's version
  mise run mise-check:show-help          this

--json, one JSON document on stdout instead of text: show, check, the updates, show-users, show-version.
`

func main() {
	args := os.Args[1:]
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("mise-check "+command, flag.ExitOnError)
	jsonOut := flags.Bool("json", false, "print one JSON document on stdout instead of text")
	noMajor := flags.Bool("no-major", false, "the updates: no major moves (packages within their ranges, Go modules within their major)")
	flags.Usage = func() { fmt.Fprint(os.Stderr, help) }
	flags.Parse(args)
	if flags.NArg() > 0 {
		flags.Usage()
		os.Exit(2)
	}
	// A flag a command doesn't take is refused, not ignored: the tasks pass only their own, and so must anyone else.
	takes := map[string][]string{"": {"json"}, "show": {"json"}, "check": {"json"}, "show-users": {"json"}, "show-version": {"json"},
		"update": {"json", "no-major"}, "update-mise": {"json", "no-major"}, "update-packages": {"json", "no-major"}, "update-go": {"json", "no-major"}}
	flags.Visit(func(f *flag.Flag) {
		if !slices.Contains(takes[command], f.Name) {
			fmt.Fprintf(os.Stderr, "mise-check %s takes no --%s (mise-check show-help: every command and its flags)\n", command, f.Name)
			os.Exit(2)
		}
	})
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
	case "update", "update-mise", "update-packages", "update-go":
		if inCI() {
			fail(fmt.Errorf("in CI %s doesn't run: CI only checks the repo, and %s changes its pins; run mise run mise-check:%s on your machine, then mise run mise-check:verify", command, command, command))
		}
		o := updates{json: *jsonOut, noMajor: *noMajor}
		if o.json {
			out = os.Stderr
		}
		switch command {
		case "update":
			r.update(o)
		case "update-mise":
			r.updateMise(o)
		case "update-packages":
			r.updatePackages(o)
		case "update-go":
			r.updateGo(o)
		}
	case "show-users":
		r.showUsers(*jsonOut)
	case "verify":
		r.verify()
	case "template":
		r.template()
	case "show-version":
		showVersion(*jsonOut)
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
