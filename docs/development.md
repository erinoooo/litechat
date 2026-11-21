# Development

## Setup

Needs Go 1.27+ and Node 24+.

```sh
make dev     # Go backend on :3000, Vite with hot reload on :5173; open :5173
make test    # go vet, Go tests, frontend type-check, design-system lint, Python package tests
make test-py # only the Python package tests (python/), against a fresh binary
make build   # production binary ./litechat with the frontend embedded
```

## Layout

```
main.go                   entrypoint; embeds web/dist into the binary
internal/db               SQLite schema and queries; messages form a tree for branching
internal/llm              provider adapters (OpenAI-compatible, Anthropic): streaming, tool calls, embeddings
internal/mcp              MCP client (Streamable HTTP and stdio) and connection manager
internal/discover         local AI server detection
internal/server           HTTP API, auth, background generation (hub.go, stream.go), tools loop,
                          file extraction, import/export, semantic indexer
web/src/lib/state.svelte.ts   app state (Svelte 5 runes)
web/src/lib/api.ts            API client and SSE parsing
web/src/lib/markdown.ts       streaming-friendly markdown rendering
web/src/lib/components        screens and features
web/src/lib/ui                the UI kit
```

## How a reply works

1. `POST /api/chats/{id}/messages` stores the user message and an empty assistant message, then starts a **generation** in a goroutine that doesn't depend on the request.
2. The generation streams from the provider, runs any tool calls, saves progress every two seconds, and publishes events to a hub.
3. The request (and any later `GET …/stream`) subscribes to the hub and forwards events as SSE. Disconnecting only unsubscribes.
4. When done, the reply is saved, the chat is titled if it's new, and the semantic indexer is nudged.

## The design system

The UI follows a small design system: read [`web/DESIGN.md`](../web/DESIGN.md) before changing the frontend.

- Tokens (color, type, radius, motion) are in `web/src/app.css`. Tailwind's default palette and scales are removed, so only tokens exist.
- Components are in `web/src/lib/ui`. Feature code builds from these instead of raw `<button>` or `<input>` elements.
- The live reference is at `/_ui` in a running app.
- `npm run lint:ui` (part of `make test` and every build) rejects off-system styling. For a justified exception, put `<!-- ui-allow: reason -->` on the line above.

## Releasing

1. Add a section for the version to `CHANGELOG.md`: `## [0.2.0] - YYYY-MM-DD`.
2. Tag and push: `git tag v0.2.0 && git push origin v0.2.0`.
3. The release workflow builds binaries for every platform, publishes a GitHub release with checksums and the changelog section as notes, and pushes the Docker image to `ghcr.io/erinoooo/litechat`.
