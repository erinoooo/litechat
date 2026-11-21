# Search, import and export

## Search

Press **Ctrl+K** (⌘K on a Mac), or click the search icon in the sidebar. With an empty search box you see recent chats; type to search the text of every message and chat title. Arrow keys move, Enter opens.

### Search by meaning

Word search only finds chats containing what you typed. Search by meaning also finds chats about the same thing in other words: "cheap balloon flights" finds a chat about a "budget-friendly balloon trip".

Turn it on in **Settings → Search and data**:

1. Pick an OpenAI-compatible provider. Anthropic has no embeddings API.
2. Enter an embedding model, for example `nomic-embed-text` on Ollama (`ollama pull nomic-embed-text`) or `text-embedding-3-small` on OpenAI.
3. Click **Turn on**.

liteChat then embeds each reply together with its question and the chat title, in the background, and keeps the vectors in the same database. New replies are added as they finish. Search results show a **Related by meaning** group under the exact matches.

Switching to another embedding model rebuilds the index. Search compares against every stored vector, which stays fast up to tens of thousands of messages.

## Export

- **All chats**: Settings → Search and data → **Export all chats** downloads a JSON file with every chat, including all edited and regenerated versions.
- **One chat**: in the sidebar, open a chat's menu → **Download as Markdown** saves the current version of the conversation.

## Import

Settings → Search and data → **Import chats** accepts:

| From | File |
|---|---|
| liteChat | The JSON from **Export all chats** |
| ChatGPT | `conversations.json` from ChatGPT's data export |
| Open WebUI | The JSON from Open WebUI's chat export |

Edited and regenerated versions are kept as branches. Imported chats are added alongside existing ones; importing the same file twice creates duplicates.

## Backups

Everything (chats, settings, providers, API keys, the search index) is in `data/litechat.db`. To back it up while liteChat is running, use SQLite's online backup:

```sh
sqlite3 data/litechat.db ".backup litechat-backup.db"
```

Or stop liteChat and copy the `data` folder. The database contains your API keys, so keep backups private.
