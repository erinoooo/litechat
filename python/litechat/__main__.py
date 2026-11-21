"""`python -m litechat [options]` runs the bundled binary with the same options."""

import os
import subprocess
import sys

from ._binary import find_binary


def main() -> int:
    binary = find_binary()
    if sys.platform != "win32":
        os.execv(binary, [binary, *sys.argv[1:]])
    try:
        return subprocess.call([binary, *sys.argv[1:]])
    except KeyboardInterrupt:
        return 130


if __name__ == "__main__":
    sys.exit(main())
