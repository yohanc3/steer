#!/bin/sh
set -eu

nginx -g 'daemon off;' &
nginx_pid="$!"

stop_nginx() {
  nginx -s quit
  wait "$nginx_pid"
  exit 0
}

trap stop_nginx INT TERM

while kill -0 "$nginx_pid" 2>/dev/null; do
  sleep 43200 &
  wait "$!" || break
  nginx -s reload
done

wait "$nginx_pid"
