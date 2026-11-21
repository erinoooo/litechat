# HTTP API

The web app uses a small JSON API, which you can also call from scripts. It isn't versioned yet, so it may change between releases. From Python, the [`litechat` package](python.md) wraps it.

## Authentication

Log in to get a session cookie, then send it with every request. Requests other than `GET` must also include the header `X-LiteChat: 1` (this blocks cross-site form attacks).

```sh
curl -c cookies -H 'X-LiteChat: 1' -H 'Content-Type: application/json' \
  -d '{"password":"…"}' http://localhost:3000/api/auth/login

curl -b cookies http://localhost:3000/api/chats
```

| Method | Path | |
|---|---|---|
| `GET` | `/api/auth/status` | `{setup, authed}` |
| `POST` | `/api/auth/setup` | First run only: `{password}` |
| `POST` | `/api/auth/login` | `{password}` |
| `POST` | `/api/auth/logout` | |
| `POST` | `/api/auth/password` | `{current, new}`; logs out other sessions |

## Models and providers

| Method | Path | |
|---|---|---|
| `GET` | `/api/models` | `{models: [{key, id, name, provider_id, provider_name}], errors}`. `?refresh=1` skips the cache |
| `GET` | `/api/providers` | List providers (keys are never returned) |
| `POST` | `/api/providers` | `{name, kind: "openai"\|"anthropic", base_url, api_key}` |
| `PUT` | `/api/providers/{id}` | Same fields, all optional. Omit `api_key` to keep it |
| `DELETE` | `/api/providers/{id}` | |
| `GET` | `/api/discover` | Detected local servers. `?network=1` also scans the LAN |

A model is referred to by its **key**: `<provider_id>/<model_id>`.

## Chats

| Method | Path | |
|---|---|---|
| `GET` | `/api/chats` | List chats |
| `POST` | `/api/chats` | `{model?, system_prompt?, title?}` |
| `GET` | `/api/chats/{id}` | `{chat, messages, generating}`. Messages form a tree through `parent_id`; `chat.leaf_id` is the end of the active branch |
| `PATCH` | `/api/chats/{id}` | Any of `{title, system_prompt, model, leaf_id, pinned}` |
| `DELETE` | `/api/chats/{id}` | |
| `POST` | `/api/chats/{id}/messages` | Send a message and stream the reply (below) |
| `GET` | `/api/chats/{id}/stream` | Re-attach to a reply in progress; `204` if none |
| `POST` | `/api/chats/{id}/stop` | Stop the reply in progress |
| `GET` | `/api/search?q=` | Full-text search |
| `GET` | `/api/semantic/search?q=` | Search by meaning (when enabled) |

### Sending a message

```sh
curl -N -b cookies -H 'X-LiteChat: 1' -H 'Content-Type: application/json' \
  -d '{"parent_id":"", "content":"Hello", "model":"<provider_id>/<model_id>"}' \
  http://localhost:3000/api/chats/<chat_id>/messages
```

| Field | |
|---|---|
| `parent_id` | Message to reply under: the last message for a normal reply, `""` for the first message |
| `content` | Text |
| `images` | Optional list of `data:` URLs |
| `model` | Model key |
| `regenerate` | `true` to write a new reply to the user message given as `parent_id` (omit `content`) |
| `tools` | `true` to offer MCP tools |

The response is a stream of server-sent events:

| Event | Data |
|---|---|
| `meta` | `{user, assistant}`: the stored messages |
| `delta` | `{t}`: the next piece of text |
| `tool_start` | `{name, server, args}`: a tool is running |
| `done` | `{content, error?}`: the reply is complete and saved |
| `title` | `{title}`: the chat got a title (first reply only; may arrive twice) |

The reply keeps generating if the connection drops; re-attach with `GET /api/chats/{id}/stream`, which starts with a `sync` event holding the text so far.

Reasoning appears in the text as `<think>…</think>`, and tool calls as `<tool name="…" server="…" status="ok|error">` blocks containing JSON.

## Other

| Method | Path | |
|---|---|---|
| `POST` | `/api/extract` | Multipart `file`: returns `{name, text, truncated}` for PDF, DOCX or text |
| `GET` | `/api/export` | Every chat as JSON |
| `POST` | `/api/import` | Body: a liteChat, ChatGPT or Open WebUI export |
| `GET`, `PUT` | `/api/settings` | App preferences |
| `GET`, `POST`, `PUT`, `DELETE` | `/api/presets[/{id}]` | Saved system prompts |
| `GET`, `POST`, `PUT`, `DELETE` | `/api/mcp[/{id}]` | MCP servers; `GET /api/mcp/status` connects and lists tools |
| `GET` | `/api/semantic/status` | Indexing progress |
