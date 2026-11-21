# Python

The [`litechat-py`](https://pypi.org/project/litechat-py/) package installs the `litechat` command and a Python library, imported as `litechat`. With it you can run a liteChat server from Python, and chat through any liteChat server from code. It uses only the standard library and supports Python 3.8+.

```sh
pip install litechat-py
```

> The import name is `litechat`. An unrelated PyPI package is also called `litechat`; don't install both in one environment.

## Quick example

```python
import litechat

with litechat.Server(password="secret") as server:
    client = server.client()
    client.add_provider("Ollama", "http://localhost:11434/v1")
    print(client.ask("Why is the sky blue?", model="qwen3.5:4b"))
```

`add_provider` returns the existing provider when one with the same address is already set up, so the script can run any number of times. Chats made from Python are ordinary liteChat chats: they show up in the web app, and chats from the web app are visible from Python.

## Running a server

### In the background: `Server`

```python
server = litechat.Server(
    port=0,                # 0 = any free port (the default)
    host="127.0.0.1",      # the default: this machine only; "0.0.0.0" for every interface
    data_dir="./chats",    # default: the per-user folder, as for the command
    password="secret",     # sets or resets the password at startup
    allow_local_mcp=False, # allow MCP servers that run local commands
)
server.start()             # returns once the server accepts connections
print(server.url)          # http://127.0.0.1:41235
print(server.port)
client = server.client()   # a Client logged in with the password above
server.stop()
```

`Server` also works as a context manager (`with litechat.Server(...) as server:`), and `litechat.start(**options)` creates and starts one in a single call. A server still running when Python exits is stopped then.

The server's log goes to the `litechat` logger. To see it:

```python
import logging
logging.basicConfig(level=logging.INFO)
```

If the server can't start (for example because the port is taken), `start()` raises `LiteChatError` with the end of its log.

### In the foreground: `run`

`litechat.run(port=8080, password="secret")` runs the server until Ctrl+C, like the `litechat` command, and returns its exit code. Options you leave out use the command's defaults, so it listens on all interfaces, port 3000. `python -m litechat` runs the command with the same options.

## Chatting

### Connecting

```python
client = litechat.Client("http://localhost:3000", password="secret")
```

On a server with no password yet, the first login sets it. A wrong password raises `AuthError`.

### Models

Wherever a model is expected, you can pass its id (`"qwen3.5:4b"`), its display name, or its full key (`"<provider_id>/<model_id>"`, which is unambiguous when two providers offer the same model). Leave it out to use the default model from the app's settings, or failing that the first model available.

```python
for m in client.models():          # cached; client.models(refresh=True) re-fetches
    print(m.key, m.id, m.provider_name)

client.add_provider("Ollama", "http://localhost:11434/v1")   # no duplicate if it already exists
client.add_provider("Anthropic", "https://api.anthropic.com", kind="anthropic", api_key="sk-ant-...")
client.providers()                 # list (API keys are never returned)
client.delete_provider(provider_id)
client.discover()                  # AI servers detected on this machine; network=True scans the LAN
```

### One question

```python
reply = client.ask("Summarise the plot of Hamlet in one line.", model="qwen3.5:4b")
print(reply)            # same as reply.text
```

Each `ask` creates a new chat.

### Conversations

```python
chat = client.new_chat(model="qwen3.5:4b", system_prompt="Answer briefly.")
chat.send("What's the capital of Turkey?")
reply = chat.send("And its population?")      # continues the same conversation
```

To stream a reply as it arrives:

```python
stream = chat.stream("Tell me a story.")
for piece in stream:
    print(piece, end="", flush=True)
print()
print(stream.reply.text)                      # the finished Reply
```

Streamed pieces include reasoning (`<think>…</think>`) and tool blocks as the model writes them. `reply.text` has neither.

`send`, `stream` and `ask` also take:

| Argument | |
|---|---|
| `model=` | Use a different model for this message |
| `images=[...]` | Image file paths, raw bytes (PNG, JPEG, WebP, GIF) or `data:` URLs, for vision models |
| `tools=True` | Let the model call the MCP tools set up in liteChat |

### Replies

| `Reply` | |
|---|---|
| `text` | The answer, without reasoning or tool blocks. `str(reply)` gives the same |
| `reasoning` | The model's reasoning, if it shows any |
| `tool_calls` | List of `ToolCall(name, server, ok, data)` |
| `content` | Everything the model wrote, as stored |
| `message_id`, `model` | Where it is stored, and which model wrote it |

If a reply fails, or stops at the model's length limit, `send`, `ask` and iterating a stream raise `ReplyError`. Its `reply` holds whatever text arrived, and the message explains the cause and how to fix it:

```python
try:
    reply = chat.send("Write a very long essay.")
except litechat.ReplyError as e:
    print("failed:", e)
    partial = e.reply.text
```

### Managing chats

```python
client.chats()                   # every chat: pinned first, then most recently updated
chat = client.chat(chat_id)
chat.messages()                  # the active branch, oldest first: Message(id, role, content, text, ...)
chat.rename("Trip planning")
chat.set_model("qwen3.5:4b")
chat.set_system_prompt("You are a travel agent.")
chat.stop()                      # stop a reply in progress (call from another thread)
chat.delete()

client.search("ankara")          # full-text search across chats
data = client.export()           # every chat as JSON
client.import_chats(data)        # a liteChat, ChatGPT or Open WebUI export
```

### Errors

All exceptions derive from `litechat.LiteChatError`:

| Exception | When |
|---|---|
| `AuthError` | Wrong password, or the session expired |
| `APIError` | The server refused a request; has `status` and `message` |
| `ReplyError` | A reply failed or was cut short; has `reply` |
| `LiteChatError` | The server can't be reached or started, or an unknown model was named |

## Where the binary comes from

The wheel puts the `litechat` binary in your environment's scripts folder, next to `python`, and `Server` and `run` start it from there. `litechat.find_binary()` shows the path. To use another build, such as one you compiled yourself, set `LITECHAT_BINARY=/path/to/litechat` or pass `binary=` to `Server` or `run`.
