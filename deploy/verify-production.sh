#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-$ROOT_DIR/deploy/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/deploy/.env}"
BASE_URL="${BASE_URL:-https://api.peterai.cc.cd}"
LOCAL_URL="${LOCAL_URL:-http://127.0.0.1:18080}"
CUSTOM_PAGE="${CUSTOM_PAGE:-$BASE_URL/custom/6768ebe29836ec72}"
CONTAINER="${CONTAINER:-peter-sub2api-sub2api-1}"
STATIC_DIR="${STATIC_DIR:-$ROOT_DIR/deploy/static/image-generator}"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

section() {
  printf '\n== %s ==\n' "$1"
}

section "container status"
sg docker -c "docker compose -f '$COMPOSE_FILE' --env-file '$ENV_FILE' ps"
image="$(sg docker -c "docker inspect '$CONTAINER' --format '{{.Config.Image}}'")"
echo "image=$image"

section "health"
curl -fsS --max-time 10 "$LOCAL_URL/health"
echo
curl -k -fsS --max-time 15 "$BASE_URL/health"
echo

section "custom iframe url"
custom_html="$tmpdir/custom-page.html"
curl -k -fsS --max-time 15 "$CUSTOM_PAGE" -o "$custom_html"
iframe_path="$(grep -o 'image-generator/?v=[^"\\]*' "$custom_html" | head -1 || true)"
if [[ -z "$iframe_path" ]]; then
  echo "image-generator iframe URL not found in $CUSTOM_PAGE" >&2
  exit 1
fi
echo "$iframe_path"

section "image-generator index"
index_html="$tmpdir/image-generator-index.html"
curl -k -fsS --max-time 15 "$BASE_URL/image-generator/" -o "$index_html"
main_ref="$(grep -o 'main\.js?v=[^"]*' "$index_html" | head -1 || true)"
if [[ -z "$main_ref" ]]; then
  echo "main.js version not found in image-generator index" >&2
  exit 1
fi
echo "$main_ref"

section "public main.js"
main_js="$tmpdir/image-generator-main.js"
curl -k -fsS --max-time 15 "$BASE_URL/image-generator/$main_ref" -o "$main_js"
node -c "$main_js"
node - "$main_js" <<'NODE'
const fs = require('fs')
const file = process.argv[2]
const s = fs.readFileSync(file, 'utf8')
let bad = 0
s.split(/\n/).forEach((line, i) => {
  if (!line.includes('.forEach')) return
  for (let p = line.indexOf('$('); p !== -1; p = line.indexOf('$(', p + 1)) {
    if (p === 0 || line[p - 1] !== '$') {
      bad++
      console.log(`${i + 1}:${line}`)
    }
  }
})
console.log('single_dollar_forEach =', bad)
if (bad !== 0) process.exit(1)
NODE

section "image prices"
prices="$(sg docker -c "docker compose -f '$COMPOSE_FILE' --env-file '$ENV_FILE' exec -T postgres sh -lc 'psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -P pager=off -Atc \"select min(image_price_1k), max(image_price_1k), min(image_price_2k), max(image_price_2k), min(image_price_4k), max(image_price_4k) from groups;\"'")"
echo "$prices"
if [[ "$prices" != "0.10000000|0.10000000|0.10000000|0.10000000|0.10000000|0.10000000" ]]; then
  echo "unexpected image prices" >&2
  exit 1
fi

section "static file hashes"
if [[ -d "$STATIC_DIR" ]]; then
  echo "repo static files:"
  sha256sum "$STATIC_DIR/index.html" "$STATIC_DIR/main.js" "$STATIC_DIR/styles.css" "$STATIC_DIR/peterai.svg"
fi
echo "container static files:"
sg docker -c "docker exec '$CONTAINER' sh -lc 'sha256sum /app/data/public/image-generator/index.html /app/data/public/image-generator/main.js /app/data/public/image-generator/styles.css /app/data/public/image-generator/peterai.svg'"

echo
echo "production verification passed"
