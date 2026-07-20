#!/usr/bin/env python3
"""ansi2png — the headless colour gate.

Rasterize an ANSI/truecolor terminal frame into a PNG so you can *see* the colour
without a live terminal (a sandbox, CI, an agent). It is the headless stand-in for
the vhs GIF gate: you still judge the colour by eye — from the image, never from
the formula.

    go run ./cmd/preview frames 1 | scripts/ansi2png.py > /tmp/f.png
    go run ./cmd/preview frames 5 | scripts/ansi2png.py > /tmp/strip.png   # filmstrip

Reads the `frames` dump on stdin (the `--- frame N ---` headers are recognised and
each frame is stacked vertically with a gap). A painted-background space shows as its
bg colour and a glyph as its fg colour; the half-block, quadrant and full-block glyphs
(▀▄▌▐ █ ▖▗▘▙▚▛▜▝▞▟) are split into their 2x2 sub-cell fg/bg regions and braille
(U+2800–28FF) into its 2x4 dot grid — a lit dot takes the foreground, an unlit dot the
background — so real half-block raster and braille line art both read correctly. Dots
fill their sub-rectangle rather than being drawn inset, so a cell's eight dots tile it
with no gaps; that is the right approximation at these sizes (use --ch >= 4 so all four
dot rows get a pixel). Sextant (U+1FB00–1FB3B) and octant (U+1CD00–1CDE5) carry finer
detail than this coarse rasterizer resolves, and they fail differently: a sextant cell
collapses to its foreground, while an octant cell is dropped *entirely*. Octants need
Unicode 16 (Sept 2024), so on a Python whose `unicodedata` is older (3.10 ships UCD 13)
`isprintable()` is False for U+1CD00–1CDE5 and the parse loop contributes no cell at all —
every row containing one shears left (a 5-cell row rasterizes to 4). Judge those two tiers
on a real terminal or the GIF gate.

Truecolor (`38;2;r;g;b` / `48;2;…`), 256-colour (`38;5;n`), and the 16 basic
colours are understood; other SGR codes are ignored. Stdlib only (zlib for PNG).

Cell size: --cw / --ch flags, else the ANSI2PNG_CW / ANSI2PNG_CH env vars, else 7x14
(cells are drawn taller than wide to match a terminal). Prefer the flags — an env var
set before the producer in a pipe (`ANSI2PNG_CW=8 go run ... | ansi2png.py`) applies to
the producer, not to ansi2png, so it is silently ignored; a flag can't be misdirected:

    go run ./cmd/preview frames 5 | ansi2png.py --cw 8 --ch 16 > /tmp/anim.png
"""
import os
import re
import struct
import sys
import zlib

CW_DEFAULT = 7
CH_DEFAULT = 14
CW = CW_DEFAULT  # cell px width; resolved from flag > env > default in _parse_args
CH = CH_DEFAULT  # cell px height; resolved from flag > env > default in _parse_args
GAP_H = 2
GAP_RGB = (48, 48, 48)
DEF_FG = (200, 200, 200)
DEF_BG = (0, 0, 0)

# The 16 basic ANSI colours (xterm-ish), indexed by 30-37 / 90-97 offset.
BASIC = [
    (0, 0, 0), (205, 0, 0), (0, 205, 0), (205, 205, 0),
    (0, 0, 238), (205, 0, 205), (0, 205, 205), (229, 229, 229),
    (127, 127, 127), (255, 0, 0), (0, 255, 0), (255, 255, 0),
    (92, 92, 255), (255, 0, 255), (0, 255, 255), (255, 255, 255),
]

# Half/quadrant/full block glyphs -> which of the four sub-quadrants are foreground.
# Bits: UL=1, UR=2, LL=4, LR=8; the rest of the cell takes the background colour.
# That is bit (row*2 + col), row-major with the LSB at top-left — the same convention
# BRAILLE uses at 2x4, so both share one sub-cell draw path.
QUAD = {
    "▀": 3, "▄": 12, "▌": 5, "▐": 10, "█": 15,  # ▀▄▌▐ █
    "▖": 4, "▗": 8, "▘": 1, "▙": 13, "▚": 9,    # ▖▗▘▙▚
    "▛": 7, "▜": 11, "▝": 2, "▞": 6, "▟": 14,   # ▛▜▝▞▟
}
# Shade glyphs -> fraction of foreground blended over background.
SHADE = {"░": 0.25, "▒": 0.5, "▓": 0.75}  # ░▒▓

