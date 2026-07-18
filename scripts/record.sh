#!/usr/bin/env bash
#
# record.sh — the beauty gate. Record a short GIF of a terminal animation's
# preview so you can *watch the motion in colour* and tune it.
#
# It generates a vhs tape on the fly and runs it. Needs vhs + ttyd + ffmpeg.
#
# Usage:
#   scripts/record.sh [options] -- <run-command...>
#
# Options:
#   -o, --out FILE      output gif                 (default: out/preview.gif)
#       --build CMD     a build step run hidden first, so the recording never
#                       captures compilation (recommended over `go run`)
#       --font N        vhs FontSize                (default: 14)
#       --width PX      vhs Width in pixels         (default: 640)
#       --height PX     vhs Height in pixels        (default: 384)
#       --padding PX    vhs Padding                 (default: 16)
#       --fps N         vhs Framerate               (default: 15)
#       --seconds N     seconds to record           (default: 6)
#   -h, --help
#
# Example (standalone animation with a cmd/preview loop):
#   scripts/record.sh --build "go build -o /tmp/anim ./cmd/preview" -- /tmp/anim
#
# Sizing note (from fresco's demo tape): at FontSize 14 / Padding 16 a
# 640x384 window is ~64x21 cells. The animation must never be wider than the
# terminal or every row wraps. Keep size and framerate modest — a field that
# changes every cell every frame compresses poorly and a bigger GIF balloons.
set -euo pipefail

OUT="out/preview.gif"
BUILD=""
FONT=14
WIDTH=640
HEIGHT=384
PADDING=16
FPS=15
SECS=6

usage() { sed -n '2,/^set -euo/p' "$0" | sed 's/^# \{0,1\}//; s/^#$//' | sed '$d'; }

while [[ $# -gt 0 ]]; do
	case "$1" in
		-o|--out)     OUT="$2"; shift 2 ;;
		--build)      BUILD="$2"; shift 2 ;;
		--font)       FONT="$2"; shift 2 ;;
		--width)      WIDTH="$2"; shift 2 ;;
		--height)     HEIGHT="$2"; shift 2 ;;
		--padding)    PADDING="$2"; shift 2 ;;
		--fps)        FPS="$2"; shift 2 ;;
		--seconds)    SECS="$2"; shift 2 ;;
		-h|--help)    usage; exit 0 ;;
		--)           shift; break ;;
		*)            echo "record.sh: unknown option '$1'" >&2; exit 2 ;;
	esac
done

RUN="$*"
if [[ -z "$RUN" ]]; then
	echo "record.sh: no run command — put it after '--' (see --help)" >&2
	exit 2
fi

missing=""
for dep in vhs ttyd ffmpeg; do
	command -v "$dep" >/dev/null 2>&1 || missing="$missing $dep"
done
if [[ -n "$missing" ]]; then
	echo "record.sh: missing on PATH:$missing" >&2
	echo "The GIF recorder needs all three: https://github.com/charmbracelet/vhs#installation" >&2
	exit 1
fi

mkdir -p "$(dirname "$OUT")"
# Trailing X's only — BSD/macOS mktemp rejects a template with a suffix after the
# X's; vhs reads the tape by content, so it needs no .tape extension.
tape="$(mktemp "${TMPDIR:-/tmp}/anim-XXXXXX")"
trap 'rm -f "$tape"' EXIT

{
	echo "Output $OUT"
	echo "Require bash"
	echo "Set Shell bash"
	echo "Set FontSize $FONT"
	echo "Set Width $WIDTH"
	echo "Set Height $HEIGHT"
	echo "Set Padding $PADDING"
	echo "Set Framerate $FPS"
	echo "Set LoopOffset 20%"
	if [[ -n "$BUILD" ]]; then
		echo "Hide"
		echo "Type \"$BUILD\" Enter"
		echo "Wait"
		echo "Type \"clear\" Enter"
		echo "Show"
	fi
	echo "Type \"$RUN\" Enter"
	echo "Sleep ${SECS}s"
	echo "Ctrl+C"
	echo "Sleep 300ms"
} >"$tape"

echo "→ recording $OUT  (${WIDTH}x${HEIGHT}px, ${FPS}fps, ${SECS}s)"
vhs "$tape"
echo "✓ wrote $OUT"
