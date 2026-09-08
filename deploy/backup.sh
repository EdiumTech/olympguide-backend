#!/bin/bash
set -euo pipefail
root=/srv/olympguide
mountpoint -q "$root"
umask 077
install -d -m 0700 "$root/backups"
exec 9>"$root/backups/.backup-lock"
flock -n 9 || exit 0
release=$(readlink -f "$root/current")
export RELEASE_ID=$(basename "$release")
stamp=$(date -u +%Y%m%dT%H%M%SZ)
target="$root/backups/postgres-$stamp.dump"
docker compose --env-file "$root/secrets/backend.env" -f "$release/deploy/compose.yaml" \
  exec -T db pg_dump -U olympguide -d olympguide -Fc > "$target.partial"
test -s "$target.partial"
mv "$target.partial" "$target"
# Only completed dumps created by this script are subject to retention.
find "$root/backups" -maxdepth 1 -type f -name 'postgres-*.dump' -mtime +7 -delete
echo "Created $(basename "$target")"
