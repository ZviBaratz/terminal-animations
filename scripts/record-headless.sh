#!/usr/bin/env bash
#
# record-headless.sh — the beauty gate without vhs. Turn a `frames` dump into a
# seamless-loop GIF (and a truecolor MP4) using only ffmpeg + python3, so you can
# *watch the motion in colour* on a box with no vhs/ttyd and no live terminal.
#
# It runs your frames-dump command, splits the dump on `--- frame N ---` headers
# into per-frame chunks, rasterizes each through ansi2png.py, then assembles a
# GIF (256-colour, motion-stable Bayer dither, loops forever) and an MP4.
#
# Usage:
#   scripts/record-headless.sh [options] -- <frames-command...>
#
# Options:
#   -o, --out BASE     output basename                 (default: out/preview)
#                      writes BASE.gif and/or BASE.mp4
#       --fps N        GIF/MP4 playback framerate       (default: 20)
#       --width PX     output width (height auto, even) (default: 640)
#       --cw N         ansi2png cell px width           (default: 8)
#       --ch N         ansi2png cell px height          (default: 16)
#       --no-gif       skip the GIF
#       --no-mp4       skip the MP4
#       --ansi2png P   path to ansi2png.py             (default: alongside this script)
#   -h, --help
#
# Example (a nebula loop of period 1080, sped up to a 6 s GIF at 20 fps):
#   # 120 frames × stride 9 = 1080 ticks = exactly one loop → a seamless GIF.
#   scripts/record-headless.sh -o out/nebula -- \
#     go run ./cmd/preview frames 120 9 220 56
#
# Seam note: for a seamless loop make (frame count × stride) equal exactly one loop
# `period`, so the dump spans 0 … period−stride and wrapping back to frame 0 is just
# one more stride — continuous. A full-motion field compresses poorly as a GIF (a
# 640px/120-frame nebula is several MB); the MP4 is the smaller, sharper artifact.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

OUT="out/preview"
FPS=20
WIDTH=640
CW=8
CH=16
DO_GIF=1
DO_MP4=1
ANSI2PNG="$HERE/ansi2png.py"

usage() { sed -n '2,/^set -euo/p' "$0" | sed 's/^# \{0,1\}//; s/^#$//' | sed '$d'; }

while [[ $# -gt 0 ]]; do
	case "$1" in
		-o|--out)     OUT="$2"; shift 2 ;;
		--fps)        FPS="$2"; shift 2 ;;
		--width)      WIDTH="$2"; shift 2 ;;
		--cw)         CW="$2"; shift 2 ;;
		--ch)         CH="$2"; shift 2 ;;
		--no-gif)     DO_GIF=0; shift ;;
		--no-mp4)     DO_MP4=0; shift ;;
		--ansi2png)   ANSI2PNG="$2"; shift 2 ;;
		-h|--help)    usage; exit 0 ;;
		--)           shift; break ;;
		*)            echo "record-headless.sh: unknown option '$1'" >&2; exit 2 ;;
	esac
done

FRAMES="$*"
if [[ -z "$FRAMES" ]]; then
	echo "record-headless.sh: no frames command — put it after '--' (see --help)" >&2
	exit 2
fi
if [[ "$DO_GIF" -eq 0 && "$DO_MP4" -eq 0 ]]; then
	echo "record-headless.sh: --no-gif and --no-mp4 together leave nothing to do" >&2
	exit 2
fi

missing=""
for dep in ffmpeg python3; do
	command -v "$dep" >/dev/null 2>&1 || missing="$missing $dep"
done
if [[ -n "$missing" ]]; then
	echo "record-headless.sh: missing on PATH:$missing (needs ffmpeg + python3)" >&2
	exit 1
fi
if [[ ! -f "$ANSI2PNG" ]]; then
	echo "record-headless.sh: ansi2png.py not found at '$ANSI2PNG' (pass --ansi2png)" >&2
	exit 1
fi

WORK="$(mktemp -d "${TMPDIR:-/tmp}/record-headless-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

# 1. Run the frames command and split its dump into per-frame .ansi chunks. Each
#    `--- frame N ---` header starts a new chunk; the header line itself is dropped
#    (ansi2png would skip it anyway, but a clean single-frame chunk keeps the PNG
#    sized to exactly that frame).
echo "→ dumping frames: $FRAMES"
bash -c "$FRAMES" | awk -v dir="$WORK" '
	/^--- frame [0-9]+ ---$/ { if (fn) close(fn); fn = sprintf("%s/%05d.ansi", dir, n++); next }
	fn { print > fn }
'

shopt -s nullglob
chunks=("$WORK"/*.ansi)
nframes=${#chunks[@]}
if [[ "$nframes" -eq 0 ]]; then
	echo "record-headless.sh: the frames command produced no '--- frame N ---' frames" >&2
	exit 1
fi

# 2. Rasterize each chunk to a PNG with ansi2png.py (numbered so ffmpeg's image2
#    demuxer reads them in order).
echo "→ rasterizing $nframes frames through ansi2png.py (${CW}×${CH}px cells)"
for chunk in "${chunks[@]}"; do
	python3 "$ANSI2PNG" --cw "$CW" --ch "$CH" < "$chunk" > "${chunk%.ansi}.png"
done

mkdir -p "$(dirname "$OUT")"
SCALE="scale=${WIDTH}:-2:flags=lanczos"

# 3a. GIF: two-pass palette (a full-stats global palette, then ordered/Bayer dither —
#     stable under motion, no temporal shimmer — see references/techniques.md), looped.
if [[ "$DO_GIF" -eq 1 ]]; then
	echo "→ encoding ${OUT}.gif (${WIDTH}px, ${FPS}fps, seamless loop)"
	ffmpeg -hide_banner -loglevel error -y -framerate "$FPS" -start_number 0 -i "$WORK/%05d.png" \
		-vf "${SCALE},palettegen=max_colors=256:stats_mode=full" "$WORK/pal.png"
	ffmpeg -hide_banner -loglevel error -y -framerate "$FPS" -start_number 0 -i "$WORK/%05d.png" -i "$WORK/pal.png" \
		-lavfi "${SCALE}[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=3" \
		-loop 0 "${OUT}.gif"
	echo "✓ wrote ${OUT}.gif"
fi

# 3b. MP4: truecolor H.264 — smaller than the GIF at higher fidelity; the better
#     "optimal" artifact for a full-motion field.
if [[ "$DO_MP4" -eq 1 ]]; then
	echo "→ encoding ${OUT}.mp4 (${WIDTH}px, ${FPS}fps)"
	ffmpeg -hide_banner -loglevel error -y -framerate "$FPS" -start_number 0 -i "$WORK/%05d.png" \
		-vf "$SCALE" -c:v libx264 -crf 20 -preset slow -pix_fmt yuv420p -movflags +faststart "${OUT}.mp4"
	echo "✓ wrote ${OUT}.mp4"
fi
