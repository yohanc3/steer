# Local Secrets

Place local Teller certificate files here when the integration needs them:

- `teller_cert.pem`
- `teller_key.pem`

PEM files are ignored by Git. Production and staging certificates live on their respective hosts and are mounted read-only into the backend container.
