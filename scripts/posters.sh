#!/usr/bin/env bash
# posters.sh — render one still frame per animation for the web pages.
#
# The poster is what a visitor sees before the ~2MB WASM module arrives, and it is
# the *whole* experience under prefers-reduced-motion, where the module is never
# fetched at all. It is produced by scripts/ansi2png.py, which uses the same
# sub-cell model as the browser painter — so the still and the live canvas agree
# on colour and structure rather than merely resembling each other.
#
# They are not pixel-identical: the poster is a fixed pane and the live canvas is
# fit-to-window. The viewer cross-fades, which covers the difference.
#
# Needs only go + python3. Output is gitignored — these are build products, the
# same as the .wasm modules, and a committed binary derived from source drifts.
#
#	scripts/posters.sh              # every animation with a cmd/wasm
#	scripts/posters.sh nebula       # just one

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/web/posters"

# Two shapes, because the live canvas REFLOWS rather than scaling: a phone renders
# roughly 65x70 cells, not a squeezed 190x48. A single landscape poster could only
# be cropped or letterboxed on a phone, and either way the hand-off to the live
# canvas would visibly jump.
#
# Landscape matches what the viewer picks on a 1920px screen (TARGET_COLS=190 in
# web/harness.js, cell 10). Portrait matches a 390px phone, where the cell clamps
# to its 6px floor; rendered at 2x so it stays crisp on a high-DPI display.
LANDSCAPE_COLS=190; LANDSCAPE_ROWS=48; LANDSCAPE_CW=10; LANDSCAPE_CH=20
PORTRAIT_COLS=65;   PORTRAIT_ROWS=70;  PORTRAIT_CW=12;  PORTRAIT_CH=24

mkdir -p "$OUT"

names=()
if [[ $# -gt 0 ]]; then
  names=("$@")
else
  for dir in "$ROOT"/examples/*/cmd/wasm; do
    [[ -d "$dir" ]] || continue
    names+=("$(basename "$(dirname "$(dirname "$dir")")")")
  done
fi

if [[ ${#names[@]} -eq 0 ]]; then
  echo "no animations with a cmd/wasm entrypoint — nothing to render" >&2
  exit 1
fi

for anim in "${names[@]}"; do
  src="$ROOT/examples/$anim"
  [[ -d "$src" ]] || { echo "no such animation: $anim" >&2; exit 1; }

  # Which frame to still. Tick 0 is rarely the animation at its best — torus is
  # near edge-on there, and a splash field has barely built up. Declared per
  # animation in meta.json ("posterTick"), because which frame reads best is a
  # judgement about that animation, made by looking at it.
  tick=$(python3 -c "import json,sys; print(json.load(open('$src/meta.json')).get('posterTick',0))" 2>/dev/null || echo 0)

  render() { # <suffix> <cols> <rows> <cw> <ch>
    local out="$OUT/$anim$1.png"
    # cmd/preview has no start-tick flag, so ask for two frames a stride apart and
    # keep the second. At tick 0 the stride is ignored and one frame comes back.
    (cd "$src" && go run ./cmd/preview frames 2 "${tick:-0}" "$2" "$3") \
      | awk '/^--- frame /{n++} n>=2 || NR==0' | tail -n +2 \
      | python3 "$ROOT/scripts/ansi2png.py" --cw "$4" --ch "$5" > "$out"
    echo "  $anim$1  tick ${tick:-0}  $2x$3 cells  $(( $2 * $4 ))x$(( $3 * $5 )) px  $(( $(wc -c < "$out") / 1024 ))KB"
  }
  render ""          "$LANDSCAPE_COLS" "$LANDSCAPE_ROWS" "$LANDSCAPE_CW" "$LANDSCAPE_CH"
  render "-portrait" "$PORTRAIT_COLS"  "$PORTRAIT_ROWS"  "$PORTRAIT_CW"  "$PORTRAIT_CH"
done
