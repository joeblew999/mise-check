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

// command is one command of mise-check as its task runs it: the task, what the command does and when to run it, each
// one short line. This table is the one source of those lines: show-help prints them, and TestCommands holds the
// template's tasks file (each task's description is what it does), its skill and its docs page (each task's row is
// both) to them, word for word.
type command struct{ task, does, when string }

// commands are every command, as the template's tasks file has them, in its order.
var commands = []command{
	{"mise-check", "what's done for the repo, and what to run next", "first, and whenever you're unsure"},
	{"mise-check:init", "writes mise-check's files again: its tasks, the workflow, the skill, the docs, the hook", "after upgrading mise-check"},
	{"mise-check:check", "the repo's rules: its mise files by misecheck, its skills against their tasks, mise-check's files current, its pin", "after any change to a mise file or a skill; when it fails, fix the side it names"},
	{"mise-check:show", "the same as mise run mise-check", "to see where the repo is"},
	{"mise-check:verify", "what GitHub runs: each pin consistent with what pins it, the repo's setup and test, then check", "before every push"},
	{"mise-check:update", "every pin at its newest: the three below, in order", "when the person asks to upgrade everything"},
	{"mise-check:update-mise", "the tools in the repo's mise files", "to upgrade the mise tools only"},
	{"mise-check:update-packages", "package.json and its lockfile, through aube", "to upgrade the Node packages only"},
	{"mise-check:update-go", "every go.mod's toolchain line and modules, majors too, their import paths rewritten", "to upgrade Go only"},
	{"mise-check:show-users", "which of the owner's repos use it, at what version, the behind ones marked", "to see which of the owner's repos use this one, and which are behind"},
	{"mise-check:template", "the template's little app, its go.mod and packages; it never writes over a file", "only in a repo with no Go program yet"},
	{"mise-check:show-version", "its version", "to say which mise-check this is"},
	{"mise-check:show-help", "every command and its flags", "to list every command"},
}

// help is show-help's text: every command, from commands, and the flags.
var help = func() string {
	var b strings.Builder
	b.WriteString("This repo's pins and mise tasks, with mise-check. Each command is a task, mise run mise-check:<command>:\n\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "  %-38s %s\n", "mise run "+c.task, c.does)
	}
	b.WriteString("\nThe updates take --no-major: no major moves. --json, one JSON document on stdout instead of text: mise-check,\nshow, check, the updates, show-users, show-version. Start with mise run mise-check: it says what to run next.\n")
	return b.String()
}()

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
