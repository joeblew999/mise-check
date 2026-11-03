# The example

A small repo that uses everything misecheck checks. Run these here, in `example/`:

```sh
mise run greet:hello --name you --shout   # greet's hello command: HELLO, YOU
mise run greet:test                       # Go that runs the tasks back by name: the loop, end to end
mise run greet:check-mise                 # misecheck on this very file, the strict way
```

## What each part shows

| File | What it is | What misecheck holds it to |
|---|---|---|
| [mise.toml](mise.toml) | The interface: one task per thing to do, each one line | Every `run` and `usage` one line; no shell syntax; each flag in `usage` passed by name (`{% if usage.shout %} --shout{% endif %}`) |
| [cmd/greet/](cmd/greet/) | The binary the tasks run, `greet`, its commands a file each: `hello.go` holds `hello`, `bye.go` holds `bye` | Each command a task runs is a file here; each file here is run by a task (`GO_LAYOUT=file`) |
| `hello.go`'s flags | `--name`, `--shout` (`fs.String("name", …)`) | The same flags in `greet:hello`'s `usage`, no more and no fewer |
| [greet_test.go](greet_test.go) | Go that runs the tasks back by name: `mise run greet:hello --name misecheck` | Every task name it uses is a task of mise.toml, written in full (`GO_CALLERS=.`) |
| [mise_test.go](mise_test.go) | All a repo writes to use misecheck: `misecheck.Check(t, misecheck.FromEnv())` | Run by `greet:check-mise`, whose `env` says what to check: this very file |
| `greet:build` | Go from its source: `go build -o bin/ ./cmd/greet` | The folder holds Go |
| `greet:readme` | Anything else (`echo …`) | Allowed, logged as not checked |

So it goes both ways: `greet:hello` runs Go (`greet hello`), `greet_test.go` runs `greet:hello` back, and
`greet:check-mise`, a task of this same file, checks every one of those links.

## Break it

misecheck's own tests (`TestBroken`, in `../misecheck_test.go`) break a copy of this folder one way at a time and check
that each is caught. Try one here and run `mise run greet:check-mise`:

1. Rename `[tasks."greet:bye"]`: `greet_test.go` still runs `greet:bye`.
2. Delete `cmd/greet/bye.go`: `greet:bye` still runs it.
3. Add `fs.Bool("loud", …)` to `hello.go`: no task declares `--loud`.
4. Add `flag "--color"` to `greet:hello`'s `usage`: greet's Go has no such flag.
5. Take `{% if usage.shout %} --shout{% endif %}` out of `greet:hello`'s run: `--shout` is declared and never passed.
6. Write `GREETING=1 greet bye`: Windows' cmd doesn't read it.
7. Spread a run over several lines: logic that belongs in Go.
8. Add `cmd/greet/wave.go` with `func wave`: a command no task runs.
9. Write `"greet:" + step` in Go: a task name that can't be checked.
