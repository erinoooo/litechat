"""Talk to a running liteChat server over its HTTP API."""

from __future__ import annotations

import base64
import http.cookiejar
import json
import mimetypes
import os
import re
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Dict, Iterator, List, Optional, Sequence, Union

from ._errors import APIError, AuthError, LiteChatError, ReplyError

_THINK = re.compile(r"<think>(.*?)(?:</think>|$)", re.S)
_TOOL = re.compile(r'<tool name="([^"]*)" server="([^"]*)" status="([^"]*)">(.*?)</tool>', re.S)
_THINK_TOKENS = re.compile(r"<think>|</think>|\n</tool>")


def _normalize_think(s: str) -> str:
    """Add the <think> that models such as GLM never write (their chat
    template opens it in the prompt), as the server does for new replies."""
    out, prev, open_, seg = [], 0, False, 0
    for m in _THINK_TOKENS.finditer(s):
        tok = m.group()
        if tok == "<think>":
            open_ = True
        elif tok == "</think>":
            if not open_:
                start = seg
                while start < m.start() and s[start] in " \t\r\n":
                    start += 1
                out += [s[prev:start], "<think>"]
                prev = start
            open_, seg = False, m.end()
        elif not open_:
            seg = m.end()
    return "".join(out) + s[prev:] if out else s

Image = Union[str, bytes, "os.PathLike[str]"]


# ---------------------------------------------------------------- data types


@dataclass(frozen=True)
class Model:
    key: str
    """What to pass as ``model=``: ``<provider_id>/<model_id>``."""
    id: str
    name: str
    provider_id: str
    provider_name: str


@dataclass(frozen=True)
class ToolCall:
    name: str
    server: str
    ok: bool
    data: Any


@dataclass(frozen=True)
class Reply:
    """A finished reply. ``str(reply)`` is its text."""

    content: str
    """Everything the model wrote, including <think> and <tool> blocks."""
    message_id: str = ""
    model: str = ""
    error: str = ""

    @property
    def text(self) -> str:
        """The answer, without reasoning or tool blocks."""
        return _TOOL.sub("", _THINK.sub("", _normalize_think(self.content))).strip()

    @property
    def reasoning(self) -> str:
        return "\n\n".join(m.strip() for m in _THINK.findall(_normalize_think(self.content))).strip()

    @property
    def tool_calls(self) -> List[ToolCall]:
        calls = []
        for name, server, status, body in _TOOL.findall(self.content):
            try:
                data = json.loads(body)
            except ValueError:
                data = body.strip()
            calls.append(ToolCall(name, server, status == "ok", data))
        return calls

    def __str__(self) -> str:
        return self.text


@dataclass(frozen=True)
class Message:
    id: str
    role: str
    content: str
    parent_id: str = ""
    model: str = ""
    error: str = ""
    images: List[str] = field(default_factory=list)
    created_at: int = 0

    @property
    def text(self) -> str:
        return Reply(self.content).text if self.role == "assistant" else self.content

    @classmethod
    def _from(cls, d: Dict[str, Any]) -> "Message":
        return cls(
            id=d["id"],
            role=d["role"],
            content=d.get("content", ""),
            parent_id=d.get("parent_id", ""),
            model=d.get("model", ""),
            error=d.get("error", ""),
            images=d.get("images") or [],
            created_at=d.get("created_at", 0),
        )


# ---------------------------------------------------------------- client


