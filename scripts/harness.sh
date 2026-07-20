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

# Manifest of everything built so far, so the page matches what the pages workflow
# produces. Keyed on the .wasm files present rather than on examples/, since only
# what you have actually built is servable — then merged with each animation's
# meta.json, which is where the gallery gets its titles, resolutions and accents.
python3 "$ROOT/scripts/manifest.py" "$ROOT"

# Posters belong to the pages build, not the authoring loop: each one costs a full
# preview run, and the viewer treats a missing poster as "start on black" rather
# than as an error. Set POSTERS=1 when you want to check the still a visitor sees
# before the module lands.
if [[ "${POSTERS:-}" == "1" ]]; then
  "$ROOT/scripts/posters.sh" "$NAME"
fi

echo "serving http://localhost:$PORT/view.html?anim=$NAME&dev  (Ctrl-C to stop)"
echo "  gallery: http://localhost:$PORT/"
cd "$ROOT/web" && exec python3 -m http.server "$PORT"
