"""liteChat from Python: run the server, and talk to it.

Run a server in the background and chat through it::

    import litechat

    with litechat.Server(password="secret") as server:
        client = server.client()
        client.add_provider("Ollama", "http://localhost:11434/v1")
        print(client.ask("Why is the sky blue?", model="qwen3.5:4b"))

Or connect to one that is already running::

    client = litechat.Client("http://localhost:3000", password="secret")
    chat = client.new_chat(model="qwen3.5:4b")
    for piece in chat.stream("Tell me a story"):
        print(piece, end="", flush=True)

Uses only the standard library. The server itself is the bundled
``litechat`` binary; :func:`find_binary` returns its path.
"""

from ._binary import find_binary
from ._client import Chat, Client, Message, Model, Reply, ReplyStream, ToolCall
from ._errors import APIError, AuthError, LiteChatError, ReplyError
from ._server import Server, run, start

try:
    from importlib.metadata import PackageNotFoundError, version as _version

    try:
        __version__ = _version("litechat-py")
    except PackageNotFoundError:
        __version__ = "dev"
except ImportError:  # pragma: no cover
    __version__ = "dev"

__all__ = [
    "APIError",
    "AuthError",
    "Chat",
    "Client",
    "LiteChatError",
    "Message",
    "Model",
    "Reply",
    "ReplyError",
    "ReplyStream",
    "Server",
    "ToolCall",
    "find_binary",
    "run",
    "start",
]
