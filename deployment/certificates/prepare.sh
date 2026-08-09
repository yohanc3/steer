#!/bin/sh
set -eu

if [ -f /certs/fullchain.pem ] && [ -f /certs/privkey.pem ]; then
  exit 0
fi

domain="${PUBLIC_BASE_URL#https://}"
domain="${domain%%/*}"

openssl req -x509 -nodes -newkey rsa:2048 -days 1 \
  -keyout /certs/privkey.pem \
  -out /certs/fullchain.pem \
  -subj "/CN=${domain}"
