#!/usr/bin/env bash
# Ship the current commit to the Linode and rebuild. The repo is private, so
# instead of git pull on the box we tar the tracked files over SSH.
#   scripts/ship.sh [user@host] [ssh-key]
set -euo pipefail
HOST="${1:-root@172.239.49.30}"
KEY="${2:-$HOME/.ssh/tpn_linode_ed25519}"
S="ssh -i $KEY -o IdentitiesOnly=yes -o BatchMode=yes $HOST"
cd "$(dirname "$0")/.."
echo "== shipping $(git rev-parse --short HEAD) to $HOST"
$S 'mkdir -p /opt/tpn'
# Clear the old tree first (keeping .env) so files deleted in git don't linger
# and break the build.
$S 'find /opt/tpn -mindepth 1 -maxdepth 1 ! -name .env -exec rm -rf {} +'
git archive --format=tar HEAD | $S 'tar -x -C /opt/tpn'
$S 'cd /opt/tpn && test -f .env || { echo "missing /opt/tpn/.env on the server"; exit 1; }'
echo "== building (2-4 min)"
$S 'cd /opt/tpn && docker compose --profile prod up -d --build --remove-orphans 2>&1 | tail -3 && docker compose ps && sleep 3 && curl -s localhost/api/health && echo'
