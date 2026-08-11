#!/usr/bin/env python3
"""Start a local ngrok tunnel before handing deployment to Docker Compose."""

from __future__ import annotations

import json
import os
import socket
import subprocess
import sys
import time
from pathlib import Path
from urllib.error import URLError
from urllib.parse import urlsplit
from urllib.request import urlopen


ROOT_DIRECTORY = Path(__file__).resolve().parents[2]
NGROK_API_URL = "http://127.0.0.1:4040/api/tunnels"
COMPOSE_FILES = {
    "dev": ("compose.yaml", "compose.dev.yaml"),
    "prod": ("compose.yaml", "compose.prod.yaml"),
}


REQUIRED_APPLICATION_VALUES = (
    "TELEGRAM_BOT_TOKEN",
    "TELEGRAM_WEBHOOK_SECRET",
    "TELLER_APPLICATION_ID",
    "TELLER_ENVIRONMENT",
    "TELLER_CERT_PEM",
    "TELLER_KEY_PEM",
    "TELLER_TOKEN_SIGNING_PUBLIC_KEY",
    "TOKEN_ENCRYPTION_KEY",
)


def environment_values(path: Path) -> dict[str, str]:
    """Read scalar setup values before Compose parses multiline PEM content."""
    values: dict[str, str] = {}
    for line in path.read_text().splitlines():
        key, separator, value = line.partition("=")
        if separator:
            values[key.strip()] = value.strip().strip("\"'")
    return values


def validate_environment(values: dict[str, str]) -> None:
    """Validate deployment inputs and production DNS before starting processes."""
    deployment = values.get("DEPLOYMENT_ENV", "")
    if deployment not in {"dev", "prod"}:
        raise RuntimeError("DEPLOYMENT_ENV must be dev or prod")
    missing = [name for name in REQUIRED_APPLICATION_VALUES if not values.get(name)]
    if missing:
        raise RuntimeError(f"missing required environment values: {', '.join(missing)}")
    if deployment != "prod":
        return
    public_base_url = values.get("PUBLIC_BASE_URL", "")
    expected_ip = values.get("DEPLOYMENT_PUBLIC_IP", "")
    if not public_base_url or not expected_ip or not values.get("LETSENCRYPT_EMAIL"):
        raise RuntimeError("prod requires PUBLIC_BASE_URL, DEPLOYMENT_PUBLIC_IP, and LETSENCRYPT_EMAIL")
    domain = urlsplit(public_base_url).hostname
    if not domain:
        raise RuntimeError("PUBLIC_BASE_URL must contain an HTTPS domain")
    try:
        # Resolve DNS through the operating-system socket layer before Certbot requests a certificate.
        addresses = {result[4][0] for result in socket.getaddrinfo(domain, None, type=socket.SOCK_STREAM)}
    except socket.gaierror as error:
        raise RuntimeError(f"could not resolve {domain}: {error}") from error
    if expected_ip not in addresses:
        raise RuntimeError(f"{domain} does not resolve to DEPLOYMENT_PUBLIC_IP")


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

    values = environment_values(env_path)
    environment = values.get("DEPLOYMENT_ENV", "")

    compose_environment = os.environ.copy()
    try:
        validate_environment(values)
        if environment == "dev":
            ngrok_process = subprocess.Popen(
                ["ngrok", "http", "8080"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            compose_environment["PUBLIC_BASE_URL"] = wait_for_ngrok_url(ngrok_process)
        else:
            compose_environment["PUBLIC_BASE_URL"] = values["PUBLIC_BASE_URL"]
    except (FileNotFoundError, RuntimeError) as error:
        print(f"deployment failed: {error}", file=sys.stderr)
        return 1
    except Exception as error:
        print(f"deployment failed unexpectedly: {type(error).__name__}", file=sys.stderr)
        return 1

    compose_command = ["docker", "compose"]
    for compose_file in COMPOSE_FILES[environment]:
        compose_command.extend(["-f", compose_file])
    compose_command.extend(["up", "--build", "-d"])
    os.execvpe("docker", compose_command, compose_environment)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
