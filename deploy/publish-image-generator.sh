#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="${SOURCE_DIR:-$ROOT_DIR/deploy/static/image-generator}"
CONTAINER="${CONTAINER:-peter-sub2api-sub2api-1}"
TARGET_DIR="${TARGET_DIR:-/app/data/public/image-generator}"
RESTART="${RESTART:-0}"

required_files=(index.html main.js styles.css peterai.svg)

for file in "${required_files[@]}"; do
  if [[ ! -f "$SOURCE_DIR/$file" ]]; then
    echo "missing required file: $SOURCE_DIR/$file" >&2
    exit 1
  fi
done

if command -v node >/dev/null 2>&1; then
  node -c "$SOURCE_DIR/main.js"
else
  echo "warning: node not found; skipped main.js syntax check" >&2
fi

if ! sg docker -c "docker inspect '$CONTAINER' >/dev/null 2>&1"; then
  echo "container not found: $CONTAINER" >&2
  exit 1
fi

sg docker -c "docker exec '$CONTAINER' sh -lc 'mkdir -p \"$TARGET_DIR\"'"
for file in "${required_files[@]}"; do
  sg docker -c "docker cp '$SOURCE_DIR/$file' '$CONTAINER:$TARGET_DIR/$file'"
done
sg docker -c "docker exec '$CONTAINER' sh -lc 'chown -R sub2api:sub2api \"$TARGET_DIR\" 2>/dev/null || true'"

echo "local files:"
sha256sum "${required_files[@]/#/$SOURCE_DIR/}"

echo "container files:"
sg docker -c "docker exec '$CONTAINER' sh -lc 'sha256sum \"$TARGET_DIR/index.html\" \"$TARGET_DIR/main.js\" \"$TARGET_DIR/styles.css\" \"$TARGET_DIR/peterai.svg\"'"

if [[ "$RESTART" == "1" ]]; then
  sg docker -c "docker restart '$CONTAINER' >/dev/null"
  curl -fsS --max-time 10 http://127.0.0.1:18080/health
  echo
fi

echo "published image-generator static files from $SOURCE_DIR to $CONTAINER:$TARGET_DIR"
echo "if index.html changed main.js?v=..., also update settings.custom_menu_items and clear HTMLCache if needed."
