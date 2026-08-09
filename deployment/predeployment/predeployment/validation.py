"""Environment validation shared by all predeployment stages."""

from __future__ import annotations

from collections.abc import Mapping


class ConfigurationError(ValueError):
    """Raised when deployment configuration is missing or inconsistent."""


APPLICATION_VARIABLES = (
    "TELEGRAM_BOT_TOKEN",
    "TELEGRAM_WEBHOOK_SECRET",
    "TELLER_APPLICATION_ID",
    "TELLER_ENVIRONMENT",
    "TELLER_CERT_PEM",
    "TELLER_KEY_PEM",
    "TELLER_TOKEN_SIGNING_PUBLIC_KEY",
    "TOKEN_ENCRYPTION_KEY",
    "PUBLIC_BASE_URL",
)


def validate_environment(environment: Mapping[str, str]) -> str:
    """Validate required values without printing their potentially secret contents."""
    deployment = environment.get("DEPLOYMENT_ENV", "")
    if deployment not in {"dev", "prod"}:
        raise ConfigurationError("DEPLOYMENT_ENV must be either 'dev' or 'prod'")

    missing = [name for name in APPLICATION_VARIABLES if not environment.get(name)]
    if missing:
        names = ", ".join(missing)
        raise ConfigurationError(f"missing required environment values: {names}")

    teller_environment = environment["TELLER_ENVIRONMENT"]
    if teller_environment not in {"sandbox", "development", "production"}:
        raise ConfigurationError(
            "TELLER_ENVIRONMENT must be sandbox, development, or production"
        )

    if deployment == "dev":
        _require(environment, "NGROK_AUTHTOKEN")
        _validate_endpoint(environment, "NGROK_DOMAIN")
    else:
        _validate_endpoint(environment, "APP_DOMAIN")
        _require(environment, "DEPLOYMENT_PUBLIC_IP")
        _require(environment, "LETSENCRYPT_EMAIL")

    return deployment


def _validate_endpoint(environment: Mapping[str, str], domain_variable: str) -> None:
    """Require the deployment URL to match the selected public domain."""
    domain = _require(environment, domain_variable)
    expected_url = f"https://{domain}"
    if environment["PUBLIC_BASE_URL"].rstrip("/") != expected_url:
        raise ConfigurationError(
            f"PUBLIC_BASE_URL must equal {expected_url} for {domain_variable}"
        )


def _require(environment: Mapping[str, str], name: str) -> str:
    """Return a required value or name the missing variable in the error."""
    value = environment.get(name, "")
    if not value:
        raise ConfigurationError(f"missing required environment value: {name}")
    return value
