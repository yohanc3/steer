#!/bin/sh
set -eu

if [ -f /certs/fullchain.pem ] && [ -f /certs/privkey.pem ]; then
  exit 0
fi

openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
  -keyout /certs/privkey.pem \
  -out /certs/fullchain.pem \
  -subj "/CN=${APP_DOMAIN}"
