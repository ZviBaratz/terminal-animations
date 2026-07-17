#!/usr/bin/env python3
"""ansi2png — the headless colour gate.

Rasterize an ANSI/truecolor terminal frame into a PNG so you can *see* the colour
without a live terminal (a sandbox, CI, an agent). It is the headless stand-in for
the vhs GIF gate: you still judge the colour by eye — from the image, never from
the formula.

    go run ./cmd/preview frames 1 | scripts/ansi2png.py > /tmp/f.png
    go run ./cmd/preview frames 5 | scripts/ansi2png.py > /tmp/strip.png   # filmstrip

Reads the `frames` dump on stdin (the `--- frame N ---` headers are recognised and
each frame is stacked vertically with a gap). Each terminal cell becomes a solid
block: a painted background shows as its bg colour, a glyph shows as its fg colour —
enough to judge whether the hue varies the way the design intends.

Truecolor (`38;2;r;g;b` / `48;2;…`), 256-colour (`38;5;n`), and the 16 basic
colours are understood; other SGR codes are ignored. Stdlib only (zlib for PNG).

Env knobs: ANSI2PNG_CW (cell px width, default 7), ANSI2PNG_CH (cell px height,
default 14). Cells are drawn taller than wide to match a terminal.
"""
import os
import re
import struct
import sys
import zlib

CW = int(os.environ.get("ANSI2PNG_CW", "7"))
CH = int(os.environ.get("ANSI2PNG_CH", "14"))
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
    """A visible line -> list of (r,g,b), one per terminal cell."""
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
        if ch == " ":
            cells.append(bg if bg is not None else DEF_BG)
        elif ch.isprintable() and ch != "\t":
            cells.append(fg if fg is not None else DEF_FG)
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
        px = bytearray(width * 3)
        for cx in range(wcells):
            rgb = cells[cx] if cx < len(cells) else DEF_BG
            px[cx * CW * 3:(cx + 1) * CW * 3] = bytes(rgb) * CW
        base = y0 * width * 3
        for k in range(CH):
            buf[base + k * width * 3: base + (k + 1) * width * 3] = px

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


def main():
    rows = build_rows(sys.stdin.readlines())
    if not any(r for r in rows):
        sys.stderr.write("ansi2png: no frame content on stdin\n")
        return 1
    w, h, rgb = render(rows)
    out = sys.stdout.buffer if hasattr(sys.stdout, "buffer") else sys.stdout
    out.write(png(w, h, rgb))
    return 0


if __name__ == "__main__":
    sys.exit(main())