class Client:
    """A logged-in connection to a liteChat server.

    >>> client = litechat.Client("http://localhost:3000", password="secret")
    >>> print(client.ask("Why is the sky blue?", model="qwen3.5:4b"))

    On a server that has no password yet, the first login sets it.
    """

    def __init__(self, url: str = "http://localhost:3000", password: Optional[str] = None, *, timeout: float = 30.0) -> None:
        self.url = url.rstrip("/")
        self.timeout = timeout
        self._opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        self._models: Optional[List[Model]] = None
        if password is not None:
            self.login(password)

    # -- auth

    def login(self, password: str) -> None:
        if self._call("GET", "/api/auth/status")["setup"]:
            self._call("POST", "/api/auth/login", {"password": password})
        else:
            self._call("POST", "/api/auth/setup", {"password": password})

    def logout(self) -> None:
        self._call("POST", "/api/auth/logout")

    # -- models and providers

    def models(self, refresh: bool = False) -> List[Model]:
        """Models from every provider."""
        if self._models is None or refresh:
            data = self._call("GET", "/api/models" + ("?refresh=1" if refresh else ""))
            self._models = [Model(**{k: m.get(k, "") for k in Model.__dataclass_fields__}) for m in data.get("models") or []]
        return list(self._models)

    def providers(self) -> List[Dict[str, Any]]:
        return self._call("GET", "/api/providers")

    def add_provider(self, name: str, base_url: str, *, kind: str = "openai", api_key: Optional[str] = None) -> Dict[str, Any]:
        """Add an OpenAI-compatible (``kind="openai"``) or Anthropic (``kind="anthropic"``) provider.

        For Ollama: ``client.add_provider("Ollama", "http://localhost:11434/v1")``.

        If a provider of the same kind and address exists, it is returned
        (with ``api_key`` updated, if given) instead of adding a duplicate, so
        scripts can call this on every run.
        """
        for p in self.providers():
            if p.get("kind") == kind and p.get("base_url", "").rstrip("/") == base_url.rstrip("/"):
                if api_key is not None:
                    p = self._call("PUT", f"/api/providers/{_q(p['id'])}", {"api_key": api_key}) or p
                    self._models = None
                return p
        body = {"name": name, "kind": kind, "base_url": base_url}
        if api_key is not None:
            body["api_key"] = api_key
        provider = self._call("POST", "/api/providers", body)
        self._models = None
        return provider

    def delete_provider(self, provider_id: str) -> None:
        self._call("DELETE", f"/api/providers/{_q(provider_id)}")
        self._models = None

    def discover(self, network: bool = False) -> List[Dict[str, Any]]:
        """AI servers detected on this machine (and the local network if ``network=True``)."""
        return self._call("GET", "/api/discover" + ("?network=1" if network else "")).get("servers") or []

    def resolve_model(self, model: Optional[str] = None) -> str:
        """Turn a model key, id or name into a key. ``None`` means the
        default model from settings, else the first model available."""
        models = self.models()
        if model is None:
            default = self._call("GET", "/api/settings").get("default_model")
            if default:
                return default
            if not models:
                raise LiteChatError("no models available: add a provider first")
            return models[0].key
        for attr in ("key", "id", "name"):
            for m in models:
                if getattr(m, attr) == model:
                    return m.key
        if "/" in model and model.split("/", 1)[0] in {m.provider_id for m in models}:
            return model  # a key the model list doesn't show (e.g. not loaded yet)
        names = ", ".join(sorted({m.id for m in models})) or "none"
        raise LiteChatError(f"unknown model {model!r}; available: {names}")

    # -- chats

    def chats(self) -> List["Chat"]:
        return [Chat(self, c) for c in self._call("GET", "/api/chats")]

    def chat(self, chat_id: str) -> "Chat":
        return Chat(self, self._call("GET", f"/api/chats/{_q(chat_id)}")["chat"])

    def new_chat(self, *, model: Optional[str] = None, system_prompt: str = "", title: Optional[str] = None) -> "Chat":
        body: Dict[str, Any] = {"model": self.resolve_model(model), "system_prompt": system_prompt}
        if title is not None:
            body["title"] = title
        return Chat(self, self._call("POST", "/api/chats", body))

    def ask(self, prompt: str, *, model: Optional[str] = None, system_prompt: str = "", images: Sequence[Image] = (), tools: bool = False) -> Reply:
        """One question in a new chat (which stays in the history)."""
        return self.new_chat(model=model, system_prompt=system_prompt).send(prompt, images=images, tools=tools)

    def search(self, query: str) -> List[Dict[str, Any]]:
        return self._call("GET", "/api/search?q=" + urllib.parse.quote(query))

    def export(self) -> Any:
        """Every chat, in liteChat's JSON export format."""
        return self._call("GET", "/api/export")

    def import_chats(self, data: Union[str, bytes, Dict[str, Any], List[Any]]) -> Any:
        """Import a liteChat, ChatGPT or Open WebUI export (parsed JSON, or its text)."""
        raw = data if isinstance(data, (str, bytes)) else json.dumps(data)
        return self._call("POST", "/api/import", raw=raw.encode() if isinstance(raw, str) else raw)

    # -- HTTP

    def _open(self, method: str, path: str, body: Any = None, *, raw: Optional[bytes] = None, timeout: Optional[float] = None):
        data = raw if raw is not None else (json.dumps(body).encode() if body is not None else None)
        req = urllib.request.Request(self.url + path, data=data, method=method)
        req.add_header("X-LiteChat", "1")
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            return self._opener.open(req, timeout=timeout or self.timeout)
        except urllib.error.HTTPError as e:
            try:
                message = json.loads(e.read()).get("error") or e.reason
            except ValueError:
                message = e.reason
            cls = AuthError if e.code in (401, 403) else APIError
            raise cls(e.code, str(message)) from None
        except urllib.error.URLError as e:
            raise LiteChatError(f"cannot reach liteChat at {self.url}: {e.reason}") from None

    def _call(self, method: str, path: str, body: Any = None, *, raw: Optional[bytes] = None) -> Any:
        with self._open(method, path, body, raw=raw) as resp:
            text = resp.read()
        return json.loads(text) if text.strip() else None

    def __repr__(self) -> str:
        return f"<litechat.Client {self.url}>"


