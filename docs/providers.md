# Providers

A provider is a server that runs models. liteChat talks to two kinds of API:

- **OpenAI-compatible**: Ollama, llama.cpp (and llamafile), LM Studio, vLLM, SGLang, KoboldCpp, TabbyAPI, LocalAI, Jan, GPT4All, LiteLLM, OpenRouter, Groq, Together, OpenAI and most others. The base URL usually ends in `/v1`.
- **Anthropic**: Claude models through the Anthropic API.

Add providers in **Settings → Providers**. Models are listed automatically and refreshed every few minutes; use **Refresh model list** in the model menu to fetch them right away. API keys are stored on the server and never sent to the browser.

## Detected servers

liteChat checks this machine for running AI servers whenever you open the Providers page (and when the model menu is empty). Each one found is listed with its address and models, and **Add** sets it up in one click.

It probes these default ports and identifies the server by its own API:

| Port | Usually |
|---|---|
| 11434 | Ollama |
| 1234 | LM Studio |
| 8080 | llama.cpp, llamafile, LocalAI, MLX |
| 8000, 8001 | vLLM |
| 5000 | Text Generation WebUI, TabbyAPI |
| 5001 | KoboldCpp |
| 1337 | Jan |
| 4891 | GPT4All |
| 30000 | SGLang |
| 2242 | Aphrodite |
| 4000 | LiteLLM |

Anything else on those ports that answers `GET /v1/models` shows up as "OpenAI-compatible server". A server on a different port won't be detected, but you can add it manually with **Other**.

When liteChat runs in Docker, it also checks the Docker host (`host.docker.internal`, or the default gateway).

### Scanning your network

**Scan network** checks every address in the local /24 network of each physical network interface (for example 192.168.1.1–254) on the ports above. It takes a couple of seconds and only runs when you click it. Virtual bridges (Docker, libvirt, LXC, VPN tunnels) are skipped.

## Reasoning models and length limits

Reasoning ("thinking") models stream their reasoning first. liteChat shows it in a **Thinking** panel while the model works, then folds it into a **Reasoning** row above the answer.

liteChat recognizes reasoning in three forms: sent separately by the server (`reasoning_content` or `reasoning`, as Ollama, llama.cpp and vLLM do when they parse it out), wrapped in `<think>…</think>` in the text, or only closed with `</think>`. The last form comes from models such as GLM, whose chat template opens `<think>` in the prompt, served without a reasoning parser. For those, the first reply's reasoning shows as text until the `</think>` arrives; after that liteChat knows the model and opens the panel from the first token (until it restarts). Starting the server with its reasoning parser on avoids this entirely: vLLM's `--reasoning-parser`, or llama.cpp's `--jinja` with the default `--reasoning-format`.

If a model stops because it hit its length limit, the reply says so. This is common with reasoning models on Ollama, whose default context window is small: the model can spend all of it reasoning and never reach the answer. Raise it with the `OLLAMA_CONTEXT_LENGTH` environment variable when starting Ollama, for example `OLLAMA_CONTEXT_LENGTH=16384 ollama serve`.

## Default model

Settings → General → **Default model** picks the model for new chats. Without one, liteChat uses the first available model that isn't an embedding model.
