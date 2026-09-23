#!/bin/sh
# Builds the Go server -- which embeds web/dist, so run `npm run build` first --
# and starts it on a throwaway data directory.
set -e
root="$(cd "$(dirname "$0")/../.." && pwd)"
out="$root/web/e2e/.out"
rm -rf "$out"
mkdir -p "$out/data"
cd "$root"
go build -o "$out/stylelab" ./cmd/stylelab
exec env STYLELAB_DEV_INSECURE_KEY=1 STYLELAB_MASTER_KEY= \
  STYLELAB_DATA_DIR="$out/data" STYLELAB_ADDR="127.0.0.1:${E2E_APP_PORT:-8199}" \
  "$out/stylelab"
