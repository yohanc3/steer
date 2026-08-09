"""Command-line entry point for predeployment stages."""

from __future__ import annotations

import argparse
import os

from .public_endpoint import PublicEndpointError, register_telegram_webhook, wait_for_public_health
from .validation import ConfigurationError, validate_environment


def main() -> int:
    """Run the selected predeployment stage and return its exit status."""
    parser = argparse.ArgumentParser(description="Steer predeployment stages")
    parser.add_argument("stage", choices=("validate", "wait-public", "register-webhook"))
    arguments = parser.parse_args()

    try:
        deployment = validate_environment(os.environ)
        if arguments.stage == "validate":
            print(f"configuration valid for {deployment} deployment")
        elif arguments.stage == "wait-public":
            wait_for_public_health(os.environ)
            print("public health route is reachable")
        elif arguments.stage == "register-webhook":
            wait_for_public_health(os.environ)
            register_telegram_webhook(os.environ)
            print("Telegram webhook registered")
    except (ConfigurationError, PublicEndpointError) as error:
        print(f"predeployment {arguments.stage} failed: {error}", flush=True)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