# Braille (U+2800..U+28FF): a 2x4 dot grid. The dot numbering is column-major for the
# historic 6-dot cell (1,2,3 down the left column; 4,5,6 down the right) and only then
# tacks 7/8 on as a bottom row — so the codepoint's bit order is NOT raster order and
# a naive 1 << (row*2 + col) is wrong. This table renumbers it:
#
#     dot1 dot4   bit 0x01 0x08   (row, col) (0,0) (0,1)
#     dot2 dot5       0x02 0x10              (1,0) (1,1)
#     dot3 dot6       0x04 0x20              (2,0) (2,1)
#     dot7 dot8       0x40 0x80              (3,0) (3,1)   <- appended later
BRAILLE_RC = ((0, 0), (1, 0), (2, 0), (0, 1), (1, 1), (2, 1), (3, 0), (3, 1))


def _braille_masks():
    """U+2800..U+28FF -> a row-major 2x4 mask in QUAD's convention (bit row*2+col)."""
    out = {}
    for bits in range(256):
        m = 0
        for d in range(8):
            if bits >> d & 1:
                r, c = BRAILLE_RC[d]
                m |= 1 << (r * 2 + c)
        out[chr(0x2800 + bits)] = m
    return out


BRAILLE = _braille_masks()


def _mix(bg, fg, a):
    return tuple(int(round(bg[k] + (fg[k] - bg[k]) * a)) for k in range(3))


_IMAP = {}


