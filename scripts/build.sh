#!/usr/bin/env bash
# Builds the release binary:
#   npm ci → typecheck → vite build → go test → go build (frontend embedded)
#
# Environment:
#   VERSION, COMMIT   injected into the binary (default: from git)
#   OUT               output path (default: release/homepage)
#   GOOS, GOARCH      honoured by go build for cross-compiling
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
OUT="${OUT:-release/homepage}"

echo "==> frontend"
(
  cd frontend
  npm ci
  npm run typecheck
  npm run build
)

echo "==> go test"
go test ./...

echo "==> go build ${VERSION} (${COMMIT})"
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
  -o "${OUT}" ./cmd/homepage

echo "==> ${OUT}"
