"""Public endpoint readiness and Telegram webhook registration."""

from __future__ import annotations

import json
import time
from collections.abc import Mapping
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


class PublicEndpointError(RuntimeError):
    """Raised when the public route or Telegram API cannot complete setup."""


def wait_for_public_health(
    environment: Mapping[str, str], *, attempts: int = 30, delay_seconds: float = 2
) -> None:
    """Wait for the deployed gateway health route to return a successful response."""
    url = f"{environment['PUBLIC_BASE_URL'].rstrip('/')}/healthz"
    last_error = "no response received"

    for attempt in range(attempts):
        try:
            with urlopen(url, timeout=5) as response:  # noqa: S310 - URL is validated config.
                if 200 <= response.status < 300:
                    return
                last_error = f"received HTTP {response.status}"
        except HTTPError as error:
            last_error = f"received HTTP {error.code}"
        except URLError as error:
            last_error = f"network error: {error.reason}"

        if attempt < attempts - 1:
            time.sleep(delay_seconds)

    raise PublicEndpointError(
        f"public health check at {url} did not succeed after {attempts} attempts ({last_error}); "
        "check the gateway and public endpoint configuration"
    )


def register_telegram_webhook(environment: Mapping[str, str]) -> None:
    """Register the Telegram update endpoint without exposing credentials in errors."""
    data = urlencode(
        {
            "url": f"{environment['PUBLIC_BASE_URL'].rstrip('/')}/telegram/webhook",
            "secret_token": environment["TELEGRAM_WEBHOOK_SECRET"],
            "allowed_updates": json.dumps(["message"]),
        }
    ).encode()
    token = environment["TELEGRAM_BOT_TOKEN"]
    request = Request(
        f"https://api.telegram.org/bot{token}/setWebhook",
        data=data,
        method="POST",
    )

    try:
        with urlopen(request, timeout=10) as response:  # noqa: S310 - Telegram API is fixed.
            payload = json.load(response)
    except HTTPError as error:
        raise PublicEndpointError(
            f"Telegram setWebhook returned HTTP {error.code}; verify the bot token and network access"
        ) from error
    except URLError as error:
        raise PublicEndpointError(
            f"Telegram setWebhook network error: {error.reason}; check outbound network access"
        ) from error

    if not payload.get("ok"):
        description = payload.get("description", "no error description returned")
        raise PublicEndpointError(f"Telegram rejected setWebhook: {description}")
