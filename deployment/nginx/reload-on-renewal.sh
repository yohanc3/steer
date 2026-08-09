#!/bin/sh
set -eu

nginx -g 'daemon off;' &
nginx_pid="$!"
domain="${PUBLIC_BASE_URL#https://}"
domain="${domain%%/*}"

renew_certificate() {
  previous_certificate="$(sha256sum /etc/letsencrypt/live/"$domain"/fullchain.pem)"
  if certbot renew --webroot --webroot-path /var/www/certbot; then
    current_certificate="$(sha256sum /etc/letsencrypt/live/"$domain"/fullchain.pem)"
    if [ "$previous_certificate" != "$current_certificate" ]; then
      cp -L /etc/letsencrypt/live/"$domain"/fullchain.pem /etc/nginx/certs/fullchain.pem
      cp -L /etc/letsencrypt/live/"$domain"/privkey.pem /etc/nginx/certs/privkey.pem
      nginx -s reload
    fi
  fi
}

watch_initial_certificate() {
  while inotifywait --quiet --event create --event moved_to /etc/nginx/certs; do
    if [ -f /etc/nginx/certs/.reload ]; then
      nginx -s reload
      rm /etc/nginx/certs/.reload
    fi
  done
}

watch_initial_certificate &
watcher_pid="$!"

(
  while sleep 5184000; do
    renew_certificate
  done
) &
renewal_pid="$!"

stop_nginx() {
  kill "$watcher_pid" "$renewal_pid" 2>/dev/null || true
  nginx -s quit
  wait "$nginx_pid"
  exit 0
}

trap stop_nginx INT TERM

wait "$nginx_pid"
