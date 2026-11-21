# liteChat

A lightweight, self-hosted chat UI for local and cloud LLMs. It runs as one small binary with one SQLite file: no Python, Node or database server at runtime. This package installs that binary as the `litechat` command.

![liteChat](https://raw.githubusercontent.com/erinoooo/litechat/main/docs/screenshots/chat-dark.png)

## Install and run

```sh
pipx install litechat-py      # or: uv tool install litechat-py, or pip install litechat-py
litechat
```

Or run it once without installing: `uvx --from litechat-py litechat`.

Open http://localhost:3000 and choose a password. liteChat finds AI servers running on the machine, such as Ollama, llama.cpp or LM Studio, and adds them in one click. You can also add OpenAI-compatible APIs and Anthropic.

Options: `--port 8080`, `--host 127.0.0.1` (this machine only), `--data <folder>`, `--password <pw>`; `litechat --help` lists them all. Chats are stored in `~/.local/share/litechat` on Linux, `~/Library/Application Support/litechat` on macOS and `%LocalAppData%\litechat` on Windows, unless you pass `--data`.

Wheels are available for Linux (x86-64, ARM64, ARMv7), macOS 12+ (Intel and Apple Silicon) and Windows (x86-64, ARM64). For other platforms, use a [release download](https://github.com/erinoooo/litechat/releases) or the Docker image.

## Use it from Python

```python
import litechat

# run a server in the background (127.0.0.1, any free port)...
with litechat.Server(password="secret") as server:
    client = server.client()
    client.add_provider("Ollama", "http://localhost:11434/v1")

    print(client.ask("Why is the sky blue?", model="qwen3.5:4b"))

    chat = client.new_chat(model="qwen3.5:4b")
    for piece in chat.stream("Tell me a story"):
        print(piece, end="", flush=True)

# ...or connect to one that's already running
client = litechat.Client("http://localhost:3000", password="secret")
```

Chats made from Python show up in the web app, and the other way round. The library uses only the standard library. Full guide: [docs/python.md](https://github.com/erinoooo/litechat/blob/main/docs/python.md).

The import name is `litechat`; an unrelated PyPI package is also called `litechat`, so don't install both in one environment.

## Features

- Streaming chat with branching: edit messages, regenerate replies, switch between versions
- Replies generate on the server, so they survive closed tabs
- Tools through MCP servers
- Images, PDF, DOCX and code attachments
- Full-text search, and optional search by meaning
- Import from ChatGPT and Open WebUI; export to JSON or Markdown

Documentation, Docker setup and source code: https://github.com/erinoooo/litechat
