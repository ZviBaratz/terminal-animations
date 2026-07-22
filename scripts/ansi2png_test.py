#!/usr/bin/env python3
"""Golden-ish functional test for ansi2png.py. Stdlib only; no PIL.

Runs the script as a subprocess on crafted ANSI input, decodes the PNG back
(zlib inflate of IDAT), and asserts sampled pixels are the intended colour.

    python3 scripts/ansi2png_test.py   # exits 0 on pass
"""
import os
import struct
import subprocess
import sys
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
SCRIPT = os.path.join(HERE, "ansi2png.py")


def decode_png(data):
    assert data[:8] == b"\x89PNG\r\n\x1a\n", "bad PNG signature"
    pos, width, height, idat = 8, None, None, b""
    while pos < len(data):
        (length,) = struct.unpack(">I", data[pos:pos + 4])
        typ = data[pos + 4:pos + 8]
        chunk = data[pos + 8:pos + 8 + length]
        if typ == b"IHDR":
            width, height, bit_depth, color_type = struct.unpack(">IIBB", chunk[:10])
            assert (bit_depth, color_type) == (8, 2), "expected 8-bit RGB"
        elif typ == b"IDAT":
            idat += chunk
        pos += 12 + length
    raw = zlib.decompress(idat)
    stride = width * 3
    rows = []
    for y in range(height):
        start = y * (stride + 1)
        assert raw[start] == 0, "expected filter 0"
        rows.append(raw[start + 1:start + 1 + stride])
    return width, height, rows


def pixel(rows, x, y):
    r = rows[y]
    return (r[x * 3], r[x * 3 + 1], r[x * 3 + 2])


def run_raw(stdin_bytes, env=None, args=None):
    """Run the script without asserting success — for exercising the error/exit paths."""
    e = dict(os.environ)
    e.update(env or {})
    return subprocess.run([sys.executable, SCRIPT] + (args or []), input=stdin_bytes,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE, env=e)


def run(stdin_bytes, env=None, args=None):
    p = run_raw(stdin_bytes, env, args)
    if p.returncode != 0:
        raise AssertionError("ansi2png exited %d: %s" % (p.returncode, p.stderr.decode()))
    return p.stdout


