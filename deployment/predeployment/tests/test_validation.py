"""Tests for safe predeployment environment validation."""

from __future__ import annotations

import unittest
from unittest.mock import patch

from predeployment.public_endpoint import PublicEndpointError, register_telegram_webhook, wait_for_public_health
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

    @patch("predeployment.public_endpoint.urlopen")
    def test_waits_for_successful_public_health(self, mocked_urlopen) -> None:
        response = mocked_urlopen.return_value.__enter__.return_value
        response.status = 200

        wait_for_public_health(development_environment(), attempts=1)

        mocked_urlopen.assert_called_once_with(
            "https://example.ngrok-free.dev/healthz", timeout=5
        )

    @patch("predeployment.public_endpoint.urlopen")
    def test_registers_telegram_webhook(self, mocked_urlopen) -> None:
        response = mocked_urlopen.return_value.__enter__.return_value
        response.read.return_value = b'{"ok": true}'

        register_telegram_webhook(development_environment())

        request = mocked_urlopen.call_args.args[0]
        self.assertEqual(request.get_method(), "POST")
        self.assertIn(b"telegram%2Fwebhook", request.data)

    @patch("predeployment.public_endpoint.urlopen")
    def test_reports_telegram_rejection(self, mocked_urlopen) -> None:
        response = mocked_urlopen.return_value.__enter__.return_value
        response.read.return_value = b'{"ok": false, "description": "bad webhook"}'

        with self.assertRaisesRegex(PublicEndpointError, "bad webhook"):
            register_telegram_webhook(development_environment())
