#!/usr/bin/env bash
# Синхронизация на сервер и перезапуск (Git Bash / WSL / Linux).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

SERVER="${SERVER:-root@217.114.1.173}"
# По умолчанию ~/tunnel на удалённой машине (не локальный $HOME). Явно: REMOTE_PATH=/opt/tunnel bash deploy.sh
: "${REMOTE_PATH:=$(ssh "$SERVER" 'printf %s "$HOME"')/tunnel}"
# По умолчанию не тянем свежие образы, чтобы лишний раз не трогать Traefik/ACME.
# Для явного обновления образов: PULL_IMAGES=1 bash deploy.sh
PULL_IMAGES="${PULL_IMAGES:-0}"

if [[ ! -f .env ]]; then
  echo "Нет .env — скопируйте .env.example в .env" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "Нужен python3 для сборки frps.toml (либо скопируйте frps.toml.example → frps.toml и пропишите auth.token вручную)." >&2
  exit 1
fi
python3 "$ROOT/scripts/generate_frps_toml.py"

ssh "$SERVER" "mkdir -p '$REMOTE_PATH/letsencrypt' && (test -f '$REMOTE_PATH/letsencrypt/acme.json' || echo '{}' > '$REMOTE_PATH/letsencrypt/acme.json') && chmod 600 '$REMOTE_PATH/letsencrypt/acme.json'"

scp docker-compose.yml .env frps.toml traefik.yml traefik-dynamic.yml "$SERVER:$REMOTE_PATH/"
scp -r "$ROOT/whitelist-guard" "$SERVER:$REMOTE_PATH/"

if [[ "$PULL_IMAGES" == "1" ]]; then
  # tunnel-whitelist-guard:local собирается на сервере; pull только у образов из registry.
  ssh "$SERVER" "cd '$REMOTE_PATH' && docker compose pull traefik frps && docker compose up -d --build --remove-orphans"
else
  ssh "$SERVER" "cd '$REMOTE_PATH' && docker compose up -d --build --remove-orphans"
fi

echo "Готово: $SERVER:$REMOTE_PATH"
