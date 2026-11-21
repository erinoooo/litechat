# Changelog

All notable changes to liteChat. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.0] - 2026-11-21

### Added

- Install with Python tooling: `pipx install litechat-py`, `uv tool install litechat-py` or `pip install litechat-py`. The wheels contain the same binary as the release downloads.

### Changed

- The default data folder is now per user (`~/.local/share/litechat` on Linux, `~/Library/Application Support/litechat` on macOS, `%LocalAppData%\litechat` on Windows), so `litechat` finds your chats from any directory. A `data` folder that already holds a database in the current directory is still used, and `-data` / `LITECHAT_DATA` still override it. The startup log shows which folder is in use.

## [0.1.0] - 2026-11-21

First public release.

### Added

- Chat with OpenAI-compatible servers (Ollama, llama.cpp, LM Studio, vLLM, OpenRouter, Groq, OpenAI…) and Anthropic, with automatic model lists.
- Detection of local AI servers on this machine, plus an optional local network scan, with one-click setup.
- Replies generate on the server: they survive closed tabs and reloads, and any open page re-attaches.
- Branching: edit messages, regenerate replies, switch between versions.
- Tools through MCP servers (remote over HTTP, or local commands with `LITECHAT_MCP_STDIO=1`).
- Attachments: images, PDF, DOCX, text and code files.
- Full-text search, and optional search by meaning with an embedding model.
- Import from ChatGPT, Open WebUI and liteChat; export to JSON or Markdown.
- Reasoning sections, code highlighting, math, system-prompt presets, light and dark themes, mobile layout, installable web app.
- Single-password login with rate limiting; API keys never leave the server.
- Notice when a model stops at its length limit, with how to fix it.
- Binaries for Linux, macOS, Windows and FreeBSD; multi-arch Docker image.

[Unreleased]: https://github.com/erinoooo/litechat/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/erinoooo/litechat/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/erinoooo/litechat/releases/tag/v0.1.0
