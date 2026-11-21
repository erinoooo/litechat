from __future__ import annotations

from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from ._client import Reply


class LiteChatError(Exception):
    """Something went wrong talking to or running liteChat."""


class APIError(LiteChatError):
    """The server answered a request with an error."""

    def __init__(self, status: int, message: str) -> None:
        super().__init__(f"{message} (HTTP {status})")
        self.status = status
        self.message = message


class AuthError(APIError):
    """Wrong password, or not logged in."""


class ReplyError(LiteChatError):
    """The model's reply failed or was cut short. ``reply`` holds what arrived."""

    def __init__(self, message: str, reply: "Optional[Reply]" = None) -> None:
        super().__init__(message)
        self.reply = reply
