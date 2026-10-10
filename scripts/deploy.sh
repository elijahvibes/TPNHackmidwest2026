#!/usr/bin/env bash
# Redeploy on the Linode. Run from the repo root on the server:
#   ./scripts/deploy.sh
# Pulls main, rebuilds the single app image (UI + Go), restarts, prints status.
set -euo pipefail
cd "$(dirname "$0")/.."
[ -f .env ] || { echo "missing .env (copy .env.example and fill in keys)"; exit 1; }
git pull --ff-only origin main
docker compose --profile prod up -d --build --remove-orphans
docker compose ps
sleep 2
curl -sf http://localhost/api/health && echo || { echo "health check failed"; docker compose logs --tail=50 app; exit 1; }
