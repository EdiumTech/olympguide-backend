#!/bin/bash
# Invoked as root over SSH, after the release and env file have been uploaded.
set -euo pipefail
release=$1
[[ "$release" =~ ^[0-9]{8}T[0-9]{6}Z-[a-f0-9]{8}$ ]]
root=/srv/olympguide
dir="$root/releases/$release"
test -f "$root/.bootstrap-complete"
mountpoint -q "$root"
exec 9>"$root/.deploy-lock"
flock -n 9 || { echo "Another deployment is already running" >&2; exit 1; }
test -f "$dir/deploy/compose.yaml"
test -f "$root/secrets/backend.env"
chmod 600 "$root/secrets/backend.env"
export RELEASE_ID="$release"
export COMPOSE_PARALLEL_LIMIT=1
compose=(docker compose --env-file "$root/secrets/backend.env" -f "$dir/deploy/compose.yaml")
"${compose[@]}" config --quiet

# Build first: a compiler or registry failure must not interrupt the running API.
"${compose[@]}" pull --ignore-buildable
"${compose[@]}" build

if [ -L "$root/current" ]; then
  "$root/current/deploy/backup.sh"
fi
"${compose[@]}" up -d --wait --wait-timeout 180 db redis minio
# A failed migration aborts here. Never start the new API against a partial schema.
"${compose[@]}" run --rm --no-deps migrate
"${compose[@]}" up -d --wait --wait-timeout 180

"${compose[@]}" exec -T api wget -q -O /dev/null http://127.0.0.1:8080/readyz
"${compose[@]}" exec -T api wget -q -O /dev/null http://127.0.0.1:8080/api/v1/universities
previous=$(readlink -f "$root/current" || true)
[ -z "$previous" ] || ln -sfn "$previous" "$root/previous"
ln -sfn "$dir" "$root/current"
"${compose[@]}" images --format json > "$dir/images.lock.json"

install -m 0644 "$dir/deploy/olympguide-backup.service" /etc/systemd/system/
install -m 0644 "$dir/deploy/olympguide-backup.timer" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now olympguide-backup.timer
echo "Release $release is ready. Verify public HTTPS after DNS points to this VM."
