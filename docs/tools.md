# Tools (MCP)

liteChat can give models tools through the [Model Context Protocol](https://modelcontextprotocol.io). Each MCP server offers a set of tools (web search, files, GitHub, databases…); the model decides when to call them, sees the results, and can call more, up to 10 rounds per reply.

Every call shows in the reply as a row like `search("weather in Ankara")`. Click it to see the input and the result.

## Adding a server

Settings → **Tools** → **Add a server**.

**Remote server**: an MCP server reachable over HTTP (the "Streamable HTTP" transport). Enter its URL, and any headers it needs, such as `Authorization: Bearer …`.

**Local server**: a command liteChat starts and talks to over stdin/stdout, for example:

| Field | Example |
|---|---|
| Command | `npx` |
| Arguments | `-y` / `@modelcontextprotocol/server-filesystem` / `/home/me/notes` (one per line) |
| Environment | `GITHUB_TOKEN=…` |

Local servers run programs on the machine hosting liteChat, so they're off by default. Start liteChat with `--allow-local-mcp` (or `LITECHAT_MCP_STDIO=1`) to allow them, and only do that if you trust everyone who knows the password. The command's runtime (Node for `npx`, Python for `uvx`) must be installed on the host; the official Docker image has neither, so use remote servers there or build your own image on a fuller base.

Header and environment values are stored on the server and shown masked in the browser.

## Using tools

When at least one server is enabled, a **Tools** button appears in the message box. Turn it on to offer tools with your next messages. Each server's status (connected, tool count, or the error) is shown in Settings → Tools; click a server to list its tools.

Models need tool-calling support. Most recent models on Ollama, OpenAI, Anthropic and OpenRouter have it; small local models may ignore tools or call them poorly.

## How results are stored

Tool calls and results are saved inside the reply. In later turns they're passed back to the model as short text summaries (results are cut to 1,500 characters), which keeps long chats affordable.
