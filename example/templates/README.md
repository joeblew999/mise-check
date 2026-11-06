# greet-template

A Go program with [greet](https://github.com/joeblew999/mise-check/tree/main/example), the example tool of
mise-check's reference: `go tool greet` (or `mise run greet`) shows what's done, and what to run next.

- `go run .` (`mise run start`): the app, which imports greet's package.
- `go test ./...` (`mise run test`): its tests.
- [docs/greet/README.md](docs/greet/README.md): greet in this repo, every command, and how to upgrade it.

greet and its module come from one line of `go.mod`, the only thing the repo needs for them: mise is optional.
