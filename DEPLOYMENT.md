# Deployment

Docker Compose runs the backend, frontend, Nginx gateway, SQLite backup, and
the one-time deployment setup stages. Compose reads `.env` to select either the
development or production override through `COMPOSE_FILE`.

## Development

1. Copy `.env.example` to the ignored `.env` file and fill in every empty
   value. Set `NGROK_DOMAIN` to the reserved ngrok domain and set
   `PUBLIC_BASE_URL` to its exact `https://` URL.
2. Start the pipeline:

   ```sh
   docker compose up -d --build --wait
   ```

3. Watch the one-time setup stage until it reports success:

   ```sh
   docker compose logs -f predeployment_register_webhook
   ```

The pipeline validates configuration, starts the backend and frontend, makes
the internal gateway healthy, starts ngrok against that gateway, verifies the
public `/healthz` route, then registers
`https://<NGROK_DOMAIN>/telegram/webhook` with Telegram.

The local gateway is also available at `http://localhost:8080`. Stop the stack
with `docker compose down`; this retains the named SQLite volume. Database
backups are written once daily to the ignored `./backups` directory.

## Production

1. Copy `.env.prod.example` to the ignored `.env` file and replace the example
   domain, public IP, email, and all empty application values.
2. Point `APP_DOMAIN` DNS at `DEPLOYMENT_PUBLIC_IP` and ensure ports 80 and 443
   reach the host.
3. Run:

   ```sh
   docker compose up -d --build --wait
   docker compose logs -f predeployment_register_webhook
   ```

Production first verifies DNS, creates temporary local TLS material, starts
Nginx for the ACME challenge, obtains a Let's Encrypt certificate, and then
registers the HTTPS Telegram webhook. Certbot checks renewal every 12 hours;
Nginx reloads every 12 hours to pick up a renewed certificate.

Use a different Telegram bot token from development before bringing up a
production stack: Telegram permits only one active webhook per bot.

## Diagnosing a failed stage

Each one-time service prints its stage name, observed failure, and remediation
without echoing credentials. Inspect the relevant stage with:

```sh
docker compose logs predeployment_validate
docker compose logs predeployment_verify_domain
docker compose logs predeployment_issue_certificate
docker compose logs predeployment_register_webhook
```
