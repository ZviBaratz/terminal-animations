// glyphs.test.js — run with: node --test web/
//
// These pin the two things the painter cannot get wrong without the page silently
// diverging from scripts/ansi2png.py: the braille dot renumbering, and the sub-cell
// tiling rule. Both are pure integer functions, so they are testable without a
// canvas — which is the whole reason glyphs.js exists as a separate module.
//
// Every expected value here was produced by executing scripts/ansi2png.py itself,
// not derived by hand. That file is the source of truth; if these disagree with it,
// these are wrong.

import test from 'node:test';
import assert from 'node:assert/strict';

import { bandEdges, BRAILLE_MASK, BLOCK_MASK } from './glyphs.js';

// --- the sub-cell tiling rule -------------------------------------------------

test('bandEdges splits 14px into 4 rows as 3+4+3+4', () => {
  // ansi2png.py's _imap docstring states this case explicitly. A floor-division
  // split (14//4 = 3, with the remainder dumped on the last row) yields
  // [0,3,6,9,14] and fails here — that is the mistake this test exists to catch.
  assert.deepEqual(bandEdges(14, 4), [0, 3, 7, 10, 14]);
});

test('bandEdges splits an odd 7px cell into 3+4, matching ansi2png at --cw 7', () => {
  // The live divergence: harness.js's Math.ceil rule gives the left half 4px here,
  // while ansi2png_test.py asserts 3. Odd cell widths are the discriminating case;
  // at cw=8 a correct and an incorrect implementation look identical.
  assert.deepEqual(bandEdges(7, 2), [0, 3, 7]);
});

test('bandEdges tiles exactly — no gap, no overlap, at any cell size', () => {
  for (let total = 2; total <= 40; total++) {
    for (const n of [2, 4]) {
      const e = bandEdges(total, n);
      assert.equal(e.length, n + 1, `${total}/${n}: wrong edge count`);
      assert.equal(e[0], 0, `${total}/${n}: must start at 0`);
      assert.equal(e[n], total, `${total}/${n}: must end at total`);
      for (let k = 0; k < n; k++) {
        assert.ok(e[k] <= e[k + 1], `${total}/${n}: band ${k} runs backwards`);
      }
    }
  }
});

test('every dot row gets at least one pixel at the minimum supported cell', () => {
  // index.html clamps cell >= 2 and harness.js always passes ch = cell*2, so
  // ch >= 4 is structurally guaranteed and no braille dot row can vanish.
  const e = bandEdges(4, 4);
  for (let k = 0; k < 4; k++) assert.ok(e[k + 1] > e[k], `dot row ${k} is empty`);
});

// --- braille dot renumbering --------------------------------------------------

test('BRAILLE_MASK renumbers the historic column-major dot order to raster order', () => {
  // U+2895 = dots 1,3,5,8. Chosen because a transposed, flipped, or naive
  // 1 << (row*2 + col) mapping all produce a different answer.
  // Expected value produced by executing ansi2png.py: BRAILLE[chr(0x2895)] == 153.
  assert.equal(BRAILLE_MASK[0x95], 153);
});

test('BRAILLE_MASK places all eight dots individually', () => {
  // The complete pin. Each mask is a bitwise OR of independent dots, so fixing all
  // eight single-dot codepoints determines the entire 256-entry table — no swap of
  // any pair can survive this.
  //
  // The 0x95 test above alone is not enough: it exercises bits 0,2,4,7, leaving a
  // swap among bits 1,5,6 undetected. Verified by mutation.
  //
  // Expected values produced by executing ansi2png.py.
  const dots = [[0x01, 1], [0x02, 4], [0x04, 16], [0x08, 2],
                [0x10, 8], [0x20, 32], [0x40, 64], [0x80, 128]];
  for (const [cp, mask] of dots) {
    assert.equal(BRAILLE_MASK[cp], mask, `dot at codepoint bit 0x${cp.toString(16)}`);
  }
  // Spelled out: dot 4 (0x08) is top-RIGHT -> raster bit 1 -> 2. A naive
  // 1 << (row*2 + col) reading bit index 3 as (row 1, col 1) would give 8.
  assert.equal(BRAILLE_MASK[0x08], 2);
});

test('BRAILLE_MASK maps the blank and full cells to empty and full masks', () => {
  assert.equal(BRAILLE_MASK[0x00], 0);   // U+2800, the braille blank
  assert.equal(BRAILLE_MASK[0xff], 255); // U+28FF, all eight dots
});

test('BRAILLE_MASK covers all 256 codepoints', () => {
  assert.equal(BRAILLE_MASK.length, 256);
});

// --- block / quadrant glyphs --------------------------------------------------

// Transcribed from ansi2png.py's QUAD by executing it, not by reading it.
// Bits: UL=1, UR=2, LL=4, LR=8 — bit (row*2 + col), LSB at top-left.
const QUAD = [
  [0x2580, 3, '▀'], [0x2584, 12, '▄'], [0x2588, 15, '█'], [0x258c, 5, '▌'],
  [0x2590, 10, '▐'], [0x2596, 4, '▖'], [0x2597, 8, '▗'], [0x2598, 1, '▘'],
  [0x2599, 13, '▙'], [0x259a, 9, '▚'], [0x259b, 7, '▛'], [0x259c, 11, '▜'],
  [0x259d, 2, '▝'], [0x259e, 6, '▞'], [0x259f, 14, '▟'],
];

test('BLOCK_MASK matches ansi2png.py for all 15 quadrant glyphs', () => {
  for (const [cp, mask, ch] of QUAD) {
    assert.equal(BLOCK_MASK[cp - 0x2580], mask, `${ch} U+${cp.toString(16)}`);
  }
});

test('BLOCK_MASK covers the six multi-quadrant glyphs the old BLOCKS table dropped', () => {
  // ▙▚▛▜▞▟ cannot be expressed as a single [x,y,w,h] rect, so the previous
  // implementation fell through to ctx.fillText — the ~70ms/frame path, and a
  // visible divergence from the PNG export. This is the regression guard.
  //
  // Assert the exact mask, not merely non-zero: `notEqual(undefined, 0)` passes
  // against an empty table, which would make this test decoration.
  const dropped = [[0x2599, 13], [0x259a, 9], [0x259b, 7],
                   [0x259c, 11], [0x259e, 6], [0x259f, 14]];
  for (const [cp, mask] of dropped) {
    assert.equal(BLOCK_MASK[cp - 0x2580], mask,
      `U+${cp.toString(16)} still falls through to the font path`);
  }
});

test('BLOCK_MASK leaves unhandled codepoints in the 2580..259F page at zero', () => {
  // U+2581..2583 are eighth blocks, which ansi2png.py also does not model.
  // They must stay 0 so the painter defers them rather than inventing coverage.
  for (const cp of [0x2581, 0x2582, 0x2583, 0x2585]) {
    assert.equal(BLOCK_MASK[cp - 0x2580], 0, `U+${cp.toString(16)} should defer`);
  }
});
