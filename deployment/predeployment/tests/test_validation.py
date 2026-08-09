"""Tests for safe predeployment environment validation."""

from __future__ import annotations

import unittest

from predeployment.validation import ConfigurationError, validate_environment


def development_environment() -> dict[str, str]:
    """Return a complete non-secret development environment for tests."""
    return {
        "DEPLOYMENT_ENV": "dev",
        "NGROK_DOMAIN": "example.ngrok-free.dev",
        "PUBLIC_BASE_URL": "https://example.ngrok-free.dev",
        "TELEGRAM_BOT_TOKEN": "token",
        "TELEGRAM_WEBHOOK_SECRET": "secret",
        "TELLER_APPLICATION_ID": "application",
        "TELLER_ENVIRONMENT": "sandbox",
        "TELLER_CERT_PEM": "certificate",
        "TELLER_KEY_PEM": "key",
        "TELLER_TOKEN_SIGNING_PUBLIC_KEY": "public-key",
        "TOKEN_ENCRYPTION_KEY": "encryption-key",
    }


class ValidateEnvironmentTests(unittest.TestCase):
    """Confirm validation reports configuration problems without secret values."""

    def test_accepts_complete_development_environment(self) -> None:
        self.assertEqual(validate_environment(development_environment()), "dev")

    def test_reports_missing_values_by_name(self) -> None:
        environment = development_environment()
        del environment["TELEGRAM_BOT_TOKEN"]

        with self.assertRaisesRegex(
            ConfigurationError, "missing required environment values: TELEGRAM_BOT_TOKEN"
        ):
            validate_environment(environment)

    def test_rejects_mismatched_public_base_url(self) -> None:
        environment = development_environment()
        environment["PUBLIC_BASE_URL"] = "https://wrong.example"

        with self.assertRaisesRegex(
            ConfigurationError, "PUBLIC_BASE_URL must equal"
        ):
            validate_environment(environment)

    def test_requires_production_endpoint_values(self) -> None:
        environment = development_environment()
        environment["DEPLOYMENT_ENV"] = "prod"
        environment["APP_DOMAIN"] = "steer.example.com"
        environment["PUBLIC_BASE_URL"] = "https://steer.example.com"

        with self.assertRaisesRegex(
            ConfigurationError, "DEPLOYMENT_PUBLIC_IP"
        ):
            validate_environment(environment)
