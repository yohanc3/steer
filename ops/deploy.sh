#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <image-tag>" >&2
  exit 2
fi

image_tag="$1"
deploy_dir="${DEPLOY_DIR:-/opt/steer}"
compose_file="$deploy_dir/compose.release.yaml"
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
export IMAGE_NAMESPACE="${IMAGE_NAMESPACE:-yohanc3}"
export APP_ENV_FILE="$env_file"

cd "$deploy_dir"
docker compose --env-file "$env_file" -f "$compose_file" config --quiet
docker compose --env-file "$env_file" -f "$compose_file" pull
docker compose --env-file "$env_file" -f "$compose_file" up --detach --remove-orphans --wait --wait-timeout 90

backend_port="$(docker compose --env-file "$env_file" -f "$compose_file" port backend 8080 | awk -F: '{print $NF}')"
frontend_port="$(docker compose --env-file "$env_file" -f "$compose_file" port frontend 8080 | awk -F: '{print $NF}')"
curl --fail --silent "http://127.0.0.1:$backend_port/healthz" >/dev/null
curl --fail --silent "http://127.0.0.1:$frontend_port/" >/dev/null

printf '%s\n' "$image_tag" > "$deploy_dir/.deployed-image-tag"
echo "deployed Steer image tag $image_tag"
