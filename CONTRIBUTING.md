# Contributing

Thanks for helping. liteChat aims to stay small and fast, so the best changes do a lot with little code and few dependencies.

## Before you start

- For anything bigger than a small fix, open an issue first to agree on the approach.
- New dependencies need a good reason, in Go and especially in the frontend bundle.

## Working on it

See [docs/development.md](docs/development.md) for setup and the code layout.

```sh
make dev    # run with hot reload
make test   # must pass before you open a pull request
```

- Frontend changes follow the design system in [web/DESIGN.md](web/DESIGN.md): build from `web/src/lib/ui`, use tokens only, and keep `npm run lint:ui` passing. Check your change in light and dark mode and at phone width.
- Go code is `gofmt`-formatted and covered by tests where it's practical; provider adapters and parsers especially.
- Interface text is sentence case and says plainly what things do. Errors say what happened and how to fix it.
- Add a line to the `Unreleased` section of [CHANGELOG.md](CHANGELOG.md) for user-visible changes.

## Pull requests

Keep each one focused on one change, explain what and why, and include a screenshot for visible changes.
