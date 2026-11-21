"""Locate the liteChat binary that this package installed."""

from __future__ import annotations

import os
import shutil
import sys
import sysconfig
from pathlib import Path

EXE = "litechat.exe" if sys.platform == "win32" else "litechat"


def _user_scheme() -> str:
    if hasattr(sysconfig, "get_preferred_scheme"):  # Python 3.10+
        return sysconfig.get_preferred_scheme("user")
    if sys.platform == "darwin" and sys._framework:  # type: ignore[attr-defined]
        return "osx_framework_user"
    return f"{os.name}_user"


def find_binary() -> str:
    """Return the path of the `litechat` binary.

    The wheel installs it into the environment's scripts folder (next to
    `python`), so look there first, then in the user scripts folder
    (`pip install --user`), then next to the package (`pip install --target`),
    then on PATH. Set LITECHAT_BINARY to use a different build.
    """
    override = os.environ.get("LITECHAT_BINARY")
    if override:
        return override

    folders = [sysconfig.get_path("scripts")]
    try:
        folders.append(sysconfig.get_path("scripts", scheme=_user_scheme()))
    except KeyError:
        pass
    folders.append(str(Path(__file__).resolve().parent.parent / "bin"))
    for folder in folders:
        path = Path(folder) / EXE
        if path.is_file():
            return str(path)

    found = shutil.which(EXE)
    if found:
        return found
    raise FileNotFoundError(
        "the litechat binary was not found. Reinstall with `pip install --force-reinstall "
        "litechat-py`, or set LITECHAT_BINARY to its path."
    )
