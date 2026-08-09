"""Command-line entry point for predeployment stages."""

from __future__ import annotations

import argparse
import os

from .validation import ConfigurationError, validate_environment


def main() -> int:
    """Run the selected predeployment stage and return its exit status."""
    parser = argparse.ArgumentParser(description="Steer predeployment stages")
    parser.add_argument("stage", choices=("validate",))
    arguments = parser.parse_args()

    try:
        if arguments.stage == "validate":
            deployment = validate_environment(os.environ)
            print(f"configuration valid for {deployment} deployment")
    except ConfigurationError as error:
        print(f"predeployment validation failed: {error}", flush=True)
        return 1

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
