# Deployment

The host launcher starts ngrok for development, discovers its public URL, then
replaces itself with Docker Compose. Compose runs the backend, frontend, and
Nginx gateway.

## Development

1. Copy `.env.example` to the ignored `.env` file and fill in every empty
   value. Configure the ngrok CLI locally with its authtoken.
2. Start the pipeline:

   ```sh
   python3 cmd/deploy/deploy.py
   ```

The launcher discovers ngrok's assigned HTTPS URL before Compose starts. The
backend uses that URL for Telegram webhook registration and Teller links.

The local gateway is also available at `http://localhost:8080`. Stop the stack
with `docker compose down`; this retains the named SQLite volume. Database
backups are written once daily to the ignored `./backups` directory.

## Production

1. Copy `.env.prod.example` to the ignored `.env` file and replace the example
   domain, public IP, email, and all empty application values.
2. Point the hostname in `PUBLIC_BASE_URL` at `DEPLOYMENT_PUBLIC_IP` and ensure ports 80 and 443
   reach the host.
3. Run:

   ```sh
   python3 cmd/deploy/deploy.py
   ```

Production first verifies DNS, creates temporary local TLS material, and
obtains a Let's Encrypt certificate. Nginx reloads immediately after issuance
and checks certificate renewal every 60 days.

Use a different Telegram bot token from development before bringing up a
production stack: Telegram permits only one active webhook per bot.

## Diagnosing a failed stage

The launcher prints safe validation errors. Inspect running services with:

```sh
docker compose logs predeployment_issue_certificate
docker compose logs backend
docker compose logs nginx
```