def _imap(total, n):
    """px offset -> sub-cell region index, for `n` regions across `total` px.

    Region k spans [k*total//n, (k+1)*total//n), so the regions tile the cell exactly —
    no gap, no overlap, no off-by-one at any cell size, and an odd size spreads the odd
    pixel rather than dumping it on one end (7px/2 -> 3+4, 14px/4 -> 3+4+3+4). This
    reproduces the old hard-coded 2x2 split (`sx < CW // 2`) byte for byte, so the
    half-block/quadrant output is unchanged. At --ch < 4 some dot rows get zero pixels
    and are invisible; the default 14 and record-headless.sh's 16 are ample."""
    m = _IMAP.get((total, n))
    if m is None:
        m = _IMAP[(total, n)] = [((p + 1) * n - 1) // total for p in range(total)]
    return m


def _mask_cell(fgc, bgc, bits, cols, rows):
    """A rows x cols sub-cell mask spec, flattened to ("solid", …) when the mask is
    empty (U+2800, the braille blank) or full (█, U+28FF) — same pixels, but it takes
    the whole-cell memcpy path. That matters for braille line art: a wireframe is
    mostly blank cells."""
    if bits == 0:
        return ("solid", bgc)
    if bits == (1 << (cols * rows)) - 1:
        return ("solid", fgc)
    return ("mask", fgc, bgc, bits, cols, rows)


SGR = re.compile(r"\x1b\[([0-9;]*)m")
ANSI_ANY = re.compile(r"\x1b\[[0-9;?]*[ -/]*[@-~]")
FRAME_HDR = re.compile(r"^-{2,}\s*frame\b.*-{2,}\s*$")


def clamp8(v):
    return 0 if v < 0 else 255 if v > 255 else v


def xterm256(n):
    n = clamp8(n)
    if n < 16:
        return BASIC[n]
    if n < 232:
        n -= 16
        r, g, b = n // 36, (n // 6) % 6, n % 6
        steps = [0, 95, 135, 175, 215, 255]
        return (steps[r], steps[g], steps[b])
    v = 8 + (n - 232) * 10
    return (v, v, v)


def apply_sgr(params, fg, bg):
    """Fold one SGR parameter list into (fg, bg). None means 'default'."""
    codes = [int(p) if p else 0 for p in params.split(";")] if params else [0]
    i = 0
    while i < len(codes):
        c = codes[i]
        if c == 0:
            fg, bg = None, None
        elif c in (38, 48) and i + 1 < len(codes) and codes[i + 1] == 2 and i + 4 < len(codes):
            col = (clamp8(codes[i + 2]), clamp8(codes[i + 3]), clamp8(codes[i + 4]))
            fg, bg = (col, bg) if c == 38 else (fg, col)
            i += 4
        elif c in (38, 48) and i + 1 < len(codes) and codes[i + 1] == 5 and i + 2 < len(codes):
            col = xterm256(codes[i + 2])
            fg, bg = (col, bg) if c == 38 else (fg, col)
            i += 2
        elif 30 <= c <= 37:
            fg = BASIC[c - 30]
        elif 90 <= c <= 97:
            fg = BASIC[c - 90 + 8]
        elif 40 <= c <= 47:
            bg = BASIC[c - 40]
        elif 100 <= c <= 107:
            bg = BASIC[c - 100 + 8]
        elif c == 39:
            fg = None
        elif c == 49:
            bg = None
        i += 1
    return fg, bg


def row_cells(line):
    """A visible line -> list of cell specs, one per terminal cell.

    A spec is ("solid", rgb) or ("mask", fg, bg, bits, cols, rows) — the latter for the
    glyphs that subdivide the cell (half/quadrant/full block at 2x2, braille at 2x4),
    whose sub-cell regions are painted from the (fg, bg) pair the way the terminal
    draws them."""
    cells, fg, bg, i = [], None, None, 0
    while i < len(line):
        ch = line[i]
        if ch == "\x1b":
            m = SGR.match(line, i)
            if m:
                fg, bg = apply_sgr(m.group(1), fg, bg)
                i = m.end()
                continue
            m = ANSI_ANY.match(line, i)  # non-colour escape: skip it
            if m:
                i = m.end()
                continue
            i += 1
            continue
        fgc = fg if fg is not None else DEF_FG
        bgc = bg if bg is not None else DEF_BG
        if ch == " ":
            cells.append(("solid", bgc))
        elif ch in QUAD:
            cells.append(_mask_cell(fgc, bgc, QUAD[ch], 2, 2))
        elif ch in BRAILLE:
            cells.append(_mask_cell(fgc, bgc, BRAILLE[ch], 2, 4))
        elif ch in SHADE:
            cells.append(("solid", _mix(bgc, fgc, SHADE[ch])))
        elif ch.isprintable() and ch != "\t":
            cells.append(("solid", fgc))
        # other control chars contribute no cell
        i += 1
    return cells


def build_rows(stdin_lines):
    """-> list of items: a list-of-cells for a text row, or None for a frame gap."""
    rows, seen_frame = [], False
    for raw in stdin_lines:
        line = raw.rstrip("\n")
        if FRAME_HDR.match(line):
            if seen_frame:
                rows.append(None)  # gap between frames
            seen_frame = True
            continue
        if line == "":
            continue  # blank separators / trailing newline
        rows.append(row_cells(line))
    return rows


def render(rows):
    text_rows = [r for r in rows if r is not None]
    wcells = max((len(r) for r in text_rows), default=1)
    width = max(wcells * CW, 1)
    height = sum(CH if r is not None else GAP_H for r in rows) or 1
    buf = bytearray(width * height * 3)

    def hline(y0, n, rgb):
        base = y0 * width * 3
        one = bytes(rgb) * width
        for k in range(n):
            buf[base + k * width * 3: base + (k + 1) * width * 3] = one

    def fill_cells(y0, cells):
        for sy in range(CH):
            row = bytearray(width * 3)
            for cx in range(wcells):
                spec = cells[cx] if cx < len(cells) else ("solid", DEF_BG)
                if spec[0] == "solid":
                    row[cx * CW * 3:(cx + 1) * CW * 3] = bytes(spec[1]) * CW
                else:
                    # A rows x cols sub-cell mask: the dot row is fixed for this scan
                    # line, so resolve it once and only walk the columns per pixel.
                    _, fgc, bgc, bits, cols, nrows = spec
                    base = _imap(CH, nrows)[sy] * cols
                    colof = _imap(CW, cols)
                    fgb, bgb = bytes(fgc), bytes(bgc)
                    for sx in range(CW):
                        off = (cx * CW + sx) * 3
                        row[off:off + 3] = fgb if bits >> (base + colof[sx]) & 1 else bgb
            base = (y0 + sy) * width * 3
            buf[base:base + width * 3] = row

    y = 0
    for r in rows:
        if r is None:
            hline(y, GAP_H, GAP_RGB)
            y += GAP_H
        else:
            fill_cells(y, r)
            y += CH
    return width, height, bytes(buf)


def png(width, height, rgb):
    def chunk(typ, data):
        return (struct.pack(">I", len(data)) + typ + data
                + struct.pack(">I", zlib.crc32(typ + data) & 0xffffffff))

    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)  # RGB, 8-bit
    raw = bytearray()
    stride = width * 3
    for yy in range(height):
        raw.append(0)  # filter: none
        raw += rgb[yy * stride:(yy + 1) * stride]
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", ihdr)
            + chunk(b"IDAT", zlib.compress(bytes(raw), 9)) + chunk(b"IEND", b""))


