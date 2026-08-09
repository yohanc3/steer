#!/bin/sh
set -eu

nginx -g 'daemon off;' &
nginx_pid="$!"
renewal_due_at="$(( $(date +%s) + 5184000 ))"
domain="${PUBLIC_BASE_URL#https://}"
domain="${domain%%/*}"

stop_nginx() {
  nginx -s quit
  wait "$nginx_pid"
  exit 0
}

trap stop_nginx INT TERM

while kill -0 "$nginx_pid" 2>/dev/null; do
  if [ -f /etc/nginx/certs/.reload ]; then
    nginx -s reload
    rm /etc/nginx/certs/.reload
  fi

  if [ "$(date +%s)" -ge "$renewal_due_at" ]; then
    previous_certificate="$(sha256sum /etc/letsencrypt/live/"$domain"/fullchain.pem)"
    if certbot renew --webroot --webroot-path /var/www/certbot; then
      current_certificate="$(sha256sum /etc/letsencrypt/live/"$domain"/fullchain.pem)"
      if [ "$previous_certificate" != "$current_certificate" ]; then
        cp -L /etc/letsencrypt/live/"$domain"/fullchain.pem /etc/nginx/certs/fullchain.pem
        cp -L /etc/letsencrypt/live/"$domain"/privkey.pem /etc/nginx/certs/privkey.pem
        nginx -s reload
      fi
    fi
    renewal_due_at="$(( $(date +%s) + 5184000 ))"
  fi

  sleep 1 &
  wait "$!" || break
done

wait "$nginx_pid"
