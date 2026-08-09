# Steer

Steer is a financial tracking app that notifies users how purchases affect their
budgets in real time. Connect a read-only bank account through Telegram and
receive timely spending updates where you already communicate.

## Demo

_Incoming demo._

## How to get started

1. Open [OfficialSteerBot](https://t.me/OfficialSteerBot) and send `/connect`.
2. The bot provisions a unique link where you grant Steer read-only access to
   one bank account through Teller Connect.
3. Tell SteerBot how to modify your budget with `/budget`. For example:

   ```text
   /budget make my monthly budget 1700 total, 300 for gas, 200 for groceries,
   800 rent, and 400 other expenses
   ```

   _Budget commands are the intended next product workflow and are not yet
   implemented in the current bot._
4. That is it: Steer will notify you when a purchase affects your budget.

You can already inspect synchronized transactions with `/transactions_24h`,
`/transactions_3d`, `/transactions_7d`, or `/transactions_30d`. Add `verbose`
to any of those commands for the complete stored transaction record.

## Architecture

Telegram delivers commands to the Go backend, which coordinates Teller Connect,
Teller transaction/balance APIs, and SQLite storage before returning messages.
Docker Compose runs the backend, frontend, Nginx gateway, backups, and the
development ngrok or production TLS deployment layer.

### Architecture diagram

_Incoming architecture diagram._

## How to self-host

This guide assumes Go, Python 3, Docker Engine with Docker Compose, and Git are
installed. Development additionally requires the ngrok CLI; production needs a
public host with ports 80 and 443 reachable.

Copy the appropriate ignored environment template to `.env` and fill every
empty value. `DEPLOYMENT_ENV=dev` starts a dynamic ngrok tunnel and supplies its
URL automatically; `DEPLOYMENT_ENV=prod` verifies the configured public domain
and provisions HTTPS through Let's Encrypt.

### Dev deployment

> **Warning:** ngrok CLI must be installed and authenticated as the same normal
> user that runs the deployment command. Docker must work without `sudo`.

```sh
git clone https://github.com/yohanc3/steer.git
cd steer
cp .env.example .env
ngrok config add-authtoken YOUR_NGROK_AUTHTOKEN
python3 cmd/deploy/deploy.py
```

Fill the Telegram, Teller, certificate, and encryption values in `.env` before
starting. The launcher starts ngrok, discovers its assigned HTTPS URL, and then
hands the terminal to Docker Compose. The gateway is also reachable locally at
`http://localhost:8080`.

### Prod deployment

```sh
git clone https://github.com/yohanc3/steer.git
cd steer
cp .env.prod.example .env
curl -4 https://icanhazip.com
python3 cmd/deploy/deploy.py
```

Use the output of `curl -4 https://icanhazip.com` as `DEPLOYMENT_PUBLIC_IP`,
then point the DNS A record for the hostname in `PUBLIC_BASE_URL` to that IP.
Use the machine or VPS public IP, not a Docker container IP: container addresses
are private implementation details and change when containers are recreated.
The deployment validates DNS before requesting its initial Let's Encrypt
certificate.

### Good to know

- `.env` is ignored and must never be committed.
- A Telegram bot supports only one active webhook. Use a separate bot token for
  dev and prod, or the latest deployment will replace the other webhook.
- `docker compose down` stops services but retains the named SQLite data volume.
- The backend writes one timestamped SQLite backup every 24 hours to the ignored
  `./backups` directory.
- Production Nginx reloads after initial certificate issuance and checks renewal
  every 60 days.
- For local development, Docker group membership avoids typing `sudo` but grants
  root-equivalent control of the host; evaluate rootless Docker before using a
  shared or production machine.

## Development checks

Run the relevant checks before opening a pull request:

```sh
cd backend && go test ./... && go vet ./...
cd ../frontend && npm run build
```