def _cell_px(raw, source):
    """Validate one cell-size value (from a flag or env var), or exit(2) with a clear
    message naming `source` (e.g. `--cw` or `ANSI2PNG_CW`)."""
    try:
        val = int(raw)
    except ValueError:
        sys.stderr.write("ansi2png: %s needs an integer, got %r\n" % (source, raw))
        sys.exit(2)
    if val < 1:
        sys.stderr.write("ansi2png: %s must be >= 1\n" % source)
        sys.exit(2)
    return val


def _resolve(flag, flags, env, default):
    """Cell size for one axis: flag > env > default, every source validated the same way
    so a garbage env var can't crash the run (and an explicit flag always overrides it)."""
    if flag in flags:
        return _cell_px(flags[flag], flag)
    if os.environ.get(env) is not None:
        return _cell_px(os.environ[env], env)
    return default


def _parse_args(argv):
    """Resolve CW/CH with precedence flag > env > default. A tiny manual parser so the
    bare `... | ansi2png.py` path stays argument-free."""
    global CW, CH
    if "-h" in argv or "--help" in argv:
        sys.stdout.write(__doc__)
        sys.exit(0)
    flags = {}
    i = 0
    while i < len(argv):
        a = argv[i]
        if a in ("--cw", "--ch"):
            i += 1
            if i >= len(argv):
                sys.stderr.write("ansi2png: %s needs an integer\n" % a)
                sys.exit(2)
            key, raw = a, argv[i]
        elif a.startswith("--cw=") or a.startswith("--ch="):
            key, raw = a.split("=", 1)
        else:
            sys.stderr.write("ansi2png: unknown argument %r\n" % a)
            sys.exit(2)
        flags[key] = raw
        i += 1
    CW = _resolve("--cw", flags, "ANSI2PNG_CW", CW_DEFAULT)
    CH = _resolve("--ch", flags, "ANSI2PNG_CH", CH_DEFAULT)


def main():
    _parse_args(sys.argv[1:])
    # Read raw bytes and decode UTF-8 ourselves — the block glyphs are multibyte,
    # so a C/POSIX-locale stdin (a sandbox, CI) must not gate on the ascii codec.
    if hasattr(sys.stdin, "buffer"):
        text = sys.stdin.buffer.read().decode("utf-8", "replace")
    else:  # pragma: no cover - a stdin without a binary buffer is unusual
        text = sys.stdin.read()
    rows = build_rows(text.splitlines())
    if not any(r for r in rows):
        sys.stderr.write("ansi2png: no frame content on stdin\n")
        return 1
    w, h, rgb = render(rows)
    out = sys.stdout.buffer if hasattr(sys.stdout, "buffer") else sys.stdout
    out.write(png(w, h, rgb))
    return 0


if __name__ == "__main__":
    sys.exit(main())
