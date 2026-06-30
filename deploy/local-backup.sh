#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKUP_DIR="${BACKUP_DIR:-/home/aihub/Peter_ws/sub2api-backups}"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/deploy/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/deploy/.env}"
PROJECT_NAME="${PROJECT_NAME:-peter-sub2api}"
APP_VOLUME="${APP_VOLUME:-${PROJECT_NAME}_sub2api_data}"
RETAIN_DAYS="${RETAIN_DAYS:-0}"
RETAIN_COUNT="${RETAIN_COUNT:-7}"

PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

timestamp="$(date +%Y%m%d_%H%M%S)"
run_dir="$BACKUP_DIR/$timestamp"
lock_dir="$BACKUP_DIR/.local-backup.lock"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

cleanup_lock() {
  rmdir "$lock_dir" 2>/dev/null || true
}

if ! mkdir -p "$BACKUP_DIR"; then
  echo "failed to create backup directory: $BACKUP_DIR" >&2
  exit 1
fi

if ! mkdir "$lock_dir" 2>/dev/null; then
  echo "another local backup appears to be running: $lock_dir" >&2
  exit 1
fi
trap cleanup_lock EXIT

mkdir -p "$run_dir"

db_file="$run_dir/sub2api_pg_${timestamp}.dump"
app_file="$run_dir/sub2api_appdata_${timestamp}.tar.gz"
manifest_file="$run_dir/manifest.txt"
sha_file="$run_dir/sha256sums.txt"

{
  echo "timestamp=$timestamp"
  echo "host=$(hostname)"
  echo "root_dir=$ROOT_DIR"
  echo "compose_file=$COMPOSE_FILE"
  echo "env_file=$ENV_FILE"
  echo "project_name=$PROJECT_NAME"
  echo "app_volume=$APP_VOLUME"
} > "$manifest_file"

log "starting local backup in $run_dir"

log "dumping postgres database"
sg docker -c "docker compose -f '$COMPOSE_FILE' --env-file '$ENV_FILE' exec -T postgres sh -lc 'PGPASSWORD=\"\$POSTGRES_PASSWORD\" pg_dump -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -Fc'" > "$db_file"

log "archiving app data volume $APP_VOLUME"
sg docker -c "docker run --rm -v '$APP_VOLUME:/data:ro' -v '$run_dir:/backup' alpine sh -lc 'tar -czf \"/backup/$(basename "$app_file")\" -C /data .'"

log "writing checksums"
(
  cd "$run_dir"
  sha256sum "$(basename "$db_file")" "$(basename "$app_file")" > "$(basename "$sha_file")"
)

log "backup files"
du -h "$db_file" "$app_file" "$manifest_file" "$sha_file" | tee -a "$manifest_file"

log "cleaning old backups: retain_days=$RETAIN_DAYS retain_count=$RETAIN_COUNT"
if [[ "$RETAIN_DAYS" =~ ^[0-9]+$ ]] && (( RETAIN_DAYS > 0 )); then
  find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -name '20????????_??????' -mtime "+$RETAIN_DAYS" -print -exec rm -rf {} +
fi

if [[ "$RETAIN_COUNT" =~ ^[0-9]+$ ]] && (( RETAIN_COUNT > 0 )); then
  mapfile -t old_dirs < <(find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -name '20????????_??????' -printf '%f\n' | sort -r | tail -n +"$((RETAIN_COUNT + 1))")
  for old in "${old_dirs[@]}"; do
    log "removing old backup $BACKUP_DIR/$old"
    rm -rf "$BACKUP_DIR/$old"
  done
fi

log "local backup completed: $run_dir"
