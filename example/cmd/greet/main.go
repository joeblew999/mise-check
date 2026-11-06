// Command greet is the example's tool, the reference for a tool whose users are Go repos. A repo gets it from its
// module, with one line of its go.mod (go get -tool github.com/joeblew999/mise-check/example/cmd/greet), and runs it
// as go tool greet <command>: the version go.mod pins, checked by go.sum, the same version as the module half a Go
// program imports (github.com/joeblew999/mise-check/example/greet). No mise is needed for any of it. A repo that uses
// mise gets each command as a task too, numbered in the order a repo meets them (.mise/tasks/greet.toml, written by
// greet init): mise run greet:<nn>-<command> runs go tool greet <command>. greet show-help lists the commands.
//
// Each command is a file of its own, holding the function named for it (check.go holds check), so misecheck can hold
// every task to the Go it runs, both ways.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
)

// module is the module greet comes from, and its tool line: the one line of a repo's go.mod that pins both halves.
const (
	module = "github.com/joeblew999/mise-check"
	tool   = module + "/example/cmd/greet"
)

const help = `greet, the example of mise-check's reference for a tool whose users are Go repos. Every command runs as
go tool greet <command>; with mise, each is a task too, numbered in the order a repo meets them.

The steps, in order (go tool greet, or mise run greet, says which is next):
  go tool greet init       03  greet's files written into the repo (again, after upgrading)
  go tool greet check      04  the repo's rules: greet's files current, go.mod's tool line
  go tool greet show       05  what's done, and what to run next (go tool greet, too)
  go tool greet hello      06  greet someone (--name you --shout)
  go tool greet bye        07  goodbye

And:
  go tool greet verify         what the repo's GitHub workflow runs: its setup and test tasks, then check
  go tool greet template       the template's little app, in a repo with no Go program yet: it never writes over a file
  go tool greet show-version   this build's version
  go tool greet show-help      this

With mise: mise run greet:03-init … greet:07-bye, greet:verify, greet:template.

To upgrade greet, upgrade its module, as any Go module: go get -tool github.com/joeblew999/mise-check/example/cmd/greet@latest,
then go tool greet init (check says when its files are out of date).
`

func main() {
	// The command first, then its flags. hello has its own.
	args := os.Args[1:]
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}
	if command == "hello" {
		hello(args)
		return
	}
	flags := flag.NewFlagSet("greet "+command, flag.ExitOnError)
	jsonOut := flags.Bool("json", false, "print one JSON document on stdout instead of text")
	flags.Usage = func() { fmt.Fprint(os.Stderr, help) }
	flags.Parse(args)
	if flags.NArg() > 0 {
		flags.Usage()
		os.Exit(2)
	}
	dir, err := os.Getwd()
	must(err)
	switch command {
	case "", "show":
		show(dir, *jsonOut)
	case "init":
		initRepo(dir)
	case "check":
		check(dir, *jsonOut)
	case "bye":
		bye(nil)
	case "verify":
		verify(dir)
	case "template":
		template(dir)
	case "show-version":
		showVersion()
	case "show-help":
		showHelp()
	default:
		fmt.Fprint(os.Stderr, help)
		os.Exit(2)
	}
}

// goMod is the go.mod of the module dir is in, as go reads it (a folder of the tool's own checkout is in the tool's
// module), and its text; "" when dir is in none.
func goMod(dir string) (path string, text string) {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	path = strings.TrimSpace(string(out))
	if err != nil || path == "" || path == os.DevNull {
		return "", ""
	}
	b, err := os.ReadFile(path)
	must(err)
	return path, string(b)
}

// hasTool reports whether a go.mod's text has greet's tool line, in a tool block or on its own.
func hasTool(gomod string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(gomod, "\r\n", "\n"), "\n") {
		f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "tool "))
		if len(f) == 1 && f[0] == tool {
			return true
		}
	}
	return false
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(v))
}

func must(err error) {
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "greet:", err)
	os.Exit(1)
}

// buildVersion is the module version greet was built at, as go.mod pins it (go tool greet builds exactly that), or
// "(devel)" for a build of the tool's own checkout.
func buildVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return "(devel)"
}

// writeFile writes b to path, saying so, unless path has exactly b already.
func writeFile(path string, b []byte) {
	if old, err := os.ReadFile(path); err == nil && string(old) == string(b) {
		fmt.Printf("✓ %s is up to date\n", path)
		return
	}
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	must(os.WriteFile(path, b, 0o644))
	fmt.Println("wrote", path)
}

// trustConfig has mise trust a config file greet wrote, so its tasks run straight away, when mise is there; a repo
// without mise needs none of it.
func trustConfig(dir, file string) {
	path := filepath.Join(dir, filepath.FromSlash(file))
	if _, err := os.Stat(path); err != nil {
		return
	}
	if _, err := exec.LookPath("mise"); err != nil {
		return
	}
	if out, err := exec.Command("mise", "trust", "-q", path).CombinedOutput(); err != nil {
		fmt.Printf("mise didn't trust %s (%v %s): run mise trust %s\n", file, err, strings.TrimSpace(string(out)), file)
		return
	}
	fmt.Printf("✓ %s trusted by mise\n", file)
}

// errNoModule is what a command that needs the repo's go.mod says in a folder with none.
var errNoModule = errors.New("no go.mod here: greet comes from a Go module, so the repo needs one (go mod init <module>, then go get -tool " + tool + "@latest)")
