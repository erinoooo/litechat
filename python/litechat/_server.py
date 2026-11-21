"""Run the liteChat server from Python."""

from __future__ import annotations

import atexit
import collections
import logging
import os
import re
import subprocess
import sys
import threading
import time
from typing import TYPE_CHECKING, List, Optional, Union

from ._binary import find_binary
from ._errors import LiteChatError

if TYPE_CHECKING:
    from ._client import Client

log = logging.getLogger("litechat")
_READY = re.compile(r"listening on (http://\S+?),")

PathLike = Union[str, "os.PathLike[str]"]


def _command(
    binary: Optional[str],
    port: Optional[int],
    host: Optional[str],
    data_dir: Optional[PathLike],
    allow_local_mcp: bool,
) -> List[str]:
    cmd = [binary or find_binary()]
    if port is not None:
        cmd += ["--port", str(port)]
    if host is not None:
        cmd += ["--host", host]
    if data_dir is not None:
        cmd += ["--data", os.fspath(data_dir)]
    if allow_local_mcp:
        cmd.append("--allow-local-mcp")
    return cmd


def _environment(password: Optional[str]) -> dict:
    env = dict(os.environ)
    if password is not None:
        # through the environment, not argv, so it doesn't show in `ps`
        env["LITECHAT_PASSWORD"] = password
    return env


class Server:
    """A liteChat server running in the background.

    >>> with litechat.Server(port=8080, password="secret") as server:
    ...     print(server.url)

    Defaults differ from the command line: the server listens on 127.0.0.1
    only, on a free port (``port=0``). Pass ``host="0.0.0.0"`` to make it
    reachable from other machines. The server's log goes to the ``litechat``
    logger.
    """

    def __init__(
        self,
        *,
        port: int = 0,
        host: str = "127.0.0.1",
        data_dir: Optional[PathLike] = None,
        password: Optional[str] = None,
        allow_local_mcp: bool = False,
        binary: Optional[str] = None,
        startup_timeout: float = 15.0,
    ) -> None:
        self.password = password
        self.startup_timeout = startup_timeout
        self._cmd = _command(binary, port, host, data_dir, allow_local_mcp)
        self._proc: Optional[subprocess.Popen] = None
        self._reader: Optional[threading.Thread] = None
        self._url: Optional[str] = None
        self._ready = threading.Event()
        self._recent: "collections.deque[str]" = collections.deque(maxlen=20)

    # lifecycle

    def start(self) -> "Server":
        """Start the server and wait until it accepts connections."""
        if self.running:
            return self
        self._ready.clear()
        self._url = None
        self._proc = subprocess.Popen(
            self._cmd,
            env=_environment(self.password),
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            text=True,
            encoding="utf-8",
            errors="replace",
        )
        self._reader = threading.Thread(target=self._read_log, args=(self._proc,), daemon=True)
        self._reader.start()
        atexit.register(self.stop)

        deadline = time.monotonic() + self.startup_timeout
        while not self._ready.wait(0.05):
            if self._proc.poll() is not None:
                proc, self._proc = self._proc, None
                self._close(proc)
                tail = "\n".join(self._recent)
                raise LiteChatError(f"liteChat exited with code {proc.returncode}:\n{tail}")
            if time.monotonic() > deadline:
                self.stop()
                raise LiteChatError(f"liteChat did not start within {self.startup_timeout:g}s")
        return self

    def stop(self, timeout: float = 10.0) -> None:
        """Stop the server; replies in progress are saved as they stand."""
        proc, self._proc = self._proc, None
        atexit.unregister(self.stop)
        if proc is None:
            return
        if proc.poll() is None:
            proc.terminate()
            try:
                proc.wait(timeout)
            except subprocess.TimeoutExpired:
                proc.kill()
        self._close(proc)

    def _close(self, proc: subprocess.Popen) -> None:
        proc.wait()
        if self._reader is not None:
            self._reader.join(2)
        if proc.stderr is not None:
            proc.stderr.close()

    def wait(self) -> int:
        """Block until the server exits; return its exit code."""
        if self._proc is None:
            raise LiteChatError("the server is not running")
        return self._proc.wait()

    def __enter__(self) -> "Server":
        return self.start()

    def __exit__(self, *exc: object) -> None:
        self.stop()

    # state

    @property
    def running(self) -> bool:
        return self._proc is not None and self._proc.poll() is None

    @property
    def url(self) -> str:
        """Base URL, such as ``http://127.0.0.1:41235``."""
        if self._url is None:
            raise LiteChatError("the server is not running")
        return self._url

    @property
    def port(self) -> int:
        return int(self.url.rsplit(":", 1)[1])

    def client(self, password: Optional[str] = None) -> "Client":
        """A :class:`Client` logged in to this server."""
        from ._client import Client

        pw = password if password is not None else self.password
        if pw is None:
            raise LiteChatError("pass password= to Server(...) or to client(...)")
        return Client(self.url, pw)

    def __repr__(self) -> str:
        state = self._url if self.running else "stopped"
        return f"<litechat.Server {state}>"

    def _read_log(self, proc: subprocess.Popen) -> None:
        assert proc.stderr is not None
        for line in proc.stderr:
            line = line.rstrip()
            self._recent.append(line)
            log.info("%s", line)
            if not self._ready.is_set():
                m = _READY.search(line)
                if m:
                    self._url = m.group(1)
                    self._ready.set()


def start(**options) -> Server:
    """Start a server in the background and return it. Options as :class:`Server`."""
    return Server(**options).start()


def run(
    *,
    port: Optional[int] = None,
    host: Optional[str] = None,
    data_dir: Optional[PathLike] = None,
    password: Optional[str] = None,
    allow_local_mcp: bool = False,
    binary: Optional[str] = None,
) -> int:
    """Run the server in the foreground, like the ``litechat`` command, until
    Ctrl+C. Unset options use the command's defaults (port 3000, all
    interfaces). Returns the exit code."""
    cmd = _command(binary, port, host, data_dir, allow_local_mcp)
    proc = subprocess.Popen(cmd, env=_environment(password))
    try:
        return proc.wait()
    except KeyboardInterrupt:
        if sys.platform != "win32":  # Windows already delivered Ctrl+C to it
            proc.terminate()
        return proc.wait()
