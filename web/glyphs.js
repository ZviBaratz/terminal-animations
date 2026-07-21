// glyphs.js — the sub-cell model, as pure data.
//
// This is the half of the painter that has no canvas in it: which sub-rectangles of
// a cell a glyph covers, and where those sub-rectangles fall in pixels. It is split
// out from harness.js for two reasons — it can be unit-tested without a browser
// (see glyphs.test.js), and the gallery draws the resolution-ladder samples with it
// without pulling in the viewer or a WASM module.
//
// scripts/ansi2png.py is the source of truth for every table here. The page claims
// to match the PNG/GIF export, and that claim is only worth anything if the two
// implementations agree glyph for glyph and pixel for pixel. Where this file and
// that one disagree, this file is wrong.

// Region k of n spans [k*total/n, (k+1)*total/n) — ansi2png.py's _imap, expressed as
// edges rather than a per-pixel lookup because the painter fills runs, not pixels.
//
// The regions tile the cell exactly: no gap, no overlap, and no off-by-one at any
// cell size. An odd size spreads the odd pixel instead of dumping it on one end
// (7px/2 -> 3+4, 14px/4 -> 3+4+3+4).
//
// This replaces an earlier truncate-offset/Math.ceil-extent rule that overshot by a
// pixel at odd cell widths — harmless for a glyph drawing one rectangle, but wrong
// for one drawing several, where ceil'd bands overlap and a lit dot paints over its
// neighbour's background.
export function bandEdges(total, n) {
  const e = new Array(n + 1);
  for (let k = 0; k <= n; k++) e[k] = ((k * total) / n) | 0;
  return e;
}

// Braille (U+2800..U+28FF) -> a row-major 2x4 mask, bit (row*2 + col), LSB top-left.
//
// The dot numbering is column-major for the historic 6-dot cell (1,2,3 down the left
// column; 4,5,6 down the right) and only then tacks 7/8 on as a bottom row. So the
// codepoint's bit order is NOT raster order and a naive 1 << (row*2 + col) is wrong:
//
//     dot1 dot4   bit 0x01 0x08   (row, col) (0,0) (0,1)
//     dot2 dot5       0x02 0x10              (1,0) (1,1)
//     dot3 dot6       0x04 0x20              (2,0) (2,1)
//     dot7 dot8       0x40 0x80              (3,0) (3,1)   <- appended later
//
// Mirrors BRAILLE_RC in ansi2png.py and brailleBit in examples/torus/torus.go, both
// of which carry the same warning.
const BRAILLE_RC = [[0, 0], [1, 0], [2, 0], [0, 1], [1, 1], [2, 1], [3, 0], [3, 1]];

export const BRAILLE_MASK = /* @__PURE__ */ (() => {
  const out = new Uint8Array(256);
  for (let bits = 0; bits < 256; bits++) {
    let m = 0;
    for (let d = 0; d < 8; d++) {
      if ((bits >> d) & 1) {
        const [r, c] = BRAILLE_RC[d];
        m |= 1 << (r * 2 + c);
      }
    }
    out[bits] = m;
  }
  return out;
})();

// Half / quadrant / full blocks (U+2580..U+259F) -> which of the four sub-quadrants
// are foreground. Bits: UL=1, UR=2, LL=4, LR=8 — bit (row*2 + col), the same
// convention BRAILLE_MASK uses at 2x4, so both share one draw path.
//
// A Uint8Array indexed by (codepoint - 0x2580) rather than an object literal: the
// keys are integers in a sparse range, which an object backs with a dictionary.
// 0 means "not modelled here" — the painter defers those, matching ansi2png.py,
// which does not model the eighth blocks either.
export const BLOCK_MASK = /* @__PURE__ */ (() => {
  const out = new Uint8Array(0x20);
  const q = [
    [0x2580, 3], [0x2584, 12], [0x2588, 15], [0x258c, 5], [0x2590, 10],
    [0x2596, 4], [0x2597, 8], [0x2598, 1], [0x2599, 13], [0x259a, 9],
    [0x259b, 7], [0x259c, 11], [0x259d, 2], [0x259e, 6], [0x259f, 14],
  ];
  for (const [cp, mask] of q) out[cp - 0x2580] = mask;
  return out;
})();
