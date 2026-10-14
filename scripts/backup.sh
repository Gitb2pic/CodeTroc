#!/usr/bin/env sh
# Sauvegarde PostgreSQL de la production dans ./backups (format custom, gardé 14 jours).
# Usage (cron quotidien) : 15 3 * * * cd /opt/codetroc && ./scripts/backup.sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p backups
chmod 700 backups
out="backups/codetroc-$(date +%Y%m%d-%H%M%S).dump"
docker compose -f compose.yaml -f compose.prod.yaml exec -T db \
  pg_dump -U codetroc -d codetroc -Fc > "$out"
chmod 600 "$out"
find backups -name 'codetroc-*.dump' -mtime +14 -delete
echo "Sauvegarde : $out"
