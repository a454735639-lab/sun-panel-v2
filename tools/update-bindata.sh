#!/usr/bin/env bash
set -euo pipefail

# Regenerate service/assets/bindata.go from service/assets/.
# NOTE: bindata.go is committed so builds stay deterministic across architectures.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR/service"

if [[ ! -d "assets" ]]; then
  echo "ERROR: ./service/assets directory not found (cwd: $(pwd))" >&2
  exit 1
fi

if ! command -v go-bindata >/dev/null 2>&1; then
  echo "go-bindata not found; installing..." >&2
  GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
  go install github.com/go-bindata/go-bindata/v3/go-bindata@latest
  export PATH="$GOBIN:$PATH"
fi

OUT="assets/bindata.go"

echo "Generating $OUT ..."
go-bindata -o="$OUT" -pkg=assets -ignore="bindata.go" assets/...

echo
echo "Done. If assets changed, commit the updated $OUT:"
if command -v git >/dev/null 2>&1; then
  git status --porcelain "$OUT" || true
fi
