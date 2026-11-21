#!/usr/bin/env python3
"""Repack liteChat release archives into PyPI wheels (package: litechat-py).

Each wheel holds the prebuilt binary under <name>.data/scripts/, so pip
installs it straight onto PATH as `litechat` (no Python runs at startup),
plus the `litechat` Python package, which starts the binary and talks to it.
The binaries are static (CGO off), so one Linux wheel serves glibc and musl.

    python3 python/build_wheels.py <dir with release archives> <version> <out dir>

Standard library only. Output is reproducible: same inputs, same bytes.
"""

import base64
import hashlib
import io
import sys
import tarfile
import zipfile
from pathlib import Path

NAME = "litechat-py"
DIST = NAME.replace("-", "_")
HERE = Path(__file__).resolve().parent

# Go target -> wheel platform tag. Go 1.25+ needs macOS 12 or later.
# FreeBSD is left out: PyPI does not accept FreeBSD wheels.
PLATFORMS = {
    "linux_amd64": "manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_1_x86_64",
    "linux_arm64": "manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_1_aarch64",
    "linux_arm": "manylinux_2_17_armv7l.manylinux2014_armv7l.musllinux_1_1_armv7l",
    "darwin_amd64": "macosx_12_0_x86_64",
    "darwin_arm64": "macosx_12_0_arm64",
    "windows_amd64": "win_amd64",
    "windows_arm64": "win_arm64",
}

EPOCH = (2026, 1, 1, 0, 0, 0)  # fixed timestamp for reproducible zips


def metadata(version: str) -> str:
    readme = (HERE / "README.md").read_text(encoding="utf-8")
    return (
        "Metadata-Version: 2.4\n"
        f"Name: {NAME}\n"
        f"Version: {version}\n"
        "Summary: Lightweight self-hosted chat UI for local and cloud LLMs: one binary, plus a Python API.\n"
        "License-Expression: MIT\n"
        "License-File: LICENSE\n"
        "Requires-Python: >=3.8\n"
        "Project-URL: Homepage, https://github.com/erinoooo/litechat\n"
        "Project-URL: Documentation, https://github.com/erinoooo/litechat/tree/main/docs\n"
        "Project-URL: Changelog, https://github.com/erinoooo/litechat/blob/main/CHANGELOG.md\n"
        "Project-URL: Issues, https://github.com/erinoooo/litechat/issues\n"
        "Keywords: llm,chat,ollama,openai,anthropic,mcp,self-hosted\n"
        "Classifier: Development Status :: 4 - Beta\n"
        "Classifier: Environment :: Web Environment\n"
        "Classifier: Intended Audience :: End Users/Desktop\n"
        "Classifier: Operating System :: MacOS\n"
        "Classifier: Operating System :: Microsoft :: Windows\n"
        "Classifier: Operating System :: POSIX :: Linux\n"
        "Classifier: Topic :: Communications :: Chat\n"
        "Classifier: Programming Language :: Python :: 3\n"
        "Classifier: Typing :: Typed\n"
        "Description-Content-Type: text/markdown\n"
        "\n" + readme
    )


def read_archive(path: Path) -> dict:
    """Return {"litechat": bytes, "LICENSE": bytes} from a release archive."""
    out = {}
    if path.suffix == ".zip":
        with zipfile.ZipFile(path) as z:
            for n in z.namelist():
                base = n.rsplit("/", 1)[-1]
                if base in ("litechat.exe", "LICENSE"):
                    out[base] = z.read(n)
    else:
        with tarfile.open(path) as t:
            for m in t.getmembers():
                base = m.name.rsplit("/", 1)[-1]
                if m.isfile() and base in ("litechat", "LICENSE"):
                    out[base] = t.extractfile(m).read()
    return out


def record_line(name: str, data: bytes) -> str:
    digest = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()
    return f"{name},sha256={digest},{len(data)}"


def build(archive: Path, platform: str, version: str, out: Path) -> Path:
    files = read_archive(archive)
    exe = "litechat.exe" if "litechat.exe" in files else "litechat"
    if exe not in files or "LICENSE" not in files:
        sys.exit(f"{archive.name}: binary or LICENSE missing")

    info = f"{DIST}-{version}.dist-info"
    wheel_tag = f"py3-none-{platform}"
    entries = [
        (f"{DIST}-{version}.data/scripts/{exe}", files[exe], 0o755),
        *(
            (f"litechat/{f.name}", f.read_bytes(), 0o644)
            for f in sorted((HERE / "litechat").iterdir())
            if f.is_file() and f.suffix in (".py", ".typed")
        ),
        (f"{info}/licenses/LICENSE", files["LICENSE"], 0o644),
        (f"{info}/METADATA", metadata(version).encode(), 0o644),
        (
            f"{info}/WHEEL",
            (
                "Wheel-Version: 1.0\n"
                "Generator: litechat build_wheels.py\n"
                "Root-Is-Purelib: false\n"
                + "".join(f"Tag: py3-none-{p}\n" for p in platform.split("."))
            ).encode(),
            0o644,
        ),
    ]
    record = "\n".join(record_line(n, d) for n, d, _ in entries) + f"\n{info}/RECORD,,\n"
    entries.append((f"{info}/RECORD", record.encode(), 0o644))

    path = out / f"{DIST}-{version}-{wheel_tag}.whl"
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        for name, data, mode in entries:
            zi = zipfile.ZipInfo(name, EPOCH)
            zi.external_attr = (0o100000 | mode) << 16
            zi.compress_type = zipfile.ZIP_DEFLATED
            z.writestr(zi, data)
    return path


def main() -> None:
    if len(sys.argv) != 4:
        sys.exit(__doc__)
    src, version, out = Path(sys.argv[1]), sys.argv[2].lstrip("v"), Path(sys.argv[3])
    out.mkdir(parents=True, exist_ok=True)
    for target, platform in PLATFORMS.items():
        matches = list(src.glob(f"litechat_{version}_{target}.*"))
        if len(matches) != 1:
            sys.exit(f"expected one archive for {target} in {src}, found {len(matches)}")
        wheel = build(matches[0], platform, version, out)
        print(f"{wheel.name}  {wheel.stat().st_size / 1048576:.1f} MB")


if __name__ == "__main__":
    main()
