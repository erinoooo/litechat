# liteChat

A lightweight, self-hosted chat UI for LLMs. One ~15 MB Go binary, one SQLite file, no runtime dependencies, ~25 MB of RAM. Built for headless machines: run it anywhere, use it from any browser.

- **Any provider**: anything OpenAI-compatible (Ollama, llama.cpp, LM Studio, vLLM, OpenRouter, Groq, OpenAI…) plus native Anthropic. Models are discovered automatically.
- **Streaming** with smooth incremental markdown, syntax highlighting, KaTeX math and collapsible "thinking" sections (highlighter and math load only when needed).
- **Branching**: edit a message or regenerate a reply, then flip between versions.
- **Replies run on the server**: close the tab, lock your phone or reload mid-answer; the reply keeps going and the UI re-attaches. Stop works from any device.
- **Tools via MCP**: connect remote (HTTP) or local (stdio) [MCP](https://modelcontextprotocol.io) servers; the model can call them in a loop, and each call is shown as an expandable card.
- **Attachments**: images (downscaled client-side), PDF and DOCX (text extracted on the server), and any text/code file.
- **Search**: full-text (Ctrl+K), plus optional **search by meaning**: an embedding model indexes every exchange so you can find a chat by what it was about, not the exact words.
- **Import/export**: export everything as JSON or a chat as Markdown; import liteChat, ChatGPT (`conversations.json`) and Open WebUI exports with branches intact.
- Pinned chats, auto titles, system-prompt presets, light/dark themes, mobile layout, installable as a PWA.
- **Single password** login; API keys stay on the server.

## Run

```sh
make            # builds web/ and embeds it into ./litechat
./litechat      # http://localhost:3000, data in ./data
```

Options (flag / env):

| Flag | Env | Default |
|---|---|---|
| `-addr` | `LITECHAT_ADDR` | `:3000` |
| `-data` | `LITECHAT_DATA` | `./data` |
| | `LITECHAT_PASSWORD` | unset; sets/resets the password at startup |
| | `LITECHAT_MCP_STDIO` | unset; `1` allows local (stdio) MCP servers, which run commands on the host |

Without `LITECHAT_PASSWORD`, the first visit asks you to choose a password.

Docker:

```sh
docker build -t litechat .
docker run -p 3000:3000 -v litechat:/data litechat
```

To reach Ollama on the host from Docker, use `http://host.docker.internal:11434/v1` (add `--add-host=host.docker.internal:host-gateway` on Linux).

## Search by meaning

Settings → Search & data → pick an OpenAI-compatible provider and an embedding model
(e.g. `nomic-embed-text` on Ollama: `ollama pull nomic-embed-text`). Every reply is
embedded in the background together with its question and the chat title; vectors are
stored in the same SQLite file and searched by brute-force cosine similarity, which stays
fast up to tens of thousands of messages. Ctrl+K then shows "Related by meaning" results
alongside exact matches. Switching models rebuilds the index.

## Tools (MCP)

Settings → Tools. Remote servers need a URL (and optional headers such as
`Authorization`). Local servers are a command plus arguments, e.g. `npx` with
`-y`, `@modelcontextprotocol/server-filesystem`, `/srv/notes`; these require
`LITECHAT_MCP_STDIO=1` and whatever runtime the server needs (Node, Python…) on the host.
The minimal Docker image has no such runtimes, so use remote servers there or build on a
fuller base image. Toggle tools per message with the **Tools** button in the composer.

## Develop

```sh
make dev        # Go backend on :3000 + Vite with hot reload on :5173 (open :5173)
make test       # Go tests, frontend type-check, design-system lint
```

The UI follows a small design system: tokens in `web/src/app.css`, components in
`web/src/lib/ui`, rules in [`web/DESIGN.md`](web/DESIGN.md), and a live reference at
`/_ui`. `npm run lint:ui` (part of every build) rejects off-system styling.

## Layout

```
main.go                 entrypoint; embeds web/dist
internal/db             SQLite schema + queries (messages form a tree for branching)
internal/llm            provider adapters (openai.go, anthropic.go): text stream + tool calls, embeddings
internal/mcp            MCP client (Streamable HTTP + stdio) and connection manager
internal/server         HTTP API, auth, background generation hub (hub.go, stream.go),
                        tools loop, file extraction, import/export, semantic indexer
web/src/lib             state (Svelte 5 runes), API client, markdown renderer
web/src/lib/components  UI
```

## Security notes

- Behind a reverse proxy, terminate TLS there; cookies are marked `Secure` when `X-Forwarded-Proto: https` is set.
- If you expose it publicly, keep in mind that provider base URLs and MCP server URLs are fetched by the server.
- Only enable `LITECHAT_MCP_STDIO=1` if you trust everyone who knows the password: it lets the UI start processes on the host.
