# Deployment

## Local containers

1. Copy `.env.example` to `.env` and replace the Auth0 and Teller placeholders.
2. Keep `TELEGRAM_ENABLED=false` unless the backend is reachable through a public HTTPS tunnel.
3. Run `docker compose up --build`.
4. Open `http://localhost:5173`; the API is available at `http://localhost:8080`.

The local SQLite database is stored in the named Docker volume `steer_steer-data`.

### Local `.env` values

- `AUTH0_DOMAIN` and `VITE_AUTH0_DOMAIN`: the tenant domain shown in Auth0 under **Settings**, without `https://`.
- `VITE_AUTH0_CLIENT_ID`: the client ID of an Auth0 Single Page Application configured for `http://localhost:5173`.
- `AUTH0_AUDIENCE` and `VITE_AUTH0_AUDIENCE`: the identifier of the Auth0 API used by the Go backend. For local-only work, use `http://localhost:8080/api` in both places.
- `VITE_SERVER_BASE_URL`: `http://localhost:8080`.
- `CORS_ALLOWED_ORIGINS`: `http://localhost:5173`.
- `VITE_TELLER_APP_ID`: the application ID from the Teller dashboard.
- `TELLER_CERT_PEM` and `TELLER_KEY_PEM`: keep the container paths from `.env.example`; put the downloaded Teller certificate and private key in `secrets/teller_cert.pem` and `secrets/teller_key.pem`.
- `TELEGRAM_ENABLED`: `false` for now. A local process cannot receive Telegram webhooks without a public HTTPS tunnel.

Never commit `.env`, the Teller PEM files, or Telegram tokens. They are ignored by Git.

## Public URL convention

Pick one domain you control and keep these names stable:

- Production web app: `https://app.your-domain.com`
- Production API: `https://api.your-domain.com`
- Staging web app: `https://staging.your-domain.com`
- Staging API: `https://api.staging.your-domain.com`

Create DNS `A`/`AAAA` records pointing each name at the VPS, then configure a host-level reverse proxy such as Caddy or Nginx to terminate HTTPS and route each name to its environment's localhost port. The production Compose ports are `8081` for the web app and `8080` for the API; staging reserves `8181` and `8180`. `ops/Caddyfile.example` contains the complete routing template.

## Production host

The GitHub workflow expects a Docker-capable host with `/opt/steer/.env`, `/opt/steer/secrets`, and `/var/lib/steer`. Copy `ops/env/production.env.example` to `/opt/steer/.env`, replace every placeholder, place the Teller PEM files under `/opt/steer/secrets`, and authenticate Docker to `ghcr.io` once using a read-only package token.

The workflow copies `compose.prod.yml` and `ops/deploy.sh` to `/opt/steer`, then runs the script with the Git commit SHA as the image tag. A host-level reverse proxy should route the public web URL to `127.0.0.1:8081` and the public API URL to `127.0.0.1:8080`.

The future staging host configuration follows the same pattern using `ops/env/staging.env.example`, `/opt/steer-staging`, and the reserved staging ports. The current workflow intentionally deploys production only.

## GitHub production environment

Create a repository variable named `RELEASE_ENABLED` with value `false` while the VPS is unavailable. Change it to `true` only after all production configuration and secrets below are installed; until then, pushes to `prod` verify the code without publishing or deploying it.

Create an environment named `production` in the GitHub repository. Add these non-secret environment variables:

- `WEB_URL`: the stable production web URL.
- `API_URL`: the stable production API URL.
- `AUTH0_DOMAIN`: the Auth0 tenant domain.
- `AUTH0_CLIENT_ID`: the production Auth0 SPA client ID.
- `AUTH0_AUDIENCE`: the production Auth0 API identifier.
- `TELLER_APP_ID`: the production Teller application ID.

Add these GitHub environment secrets after the VPS exists:

- `PROD_HOST`: the VPS hostname or public IP.
- `PROD_PORT`: the SSH port, normally `22`.
- `PROD_USER`: the dedicated deployment user that owns `/opt/steer`.
- `PROD_SSH_PRIVATE_KEY`: the private half of a deployment-only SSH key.
- `PROD_SSH_KNOWN_HOSTS`: the verified host-key line for the VPS. Generate it with `ssh-keyscan -p <port> <host>` and verify the fingerprint separately before saving it.

When `RELEASE_ENABLED=true`, every push to the `prod` branch tests both applications, builds commit-addressed backend and frontend images, pushes them to GitHub Container Registry, and deploys that exact commit to the VPS. Concurrent releases are serialized so an older deployment cannot overtake a newer one.

## Manual deployment or rollback

```bash
DEPLOY_DIR=/opt/steer /opt/steer/deploy.sh <git-commit-sha>
```

Use the previous value from `/opt/steer/.deployed-image-tag` before deploying if you want to retain an easy rollback reference.
