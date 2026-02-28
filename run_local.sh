#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_PATH="/tmp/shimibot-build"

for env_file in "$ROOT_DIR/.env" "$ROOT_DIR/app/.env"; do
  if [[ -f "$env_file" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "$env_file"
    set +a
  fi
done

(
  cd "$ROOT_DIR"
  go build -o "$BIN_PATH" ./app
)

exec "$BIN_PATH" "$@"
