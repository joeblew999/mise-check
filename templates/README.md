# mise-check-template

A repo with [mise-check](https://github.com/joeblew999/mise-check): its pins and mise tasks kept by it.

```sh
mise install                 # node, aube, mise-check, the Go go.mod names
mise run setup               # the packages
mise run mise-check          # what's done, and what to run next
mise run app:start           # the app
mise run mise-check:update   # every pin at its newest release, in one go
```

How mise-check works here: [docs/mise-check/README.md](docs/mise-check/README.md).
