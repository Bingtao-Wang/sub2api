#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-all}"
GO_IMAGE="${GO_IMAGE:-golang:1.26.4-alpine}"
NODE_IMAGE="${NODE_IMAGE:-node:24-alpine}"
BACKEND_TEST_CMD="${BACKEND_TEST_CMD:-/usr/local/go/bin/go test ./internal/service -run OpenAIImages\\|Gallery -count=1}"
FRONTEND_TEST_CMD="${FRONTEND_TEST_CMD:-pnpm vitest run src/utils/__tests__/embedded-url.spec.ts src/utils/__tests__/clientConfig.spec.ts src/components/keys/__tests__/UseKeyModal.spec.ts}"

run_backend() {
  echo "== backend tests =="
  sg docker -c "docker run --rm \
    -v '$ROOT_DIR/backend:/app/backend' \
    -w /app/backend \
    -e GOPROXY='https://goproxy.cn,direct' \
    -e GOSUMDB='sum.golang.google.cn' \
    -e GOCACHE='/tmp/go-cache' \
    -e GOMODCACHE='/tmp/go-mod-cache' \
    -e PATH='/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin' \
    -e BACKEND_TEST_CMD='$BACKEND_TEST_CMD' \
    '$GO_IMAGE' \
    sh -lc 'eval \"\$BACKEND_TEST_CMD\"'"
}

run_frontend() {
  echo "== frontend tests =="
  sg docker -c "docker run --rm \
    -v '$ROOT_DIR/frontend:/app/frontend' \
    -v 'sub2api_frontend_node_modules:/app/frontend/node_modules' \
    -v 'sub2api_pnpm_store:/pnpm-store' \
    -w /app/frontend \
    -e PATH='/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin' \
    -e FRONTEND_TEST_CMD='$FRONTEND_TEST_CMD' \
    '$NODE_IMAGE' \
    sh -lc 'corepack enable && corepack prepare pnpm@9 --activate && pnpm config set store-dir /pnpm-store && pnpm install --frozen-lockfile && eval \"\$FRONTEND_TEST_CMD\"'"
}

case "$MODE" in
  backend)
    run_backend
    ;;
  frontend)
    run_frontend
    ;;
  all)
    run_backend
    run_frontend
    ;;
  *)
    echo "usage: $0 [backend|frontend|all]" >&2
    exit 2
    ;;
esac
