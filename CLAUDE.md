# liteChat

Go backend (`internal/`, single binary, SQLite) + Svelte 5 frontend (`web/`) embedded at build time.

## Commands

- `make build`: lint UI, build frontend, build `./litechat`
- `make test`: `go vet`, Go tests, frontend type-check, design-system lint
- `make dev`: backend on :3000 + Vite on :5173
- Go lives in `~/.local/go/bin` (add it to PATH in each shell)

## UI rules (mandatory for every frontend change)

All UI must follow the design system in `web/DESIGN.md`:

- Build from the kit in `web/src/lib/ui` (Button, IconButton, TextField, Menu, Dialog, List/ListRow, Section…). No raw `<button>/<input>/<select>/<textarea>` in feature components.
- Only design tokens exist: colors `canvas panel raised hover line line-strong ink ink-2 ink-3 pine pine-ink pine-wash red red-wash`, type `text-xs/sm/base/lg/xl`, radius `rounded-sm/md/lg`, shadow `shadow-overlay`. No arbitrary `[...]` values, hex colors, or Tailwind palette colors in feature code.
- Sentence case; no all-caps labels; no "·"-joined meta strings.
- Missing a component or value? Add it to `web/src/lib/ui` or `web/src/app.css`, and show it on the `/_ui` reference page.
- `npm run lint:ui` must pass (it also runs inside `npm run build`).
