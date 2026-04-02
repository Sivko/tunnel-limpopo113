"""Собирает frps.toml из frps.toml.example и FRP_TOKEN из .env (рядом с корнем проекта)."""
from __future__ import annotations

import re
import sys
from pathlib import Path


def main() -> None:
    root = Path(__file__).resolve().parent.parent
    env_path = root / ".env"
    example = root / "frps.toml.example"
    out = root / "frps.toml"
    if not env_path.is_file():
        sys.exit("Нет файла .env")
    token = None
    for line in env_path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, _, v = line.partition("=")
        k, v = k.strip(), v.strip()
        if (v.startswith('"') and v.endswith('"')) or (v.startswith("'") and v.endswith("'")):
            v = v[1:-1]
        if k == "FRP_TOKEN":
            token = v
            break
    if not token:
        sys.exit("В .env не найден FRP_TOKEN")
    text = example.read_text(encoding="utf-8")
    esc = token.replace("\\", "\\\\").replace('"', '\\"')
    text, n = re.subn(
        r'^auth\.token = "CHANGE_ME_USE_FRP_TOKEN_FROM_ENV"\s*$',
        f'auth.token = "{esc}"',
        text,
        count=1,
        flags=re.MULTILINE,
    )
    if n != 1:
        sys.exit("В frps.toml.example ожидается строка auth.token = \"CHANGE_ME_USE_FRP_TOKEN_FROM_ENV\"")
    out.write_text(text, encoding="utf-8")


if __name__ == "__main__":
    main()
