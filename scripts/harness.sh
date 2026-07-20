#!/usr/bin/env bash
# harness.sh — build an animation to WASM and serve the authoring harness.
#
#   scripts/harness.sh examples/nebula          # build + serve on :8731
#   scripts/harness.sh examples/nebula 9000     # …on a different port
#
# The animation needs a cmd/wasm entrypoint (see examples/nebula/cmd/wasm).
# Requires: go. No node, no npm, no bundler — the harness is three static files.
set -euo pipefail

ANIM="${1:?usage: scripts/harness.sh <animation-dir> [port]}"
PORT="${2:-8731}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAME="$(basename "$ANIM")"

if [[ ! -d "$ROOT/$ANIM/cmd/wasm" ]]; then
  echo "error: $ANIM has no cmd/wasm entrypoint" >&2
  exit 1
fi

# wasm_exec.js must match the toolchain that built the module, so re-copy it
# rather than trusting whatever is already staged.
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$ROOT/web/wasm_exec.js"

echo "building $NAME -> web/$NAME.wasm"
(cd "$ROOT/$ANIM" && GOOS=js GOARCH=wasm go build -o "$ROOT/web/$NAME.wasm" ./cmd/wasm)
echo "  $(du -h "$ROOT/web/$NAME.wasm" | cut -f1) ($(gzip -9 -c "$ROOT/web/$NAME.wasm" | wc -c | awk '{printf "%dKB", $1/1024}') gzipped)"

# Manifest of everything built so far, so the page's picker matches what the
# pages workflow produces. Built from the .wasm files present rather than from
# examples/, since only what you've actually built is servable.
( cd "$ROOT/web" && printf '%s\n' *.wasm \
    | sed 's/\.wasm$//' \
    | awk 'BEGIN{printf "["} {printf "%s\"%s\"", (NR>1 ? "," : ""), $0} END{print "]"}' \
    > animations.json )
echo "  built: $(cat "$ROOT/web/animations.json")"

echo "serving http://localhost:$PORT/?anim=$NAME  (Ctrl-C to stop)"
cd "$ROOT/web" && exec python3 -m http.server "$PORT"
