#!/bin/sh
# Builds the browser version of the port into build/web (index.html, the
# scripts, openorion2.wasm and its gzip). Serve it with the game files:
#   go run ./cmd/webserve [-addr :8080] [-auth user:pass]
set -e
cd "$(dirname "$0")/../.."
OUT=build/web
mkdir -p "$OUT"
GOOS=js GOARCH=wasm go build -trimpath -o "$OUT/openorion2.wasm" ./cmd/openorion2
gzip -9 -k -f "$OUT/openorion2.wasm"
install -m 644 "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/index.html web/fs.js web/host.js "$OUT/"
ls -l "$OUT"
