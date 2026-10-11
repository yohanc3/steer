#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <image-tag>" >&2
  exit 2
fi

image_tag="$1"
deploy_dir="${DEPLOY_DIR:-/opt/steer}"
compose_file="$deploy_dir/compose.prod.yml"
env_file="${APP_ENV_FILE:-$deploy_dir/.env}"

if [[ ! -f "$compose_file" ]]; then
  echo "missing Compose file: $compose_file" >&2
  exit 1
fi

if [[ ! -f "$env_file" ]]; then
  echo "missing environment file: $env_file" >&2
  exit 1
fi

export IMAGE_TAG="$image_tag"
export APP_ENV_FILE="$env_file"

cd "$deploy_dir"

set -a
# The host-owned file contains trusted deployment configuration and secrets.
# shellcheck disable=SC1090
source "$env_file"
set +a

# The requested release tag always wins over any stale value in the env file.
export IMAGE_TAG="$image_tag"
export APP_ENV_FILE="$env_file"

docker compose --env-file "$env_file" -f "$compose_file" config --quiet
docker compose --env-file "$env_file" -f "$compose_file" pull
docker compose --env-file "$env_file" -f "$compose_file" up \
  --detach \
  --remove-orphans \
  --wait \
  --wait-timeout 90

curl --fail --silent "http://127.0.0.1:${BACKEND_HOST_PORT:-8080}/healthz" >/dev/null
curl --fail --silent "http://127.0.0.1:${FRONTEND_HOST_PORT:-8081}/healthz" >/dev/null

printf '%s\n' "$image_tag" > "$deploy_dir/.deployed-image-tag"
echo "deployed Steer image tag $image_tag"
