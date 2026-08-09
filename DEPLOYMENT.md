# Deployment

This is the operational runbook for Steer's Docker Compose deployment. The
Python launcher validates `.env`, prepares the public endpoint, and then
replaces itself with `docker compose up --build`.

The host needs Docker Engine with Docker Compose and Python 3. Development also
needs the ngrok CLI installed and authenticated; Docker must work for the
deploying user without `sudo`.

## Development

1. Copy `.env.example` to the ignored `.env` file and fill in every empty
   value. This selects `DEPLOYMENT_ENV=dev` and the development Compose
   override.
2. Configure ngrok for the deploying user:

   ```sh
   ngrok config add-authtoken YOUR_NGROK_AUTHTOKEN
   ```
3. Start the pipeline:

   ```sh
   python3 cmd/deploy/deploy.py
   ```

The launcher starts ngrok, discovers its assigned HTTPS URL from ngrok's local
API, and supplies it to Compose as `PUBLIC_BASE_URL`. The backend uses that URL
for Telegram webhook registration and Teller links.

The local gateway is also available at `http://localhost:8080`. Stop the stack
with `docker compose down`; this retains the named SQLite volume. Database
backups are written once daily to the ignored `./backups` directory.

## Production

1. Copy `.env.prod.example` to the ignored `.env` file. This selects
   `DEPLOYMENT_ENV=prod` and the production Compose override.
2. Replace the example domain, public IP, email, and all empty application
   values. `DEPLOYMENT_PUBLIC_IP` can be found with:

   ```sh
   curl -4 https://icanhazip.com
   ```
3. Point the hostname in `PUBLIC_BASE_URL` at `DEPLOYMENT_PUBLIC_IP` and ensure
   ports 80 and 443 reach the host.
4. Run:

   ```sh
   python3 cmd/deploy/deploy.py
   ```

Production first verifies DNS, creates temporary local TLS material, starts
Nginx for the ACME challenge, then obtains a Let's Encrypt certificate.
Nginx reloads immediately after issuance and checks certificate renewal every
60 days.

Use a different Telegram bot token from development before bringing up a
production stack: Telegram permits only one active webhook per bot.

## Diagnosing a failed stage

The launcher prints safe validation errors. Inspect running services with:

```sh
docker compose logs predeployment_issue_certificate
docker compose logs backend
docker compose logs nginx
```

`docker compose down` stops the services while retaining the named SQLite data
volume. Database backups are written every 24 hours to the ignored `./backups`
directory.
