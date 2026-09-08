#!/bin/bash
# Run as root after activation. Tests a restore in a separate temporary database.
set -euo pipefail
root=/srv/olympguide
release=$(readlink -f "$root/current")
test -d "$release"
export RELEASE_ID=$(basename "$release")
compose=(docker compose --env-file "$root/secrets/backend.env" -f "$release/deploy/compose.yaml")
for path in /healthz /readyz /api/v1/universities /api/v1/olympiads; do
  "${compose[@]}" exec -T api wget -q -O /dev/null "http://127.0.0.1:8080$path"
  echo "$path: OK"
done
response=$("${compose[@]}" exec -T api wget -S -O /dev/null --post-data '{}' http://127.0.0.1:8080/api/v1/program/ 2>&1 || true)
grep -q 'HTTP/1.1 401' <<< "$response"
echo 'Anonymous program creation: rejected (401)'
"${compose[@]}" run --rm --no-deps migrate
"$release/deploy/backup.sh"
dump=$(find "$root/backups" -maxdepth 1 -type f -name 'postgres-*.dump' | sort | tail -n 1)
test -s "$dump"
check_db="olympguide_restore_check_$(date -u +%Y%m%d%H%M%S)_$$"
"${compose[@]}" exec -T db createdb -U olympguide "$check_db"
trap '"${compose[@]}" exec -T db dropdb -U olympguide "$check_db"' EXIT
"${compose[@]}" exec -T db pg_restore -U olympguide --exit-on-error -d "$check_db" < "$dump"
query="SELECT (SELECT count(*) FROM olympguide.university), (SELECT count(*) FROM olympguide.olympiad), (SELECT count(*) FROM olympguide.field_of_study), (SELECT count(*) FROM olympguide.benefit), (SELECT count(*) FROM public.databasechangelog);"
original=$("${compose[@]}" exec -T db psql -U olympguide -d olympguide -Atc "$query")
restored=$("${compose[@]}" exec -T db psql -U olympguide -d "$check_db" -Atc "$query")
test "$original" = "$restored"
echo "Backup restore: OK; universities|olympiads|fields|benefits|migrations = $restored"
systemctl is-enabled olympguide-backup.timer
"${compose[@]}" ps
