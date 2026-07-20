#!/usr/bin/env bash
#
# bake.sh — author-time build step that turns the source bust PNG into the committed,
# watermark-free frame sheet (frames.png) that bust.go embeds and replays offline.
#
# What is baked vs. synthesized live (this split is the whole point):
#   BAKED HERE  — only the *subject*: the matted bust, as a seamless pseudo-3D turn, with an
#                 alpha channel. A still can't reveal the statue's back, so the "turn" is a
#                 gentle perspective keystone (yaw = A·sinθ) that reads as rotation and loops.
#   SYNTHESIZED — the *atmosphere* (drifting mist, a sweeping directional key + rim light, a
#     IN bust.go   shifting backdrop) is a pure function of tick in Go. Nothing here bakes a
#                 light or a background: those move so they must stay live.
#
# Pipeline (real image tools, build time only — nothing here runs at animation run time):
#   1. clean.py — matte the bust off its watermarked field → a clean RGBA cut-out
#   2. Pillow   — place the cut-out, warp N seamless turn frames, premultiply, downscale to
#                 the half-block pixel grid, stack into one RGBA PNG sheet → frames.png
#
# Frames are baked *premultiplied* (RGB already × alpha) so the downscale carries no dark
# edge fringe and bust.go composites with the simple "over" rule premult + bg·(1−α).
#
# The committed frames.png is watermark-free; the watermarked source is NOT copied into the
# repo. Re-run only to regenerate the artifact, then refresh the golden:
#   ./bake.sh && go test ./... -run TestGolden -update
#
# Usage: ./bake.sh [SRC_PNG]        (default source: ~/Downloads/bust.png)
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC="${1:-$HOME/Downloads/bust.png}"

W=140    # half-block pixel grid width  (= cell columns = frameW)
H2=140   # half-block pixel grid height (= 2 * 70 cell rows)
N=72     # frame count / loop period

# --- turn taste constants (sweep these by eye against the ansi2png filmstrip) ---
WK=320       # working resolution for the warp, downscaled to WxH2 after (crisper edges)
FIT=0.80     # subject fills this fraction of the working canvas (rest = transparent room)
AMP=0.55     # yaw signal amplitude in [0,1] — how far the bust turns each way
EX=0.110     # horizontal inset of the receding edge at full yaw (fraction of width)
EY=0.060     # vertical foreshorten of the receding edge at full yaw (fraction of height)
TX=0.022     # horizontal swing of the whole subject, opposite the turn (parallax feel)

[[ -f "$SRC" ]] || { echo "bake.sh: source PNG not found: $SRC" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "bake.sh: needs python3 (Pillow)" >&2; exit 1; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/bake-bust-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

echo "→ clean.py: matte the bust → RGBA cut-out"
python3 "$HERE/clean.py" "$SRC" "$WORK/cut.png"

echo "→ Pillow: warping $N seamless turn frames → frames.png"
python3 - "$WORK/cut.png" "$HERE/frames.png" "$W" "$H2" "$N" "$WK" "$FIT" "$AMP" "$EX" "$EY" "$TX" <<'PY'
import sys, math
from PIL import Image, ImageChops

cut_path, out, W, H2, N, WK, FIT, AMP, EX, EY, TX = (
    sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5]),
    int(sys.argv[6]), float(sys.argv[7]), float(sys.argv[8]), float(sys.argv[9]),
    float(sys.argv[10]), float(sys.argv[11]),
)


def find_coeffs(dst, src):
    """8 perspective coeffs mapping OUTPUT point dst[i] → INPUT point src[i] (Pillow's
    PERSPECTIVE convention), by solving the 8x8 linear system with Gaussian elimination
    (no numpy on this machine)."""
    A, b = [], []
    for (x, y), (X, Y) in zip(dst, src):
        A.append([x, y, 1, 0, 0, 0, -X * x, -X * y]); b.append(X)
        A.append([0, 0, 0, x, y, 1, -Y * x, -Y * y]); b.append(Y)
    n = 8
    for col in range(n):                       # forward elimination, partial pivot
        p = max(range(col, n), key=lambda r: abs(A[r][col]))
        A[col], A[p] = A[p], A[col]; b[col], b[p] = b[p], b[col]
        piv = A[col][col]
        for r in range(n):
            if r == col:
                continue
            f = A[r][col] / piv
            if f:
                for c in range(col, n):
                    A[r][c] -= f * A[col][c]
                b[r] -= f * b[col]
    return [b[i] / A[i][i] for i in range(n)]


# place the cut-out on a transparent working canvas, scaled to FIT, centered a touch high
cut = Image.open(cut_path).convert("RGBA")
sc = min(FIT * WK / cut.width, FIT * WK / cut.height)
cut = cut.resize((max(1, round(cut.width * sc)), max(1, round(cut.height * sc))), Image.LANCZOS)
canvas = Image.new("RGBA", (WK, WK), (0, 0, 0, 0))
canvas.paste(cut, ((WK - cut.width) // 2, int(WK * 0.46) - cut.height // 2), cut)

incorners = [(0, 0), (WK, 0), (WK, WK), (0, WK)]  # TL, TR, BR, BL of the working canvas
sheet = Image.new("RGBA", (W, H2 * N), (0, 0, 0, 0))
for n in range(N):
    theta = 2 * math.pi * n / N
    s = AMP * math.sin(theta)                  # signed yaw; +→right edge recedes
    li, ri = max(0.0, -s), max(0.0, s)         # which vertical edge foreshortens
    tx = -s * TX * WK                          # whole-subject swing, opposite the turn
    dst = [
        (li * EX * WK + tx,           li * EY * WK),           # TL
        (WK - ri * EX * WK + tx,      ri * EY * WK),           # TR
        (WK - ri * EX * WK + tx, WK - ri * EY * WK),           # BR
        (li * EX * WK + tx,      WK - li * EY * WK),           # BL
    ]
    coeffs = find_coeffs(dst, incorners)
    warp = canvas.transform((WK, WK), Image.PERSPECTIVE, coeffs, Image.BICUBIC, fillcolor=(0, 0, 0, 0))
    r, g, b, a = warp.split()                  # premultiply so the downscale has no fringe
    pm = Image.merge("RGBA", (ImageChops.multiply(r, a), ImageChops.multiply(g, a),
                              ImageChops.multiply(b, a), a))
    # BILINEAR (not LANCZOS): a premultiplied edge drops to 0 abruptly, and a ringing kernel
    # overshoots there into a bright cutout halo. Bilinear has no overshoot.
    small = pm.resize((W, H2), Image.BILINEAR)
    sheet.paste(small, (0, n * H2))            # no mask: store the premultiplied RGBA verbatim
import os
sheet.save(out, optimize=True)
print(f"bake.sh: wrote {out} ({W}x{H2 * N} RGBA, {os.path.getsize(out) // 1024} KB)", file=sys.stderr)
PY

echo "✓ frames.png ready. Next: go test ./...  (refresh golden with -update)"