# ---------------------------------------------------------------- chats


class Chat:
    """A conversation. ``send`` continues its active branch."""

    def __init__(self, client: Client, data: Dict[str, Any]) -> None:
        self._client = client
        self._update(data)

    def _update(self, d: Dict[str, Any]) -> None:
        self.id: str = d["id"]
        self.title: str = d.get("title", "")
        self.model: str = d.get("model", "")
        self.system_prompt: str = d.get("system_prompt", "")
        self.pinned: bool = d.get("pinned", False)
        self.leaf_id: str = d.get("leaf_id", "")
        self.created_at: int = d.get("created_at", 0)
        self.updated_at: int = d.get("updated_at", 0)

    def send(self, text: str, *, model: Optional[str] = None, images: Sequence[Image] = (), tools: bool = False) -> Reply:
        """Send a message and wait for the whole reply.

        Raises :class:`ReplyError` if the reply fails or stops at the model's
        length limit; the error's ``reply`` holds the text that arrived.
        """
        stream = self.stream(text, model=model, images=images, tools=tools)
        for _ in stream:
            pass
        return stream.reply

    def stream(self, text: str, *, model: Optional[str] = None, images: Sequence[Image] = (), tools: bool = False) -> "ReplyStream":
        """Send a message and iterate over the reply as it arrives:

        >>> for piece in chat.stream("Tell me a story"):
        ...     print(piece, end="", flush=True)

        Pieces include reasoning and tool blocks as the model writes them.
        The finished :class:`Reply` is in ``.reply`` afterwards.
        """
        key = self._client.resolve_model(model or self.model or None)
        body = {
            "parent_id": self.leaf_id,
            "content": text,
            "model": key,
            "images": [_data_url(i) for i in images],
            "tools": tools,
        }
        return ReplyStream(self, key, body)

    def messages(self) -> List[Message]:
        """The active branch, oldest first."""
        data = self._client._call("GET", f"/api/chats/{_q(self.id)}")
        self._update(data["chat"])
        by_id = {m["id"]: m for m in data["messages"]}
        path, cur = [], self.leaf_id
        while cur and cur in by_id:
            path.append(Message._from(by_id[cur]))
            cur = by_id[cur]["parent_id"]
        return path[::-1]

    def rename(self, title: str) -> None:
        self._patch({"title": title})

    def set_model(self, model: str) -> None:
        self._patch({"model": self._client.resolve_model(model)})

    def set_system_prompt(self, prompt: str) -> None:
        self._patch({"system_prompt": prompt})

    def stop(self) -> None:
        """Stop a reply in progress (from another thread)."""
        self._client._call("POST", f"/api/chats/{_q(self.id)}/stop")

    def delete(self) -> None:
        self._client._call("DELETE", f"/api/chats/{_q(self.id)}")

    def refresh(self) -> None:
        self._update(self._client._call("GET", f"/api/chats/{_q(self.id)}")["chat"])

    def _patch(self, fields: Dict[str, Any]) -> None:
        res = self._client._call("PATCH", f"/api/chats/{_q(self.id)}", fields)
        if isinstance(res, dict) and "id" in res:
            self._update(res)
        else:
            self.refresh()

    def __repr__(self) -> str:
        return f"<litechat.Chat {self.id} {self.title!r}>"


