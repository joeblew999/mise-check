# mise-check-template

**The problem.** Write here, in a paragraph, what's hard without this repo, for a person or an agent who has never
seen it.

**What mise-check-template does.** Write here, in a paragraph, how it solves that. Its pins and mise tasks are kept by
[mise-check](https://github.com/joeblew999/mise-check), whose check holds this README to these two openings.

```sh
mise install                 # node, aube, mise-check, the Go go.mod names
mise run setup               # the packages
mise run mise-check          # what's done, and what to run next
mise run app:start           # the app
mise run mise-check:update   # every pin at its newest release, in one go
```

How mise-check works here: [docs/mise-check/README.md](docs/mise-check/README.md).
