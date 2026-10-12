# Automated Releases

The workflow in `.github/workflows/deploy-prod.yml` verifies every push to `prod`. When the repository variable `RELEASE_ENABLED` is `true`, it also publishes commit-tagged backend and frontend images to GitHub Container Registry and deploys that exact tag over SSH.

## Local development

Copy `.env.example` to the ignored `.env`, fill the Telegram and Teller values, then use the existing local launcher or Docker Compose workflow documented in `DEPLOYMENT.md`.

Polling configuration:

- `TELLER_POLL_INTERVAL`: how often the current in-memory user snapshot is synchronized, for example `30m`.
- `TELLER_QUEUE_REFRESH_INTERVAL`: how often connected user IDs are reloaded from SQLite, for example `5m`.
- `TELLER_SYNC_LOOKBACK_DAYS`: inclusive Teller date overlap, normally `10`.
- `TELEGRAM_DELIVERY_INTERVAL`: how often settled transactions are drained for notification, for example `30s`.
- `TELEGRAM_DELIVERY_CONCURRENCY`: batch size and maximum simultaneous Telegram sends, normally `5`.

## Production host

1. Copy `ops/env/production.env.example` to `/opt/steer/.env` and fill every blank value.
2. Install Docker Compose and Caddy. Replace the domains in `ops/Caddyfile.example` and install it as the host Caddyfile.
3. Authenticate Docker to `ghcr.io` once with a read-only package token.
4. Ensure the deployment SSH user owns `/opt/steer`.

Create a GitHub environment named `production` with variables `WEB_URL`, `TELLER_APPLICATION_ID`, and `TELLER_ENVIRONMENT`. Add secrets `PROD_HOST`, `PROD_PORT`, `PROD_USER`, `PROD_SSH_PRIVATE_KEY`, and `PROD_SSH_KNOWN_HOSTS`.

Keep the repository variable `RELEASE_ENABLED=false` until the VPS, DNS, Caddy, `/opt/steer/.env`, and GitHub values are ready. Set it to `true` to enable image publishing and deployment.

Production uses `https://app.your-domain.com`; staging reserves `https://staging.your-domain.com`. Point each DNS record to the VPS and keep the matching `PUBLIC_BASE_URL` in that environment's host-owned `.env` file.

## Manual deployment or rollback

Run the deployment script with any published commit SHA:

```sh
DEPLOY_DIR=/opt/steer /opt/steer/deploy.sh <git-commit-sha>
```

The most recently deployed tag is recorded in `/opt/steer/.deployed-image-tag`.