def main():
    cw, ch = 4, 4
    env = {"ANSI2PNG_CW": str(cw), "ANSI2PNG_CH": str(ch)}

    # One frame, two cells: a bg-painted red space, then a green '#'.
    frame = ("--- frame 0 ---\n"
             "\x1b[48;2;255;0;0m \x1b[0m\x1b[38;2;0;255;0m#\x1b[0m\n")
    w, h, rows = decode_png(run(frame.encode(), env))
    assert (w, h) == (2 * cw, ch), (w, h)
    assert pixel(rows, 0, 0) == (255, 0, 0), pixel(rows, 0, 0)          # cell 0 = red bg
    # cell 1 = green fg, blended at '#'s ink coverage rather than a flat fg block
    # (see the ramp case below): the green channel is well lit but short of full.
    g = pixel(rows, cw, 0)
    assert g[0] == 0 and g[2] == 0 and 100 < g[1] < 255, g

    # 256-colour and basic-colour paths resolve to real RGB (not default).
    frame2 = "--- frame 0 ---\n\x1b[48;5;196m \x1b[0m\n"                # xterm 196 ~ bright red
    _, _, rows2 = decode_png(run(frame2.encode(), env))
    assert pixel(rows2, 0, 0) == (255, 0, 0), pixel(rows2, 0, 0)

    # Out-of-range colour values (a buggy animation) clamp instead of crashing.
    bad = "--- frame 0 ---\n\x1b[48;2;300;0;0m \x1b[0m\n"
    _, _, rowsb = decode_png(run(bad.encode(), env))
    assert pixel(rowsb, 0, 0) == (255, 0, 0), pixel(rowsb, 0, 0)        # 300 clamped to 255

    # Two frames stack vertically with a gap row between them.
    two = ("--- frame 0 ---\n\x1b[48;2;10;20;30m \x1b[0m\n"
           "--- frame 1 ---\n\x1b[48;2;40;50;60m \x1b[0m\n")
    w3, h3, rows3 = decode_png(run(two.encode(), env))
    assert h3 == ch + 2 + ch, h3                                       # frame + GAP_H(2) + frame
    assert pixel(rows3, 0, 0) == (10, 20, 30), pixel(rows3, 0, 0)
    assert pixel(rows3, 0, ch + 2) == (40, 50, 60), pixel(rows3, 0, ch + 2)

    # Half-block ▀ splits into fg (top half) and bg (bottom half) — real raster,
    # not a flat fg block. (Multibyte glyph also exercises the UTF-8 stdin path.)
    hb = "--- frame 0 ---\n\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀\x1b[0m\n"
    _, _, rowsh = decode_png(run(hb.encode(), env))
    assert pixel(rowsh, 0, 0) == (255, 0, 0), pixel(rowsh, 0, 0)            # top half = fg red
    assert pixel(rowsh, 0, ch // 2) == (0, 0, 255), pixel(rowsh, 0, ch // 2)  # bottom half = bg blue

    # Left half ▌ splits horizontally: fg (left column) vs bg (right column).
    lh = "--- frame 0 ---\n\x1b[38;2;0;255;0m\x1b[48;2;0;0;0m▌\x1b[0m\n"
    _, _, rowsl = decode_png(run(lh.encode(), env))
    assert pixel(rowsl, 0, 0) == (0, 255, 0), pixel(rowsl, 0, 0)            # left col = fg green
    assert pixel(rowsl, cw // 2, 0) == (0, 0, 0), pixel(rowsl, cw // 2, 0)  # right col = bg black

    # Braille resolves into its 2x4 dot grid — a lit dot is fg, an unlit dot is bg
    # (it used to collapse the whole cell to a solid fg block). U+2895 is DOTS-1358:
    #
    #     # .      (0,0) lit   (0,1) off
    #     . #      (1,0) off   (1,1) lit    <- dot5, the column-major right half
    #     # .      (2,0) lit   (2,1) off
    #     . #      (3,0) off   (3,1) lit    <- dot8, the irregular appended bottom row
    #
    # One lit and one unlit dot per row, alternating columns: a transposed, flipped or
    # naively raster-ordered bit mapping all fail here.
    FG, BG = (255, 0, 0), (0, 0, 255)
    sgr = "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m"
    br = "--- frame 0 ---\n" + sgr + "⢕\x1b[0m\n"
    _, _, rowsbr = decode_png(run(br.encode(), env))
    lit = {(0, 0), (1, 1), (2, 0), (3, 1)}
    for r_ in range(4):
        for c_ in range(2):
            x, y = c_ * cw // 2, r_ * ch // 4      # same k*total//n edge as the renderer
            want = FG if (r_, c_) in lit else BG
            assert pixel(rowsbr, x, y) == want, ((r_, c_), pixel(rowsbr, x, y))

    # The braille blank U+2800 is real negative space, not a block: every pixel is bg.
    # (It is `isprintable()`, so it used to fall through to a solid *foreground* cell.)
    blank = "--- frame 0 ---\n" + sgr + "⠀\x1b[0m\n"
    _, _, rowsbl = decode_png(run(blank.encode(), env))
    assert all(pixel(rowsbl, x, y) == BG
               for x in range(cw) for y in range(ch)), "U+2800 is not blank"

    # …and U+28FF (all eight dots) is a full fg block.
    solid = "--- frame 0 ---\n" + sgr + "⣿\x1b[0m\n"
    _, _, rowsbf = decode_png(run(solid.encode(), env))
    assert all(pixel(rowsbf, x, y) == FG
               for x in range(cw) for y in range(ch)), "U+28FF is not solid"

    # The dot grid tiles an ODD cell exactly — no gap, no overlap, on BOTH axes. At the
    # 7x14 default the splits are uneven (cols 3+4, rows 3+4+3+4), which is where an
    # off-by-one hides. U+2847 is dots 1,2,3,7 = the whole left column.
    lcol = "--- frame 0 ---\n" + sgr + "⡇\x1b[0m\n"
    _, _, rowslc = decode_png(run(lcol.encode(), None, ["--cw", "7", "--ch", "14"]))
    for y in range(14):
        assert ([pixel(rowslc, x, y) for x in range(7)]
                == [FG] * 3 + [BG] * 4), (y, [pixel(rowslc, x, y) for x in range(7)])

    # …and the row split, which the left-column glyph above cannot see. Each of these
    # lights exactly one dot row (both columns), so the four fg spans must partition the
    # 14px cell with no gap and no overlap: 0-2, 3-6, 7-9, 10-13. A floor-division
    # split (14//4 = 3px rows + a fat last row) gets these wrong and is caught here.
    spans = []
    for glyph in ("⠉", "⠒", "⠤", "⣀"):          # dot rows 0, 1, 2, 3
        one = "--- frame 0 ---\n" + sgr + glyph + "\x1b[0m\n"
        _, _, rws = decode_png(run(one.encode(), None, ["--cw", "7", "--ch", "14"]))
        for y in range(14):                       # each row is uniform across the cell
            line = [pixel(rws, x, y) for x in range(7)]
            assert len(set(line)) == 1, (glyph, y, line)
        spans.append([y for y in range(14) if pixel(rws, 0, y) == FG])
    assert spans == [[0, 1, 2], [3, 4, 5, 6], [7, 8, 9], [10, 11, 12, 13]], spans

    # Regression guard for the now-shared sub-cell path: the 2x2 quadrant split must be
    # unchanged at an odd cell width — ▌ is still 3 fg columns then 4 bg, matching the
    # old hard-coded `sx < CW // 2`. Every other case here runs at an even cw=4, which
    # would not catch a one-pixel shift at the 7px default.
    odd = "--- frame 0 ---\n\x1b[38;2;0;255;0m\x1b[48;2;0;0;0m▌\x1b[0m\n"
    _, _, rowsod = decode_png(run(odd.encode(), None, ["--cw", "7", "--ch", "14"]))
    assert ([pixel(rowsod, x, 0) for x in range(7)]
            == [(0, 255, 0)] * 3 + [(0, 0, 0)] * 4), [pixel(rowsod, x, 0) for x in range(7)]

    # The shade blocks ░▒▓ blend fg over bg at a fixed coverage — the tier INK was
    # modelled on. White on black makes the coverage readable straight off the channel
    # value, so this pins the actual constants (0.25/0.5/0.75), not just their order:
    # a shade rendered as a solid fg block reads 255, and as bg reads 0.
    shades = "--- frame 0 ---\n\x1b[48;2;0;0;0m\x1b[38;2;255;255;255m░▒▓\x1b[0m\n"
    _, _, rs = decode_png(run(shades.encode(), env))
    for i, want in enumerate((64, 128, 191)):              # round(255*0.25/0.5/0.75)
        px = pixel(rs, i * cw, 0)
        assert px == (want,) * 3, (i, px, want)
        # …and the blend fills the whole cell uniformly — it is a flat wash, not a
        # sub-cell split like the half-block and braille tiers above.
        assert all(pixel(rs, i * cw + x, y) == px
                   for x in range(cw) for y in range(ch)), (i, "shade cell not uniform")

    # A typographic ramp rasterizes as ink *coverage*, not flat fg blocks. '·' and '@'
    # carry the same fg colour, so a flat-block rasterizer draws them identically —
    # which is exactly the blindness that made this gate useless for judging a
    # glyph-density ramp (fresco's lumRange splits brightness into that ramp).
    ramp = "--- frame 0 ---\n\x1b[48;2;0;0;0m\x1b[38;2;255;255;255m·@\x1b[0m\n"
    _, _, rr = decode_png(run(ramp.encode(), env))
    light, heavy = pixel(rr, 0, 0), pixel(rr, cw, 0)
    assert light != heavy, (light, heavy)                  # the whole point
    assert sum(light) < sum(heavy), (light, heavy)         # lighter glyph = less ink
    assert sum(light) < sum(heavy) // 3, (light, heavy)    # '·' near bg, '@' near fg

    # Ink coverage is monotonic along a ramp, so a gradient reads as a gradient.
    mono = "--- frame 0 ---\n\x1b[48;2;0;0;0m\x1b[38;2;255;255;255m .:+*oO0@\x1b[0m\n"
    _, _, rm = decode_png(run(mono.encode(), env))
    inks = [sum(pixel(rm, i * cw, 0)) for i in range(9)]
    assert inks == sorted(inks), inks
    assert inks[0] == 0, inks                              # leading space stays bg

    # A printable glyph with no ink entry falls back to solid fg, so text and box
    # drawing stay visible rather than vanishing into an invented coverage.
    unk = "--- frame 0 ---\n\x1b[48;2;0;0;0m\x1b[38;2;0;255;0m§\x1b[0m\n"
    _, _, ru = decode_png(run(unk.encode(), env))
    assert pixel(ru, 0, 0) == (0, 255, 0), pixel(ru, 0, 0)

    # --cw/--ch flags set the cell size, and a flag overrides the env var — the
    # pipe-safe path (env before the `|` reaches the producer, not ansi2png).
    wf, hf, _ = decode_png(run(frame.encode(), None, ["--cw", "5", "--ch", "9"]))
    assert (wf, hf) == (2 * 5, 9), (wf, hf)
    wp, hp, _ = decode_png(run(frame.encode(), {"ANSI2PNG_CW": "99"}, ["--cw", "4", "--ch", "8"]))
    assert (wp, hp) == (2 * 4, 8), (wp, hp)

    # A flag overrides even a *malformed* env var — flag > env holds without the env
    # value ever being parsed, so a garbage ANSI2PNG_CW can't crash the run.
    wm, hm, _ = decode_png(run(frame.encode(), {"ANSI2PNG_CW": "junk"}, ["--cw", "6", "--ch", "8"]))
    assert (wm, hm) == (2 * 6, 8), (wm, hm)

    # A malformed env var with no flag to override it fails cleanly (exit 2), not with a traceback.
    r = run_raw(frame.encode(), {"ANSI2PNG_CW": "junk"})
    assert r.returncode == 2, r.returncode
    assert b"ANSI2PNG_CW" in r.stderr, r.stderr

    # --stats reports to stderr while stdout still carries a valid PNG, counts *frames*
    # (not text rows), and buckets the lit pixels by hue. Two frames: one all-red cell
    # and one all-blue, so the expected report is exact — half the lit pixels red, half
    # blue, and nothing dark.
    two = ("--- frame 0 ---\n"
           "\x1b[48;2;255;0;0m \x1b[0m\n"
           "--- frame 1 ---\n"
           "\x1b[48;2;0;0;255m \x1b[0m\n")
    p = run_raw(two.encode(), env, ["--stats"])
    assert p.returncode == 0, p.stderr
    ws, hs, _ = decode_png(p.stdout)          # stdout is still the image
    assert (ws, hs) == (cw, 2 * ch + 2), (ws, hs)
    report = p.stderr.decode()
    assert "2 frame(s)" in report, report      # frames, not the 2 text rows
    assert "% of lit" in report, report        # not a literal %%
    assert "red" in report and "blue" in report, report
    for line in report.splitlines():
        if line.strip().startswith("0- 29"):   # the red bucket
            assert "50.00%" in line, line

    # The two summary percentages are deliberately different cuts — "dark" is the
    # bottom histogram bin, "lit" is the higher hue-readability threshold — so each
    # must report the cut it actually counted. A frame painted at luminance 17.2
    # (0,24,0) sits in the gap between them: above the bottom bin, below the lit
    # threshold, so BOTH percentages are 0 and the header has to say so honestly.
    # Printing one threshold while counting by another is exactly the bug this pins.
    gap = "--- frame 0 ---\n\x1b[48;2;0;24;0m \x1b[0m\n"
    g = run_raw(gap.encode(), env, ["--stats"])
    assert g.returncode == 0, g.stderr
    grep = g.stderr.decode()
    head = [l for l in grep.splitlines() if "luminance  mean" in l][0]
    assert "dark (<16)" in head, head        # the bin edge, not LIT_MIN
    assert "lit (>18)" in head, head         # the lit cut names its own threshold
    assert "dark (<16) 0.0%" in head, head   # nothing in the bottom bin...
    assert "lit (>18) 0.0%" in head, head    # ...and nothing above the lit cut
    bins = [l.strip() for l in grep.splitlines() if l.startswith("    ") and "%" in l]
    assert any(b.startswith("16- 31 100.00%") for b in bins), bins
    assert not any(b.startswith("0- 15") for b in bins), bins

    print("ansi2png_test: OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
