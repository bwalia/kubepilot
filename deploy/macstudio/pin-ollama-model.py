#!/usr/bin/env python3
"""Pin interactive Ollama settings on the Mac Studio launchd host.

Keeps dashboard RCA on a small non-thinking model. Large thinking models
(qwen3.8, etc.) routinely exceed UI timeouts; every Mac Studio deploy re-applies
this pin so a manual experiment cannot stick forever.

Usage:
  pin-ollama-model.py <start-server.sh> <ollama_base_url> <model>
"""

from __future__ import annotations

import pathlib
import re
import sys


def main() -> int:
    if len(sys.argv) != 4:
        print(
            "usage: pin-ollama-model.py <start-server.sh> <ollama_base_url> <model>",
            file=sys.stderr,
        )
        return 2

    path = pathlib.Path(sys.argv[1])
    url = sys.argv[2]
    model = sys.argv[3]
    text = path.read_text()

    if "KUBEPILOT_OLLAMA_BASE_URL=" in text:
        text = re.sub(
            r"export KUBEPILOT_OLLAMA_BASE_URL=.*",
            f'export KUBEPILOT_OLLAMA_BASE_URL="${{KUBEPILOT_OLLAMA_BASE_URL:-{url}}}"',
            text,
        )
    else:
        text = text.replace(
            "set -euo pipefail\n",
            "set -euo pipefail\n"
            f'export KUBEPILOT_OLLAMA_BASE_URL="${{KUBEPILOT_OLLAMA_BASE_URL:-{url}}}"\n',
            1,
        )

    if "KUBEPILOT_OLLAMA_MODEL=" in text:
        text = re.sub(
            r"export KUBEPILOT_OLLAMA_MODEL=.*",
            f'export KUBEPILOT_OLLAMA_MODEL="${{KUBEPILOT_OLLAMA_MODEL:-{model}}}"',
            text,
        )
    else:
        text = text.replace(
            "set -euo pipefail\n",
            "set -euo pipefail\n"
            f'export KUBEPILOT_OLLAMA_MODEL="${{KUBEPILOT_OLLAMA_MODEL:-{model}}}"\n',
            1,
        )

    # Drop any previous --ollama-* flags, then re-insert so empty cobra defaults
    # cannot win over env via viper BindPFlag.
    text = re.sub(r"\n\s*--ollama-model=.*\\?\n?", "\n", text)
    text = re.sub(r"\n\s*--ollama-base-url=.*\\?\n?", "\n", text)

    needle = "exec /Users/balinderwalia/.kubepilot/bin/kubepilot serve \\"
    if needle not in text:
        match = re.search(r"(exec \S+/kubepilot serve \\)", text)
        if not match:
            print("could not find kubepilot serve exec line in start-server.sh", file=sys.stderr)
            return 1
        needle = match.group(1)

    insert = (
        needle
        + '\n  --ollama-base-url="${KUBEPILOT_OLLAMA_BASE_URL}" \\'
        + '\n  --ollama-model="${KUBEPILOT_OLLAMA_MODEL}" \\'
    )
    text = text.replace(needle, insert, 1)
    path.write_text(text)
    print(f"Updated {path} → model={model} base_url={url}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