class ReplyStream:
    """Iterator over a reply's text; ``reply`` is set once it finishes."""

    def __init__(self, chat: Chat, model: str, body: Dict[str, Any]) -> None:
        self._chat = chat
        self._model = model
        self._body = body
        self._started = False
        self.reply: Optional[Reply] = None  # type: ignore[assignment]

    def __iter__(self) -> Iterator[str]:
        if self._started:
            raise LiteChatError("a reply stream can only be read once")
        self._started = True
        chat, client = self._chat, self._chat._client
        message_id, parts = "", []
        resp = client._open("POST", f"/api/chats/{_q(chat.id)}/messages", self._body, timeout=300)
        try:
            for event, data in _sse(resp):
                if event == "meta":
                    message_id = data["assistant"]["id"]
                    chat.leaf_id = message_id
                elif event == "delta":
                    parts.append(data["t"])
                    yield data["t"]
                elif event == "done":
                    self.reply = Reply(data.get("content", ""), message_id, self._model, data.get("error", ""))
                    break
        finally:
            resp.close()
        if self.reply is None:
            # connection dropped: the server keeps generating; report what arrived
            self.reply = Reply("".join(parts), message_id, self._model, "connection closed before the reply finished")
        if self.reply.error:
            raise ReplyError(self.reply.error, self.reply)


# ---------------------------------------------------------------- helpers


def _q(s: str) -> str:
    return urllib.parse.quote(s, safe="")


def _sse(resp) -> Iterator[tuple]:
    event, data = "message", []
    for raw in resp:
        line = raw.decode("utf-8").rstrip("\r\n")
        if not line:
            if data:
                yield event, json.loads("\n".join(data))
            event, data = "message", []
        elif line.startswith("event:"):
            event = line[6:].strip()
        elif line.startswith("data:"):
            data.append(line[5:].lstrip())


def _data_url(image: Image) -> str:
    if isinstance(image, str) and image.startswith("data:"):
        return image
    if isinstance(image, bytes):
        mime, raw = _sniff(image), image
    else:
        path = Path(image)
        raw = path.read_bytes()
        mime = mimetypes.guess_type(path.name)[0] or _sniff(raw)
    return f"data:{mime};base64,{base64.b64encode(raw).decode()}"


def _sniff(raw: bytes) -> str:
    if raw.startswith(b"\x89PNG"):
        return "image/png"
    if raw.startswith(b"\xff\xd8"):
        return "image/jpeg"
    if raw[:4] == b"RIFF" and raw[8:12] == b"WEBP":
        return "image/webp"
    if raw.startswith((b"GIF87a", b"GIF89a")):
        return "image/gif"
    raise LiteChatError("unrecognised image format; pass a file path or a data: URL")
