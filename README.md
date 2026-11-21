# liteChat

A lightweight, self-hosted chat UI for LLMs. One ~14 MB binary, one SQLite file, no runtime dependencies.

- **Any provider**: anything OpenAI-compatible (Ollama, llama.cpp, LM Studio, vLLM, OpenRouter, Groq, OpenAI…) plus native Anthropic. Models are discovered automatically.
- **Streaming** with smooth incremental markdown, syntax highlighting, KaTeX math and collapsible "thinking" sections (highlighter and math load only when needed).
- **Branching**: edit a message or regenerate a reply, then flip between versions.
- **Images & files**: paste, drop or attach images (downscaled client-side) and text/code files.
- Full-text **search** (Ctrl+K), pinned chats, auto titles, system-prompt presets, light/dark themes, mobile layout, installable as a PWA.
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

Without `LITECHAT_PASSWORD`, the first visit asks you to choose a password.

Docker:

```sh
docker build -t litechat .
docker run -p 3000:3000 -v litechat:/data litechat
```

To reach Ollama on the host from Docker, use `http://host.docker.internal:11434/v1` (add `--add-host=host.docker.internal:host-gateway` on Linux).

## Develop

```sh
make dev        # Go backend on :3000 + Vite with hot reload on :5173 (open :5173)
```

## Layout

```
main.go                 entrypoint; embeds web/dist
internal/db             SQLite schema + queries (messages form a tree for branching)
internal/llm            provider adapters (openai.go, anthropic.go) → plain text stream
internal/server         HTTP API, auth, SSE streaming, static files
web/src/lib             state (Svelte 5 runes), API client, markdown renderer
web/src/lib/components  UI
```

## Security notes

- Behind a reverse proxy, terminate TLS there; cookies are marked `Secure` when `X-Forwarded-Proto: https` is set.
- If you expose it publicly, keep in mind that provider base URLs are fetched by the server.
