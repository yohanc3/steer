#!/usr/bin/env python3
"""Start a local ngrok tunnel before handing deployment to Docker Compose."""

from __future__ import annotations

import json
import os
import subprocess
import sys
import time
from pathlib import Path
from urllib.error import URLError
from urllib.request import urlopen


ROOT_DIRECTORY = Path(__file__).resolve().parents[2]
NGROK_API_URL = "http://127.0.0.1:4040/api/tunnels"


def deployment_environment(path: Path) -> str:
    """Read the deployment mode without parsing unrelated multiline secret values."""
    for line in path.read_text().splitlines():
        key, separator, value = line.partition("=")
        if separator and key.strip() == "DEPLOYMENT_ENV":
            return value.strip().strip("\"'")
    return ""


def wait_for_ngrok_url(process: subprocess.Popen[bytes]) -> str:
    """Read ngrok's assigned HTTPS tunnel URL while its local API becomes ready."""
    for _ in range(30):
        if process.poll() is not None:
            raise RuntimeError("ngrok exited before creating a tunnel; check its local configuration")
        try:
            with urlopen(NGROK_API_URL, timeout=1) as response:  # noqa: S310 - loopback API.
                tunnels = json.load(response)["tunnels"]
        except (URLError, KeyError, json.JSONDecodeError):
            time.sleep(1)
            continue

        for tunnel in tunnels:
            public_url = tunnel.get("public_url", "")
            if public_url.startswith("https://"):
                return public_url
        time.sleep(1)

    raise RuntimeError("ngrok did not expose an HTTPS tunnel within 30 seconds")


def main() -> int:
    """Discover a development URL, then replace this process with Docker Compose."""
    os.chdir(ROOT_DIRECTORY)
    env_path = ROOT_DIRECTORY / ".env"
    if not env_path.is_file():
        print("deployment failed: .env is required; copy .env.example and configure it", file=sys.stderr)
        return 1

    environment = deployment_environment(env_path)
    if environment not in {"dev", "prod"}:
        print("deployment failed: DEPLOYMENT_ENV must be dev or prod", file=sys.stderr)
        return 1

    compose_environment = os.environ.copy()
    if environment == "dev":
        try:
            ngrok_process = subprocess.Popen(
                ["ngrok", "http", "8080"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            compose_environment["PUBLIC_BASE_URL"] = wait_for_ngrok_url(ngrok_process)
        except (FileNotFoundError, RuntimeError) as error:
            print(f"development tunnel failed: {error}", file=sys.stderr)
            return 1

    os.execvpe("docker", ["docker", "compose", "up", "--build"], compose_environment)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
