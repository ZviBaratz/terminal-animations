// harness.js — the authoring loop: scrub, resize, compare.
//
// The Go side hands us a packed Int32Array (fg, bg, glyph per cell); this file
// only paints. Block elements (2x2 sub-cells) and braille (2x4 dots) are drawn as
// filled sub-cell rectangles — the same model scripts/ansi2png.py uses — so what
// you see here matches the PNG/GIF export rather than approximating it with a font.
//
// That equivalence is verifiable, not aspirational: render any frame through
// `ansi2png.py --cw N --ch 2N` and through this painter at `cell=N`, and the two
// images are pixel-identical. Anything that falls through to the font path is
// counted in `Pane.deferred` and surfaced in the status line, because it is both
// far slower and no longer guaranteed to match.

import { bandEdges, BRAILLE_MASK, BLOCK_MASK } from './glyphs.js';

const CELL_INTS = 3;

const hex = (v) => '#' + (v >>> 0).toString(16).padStart(6, '0');

// 0xRRGGBB -> the ABGR word an ImageData Uint32 view wants on little-endian.
const abgr = (v) => 0xff000000 | ((v & 0xff) << 16) | (v & 0xff00) | ((v >> 16) & 0xff);

class Pane {
  constructor(canvas) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d', { alpha: false });
    this.buf = null;
    this.img = null;
    this.px = null;
  }

  // Ensure the shared buffer is big enough for w*h cells. Reallocating only on
  // growth keeps scrubbing allocation-free once the pane settles.
  ensure(w, h) {
    const need = w * h * CELL_INTS;
    if (!this.buf || this.buf.length < need) this.buf = new Int32Array(need);
    return this.buf;
  }

  // Paint straight into an ImageData word array. The earlier fillRect/fillStyle
  // path cost ~70ms/frame at 220x56, almost all of it allocating a CSS colour
  // string per cell; these frames are ~100% block glyphs, so filling horizontal
  // pixel runs instead avoids the 2D context per-cell entirely.
  draw(w, h, tick, cw, ch) {
    if (typeof renderFrame !== 'function') return false;
    const buf = this.ensure(w, h);
    const done = renderFrame(w, h, tick, buf);

    const { canvas, ctx } = this;
    const pxW = w * cw, pxH = h * ch;
    if (canvas.width !== pxW || canvas.height !== pxH) {
      canvas.width = pxW;
      canvas.height = pxH;
      this.img = ctx.createImageData(pxW, pxH);
      this.px = new Uint32Array(this.img.data.buffer);
    }
    const px = this.px;

    // Sub-cell geometry, hoisted: it depends only on the cell size, not the cell.
    // Both tiers are 2 columns wide, so one x split serves them; they differ only
    // in row count. bandEdges tiles exactly, so bands never overlap — which matters
    // here in a way it did not for the old one-rect-per-glyph path, because a
    // multi-band glyph would otherwise paint over its own background.
    const xm = bandEdges(cw, 2)[1];
    const rows2 = bandEdges(ch, 2);
    const rows4 = bandEdges(ch, 4);

    // Glyphs neither tier models are rare; defer them to a text pass so the fast
    // path stays branch-light. Counted so the fallback cannot go unnoticed — it is
    // an order of magnitude slower and renders in whatever font the machine has.
    let text = null;

    for (let row = 0; row < h; row++) {
      for (let col = 0; col < w; col++) {
        const i = (row * w + col) * CELL_INTS;
        const glyph = buf[i + 2];
        const bg = abgr(buf[i + 1]);
        const x0 = col * cw, y0 = row * ch;

        for (let y = 0; y < ch; y++) {
          const o = (y0 + y) * pxW + x0;
          px.fill(bg, o, o + cw);
        }
        if (glyph === 32) continue;

        // Braille first, on one shift-compare: a wireframe is ~100% braille, and
        // this keeps it off the block table entirely. Exact for U+2800..U+28FF.
        let mask, edges, full;
        if ((glyph >> 8) === 0x28) {
          mask = BRAILLE_MASK[glyph & 0xff];
          // U+2800 is a genuine blank — the background is already correct.
          if (mask === 0) continue;
          edges = rows4;
          full = 0xff;
        } else if (glyph >= 0x2580 && glyph <= 0x259f) {
          mask = BLOCK_MASK[glyph - 0x2580];
          // 0 here means "not modelled" (the eighth blocks), not "empty" — defer,
          // rather than silently leaving the cell as background.
          if (mask === 0) {
            (text || (text = [])).push(x0, y0, glyph, buf[i]);
            continue;
          }
          edges = rows2;
          full = 0xf;
        } else {
          (text || (text = [])).push(x0, y0, glyph, buf[i]);
          continue;
        }

        const fg = abgr(buf[i]);

        // A fully-lit cell is one flat run per pixel row, same as the old █ path.
        // Worth special-casing: it is the common case for dense block fields, and
        // it bounds a braille cell's cost at exactly a full block's.
        if (mask === full) {
          for (let y = 0; y < ch; y++) {
            const o = (y0 + y) * pxW + x0;
            px.fill(fg, o, o + cw);
          }
          continue;
        }

        // Decompose by row band, not by dot: within a band only the two columns
        // vary, so a band is at most one fill per pixel row — and a both-columns
        // band (the common case in dense line art) collapses to a single full-width
        // fill rather than two half-width ones.
        for (let r = 0; r < edges.length - 1; r++) {
          const b = (mask >> (r * 2)) & 3;
          if (b === 0) continue;
          const fx0 = b === 2 ? xm : 0;
          const fx1 = b === 1 ? xm : cw;
          for (let y = y0 + edges[r]; y < y0 + edges[r + 1]; y++) {
            const o = y * pxW + x0;
            px.fill(fg, o + fx0, o + fx1);
          }
        }
      }
    }

    ctx.putImageData(this.img, 0, 0);

    // Surfaced in the status line. Any non-zero count means cells are rendering
    // through a font rather than the sub-cell model — so they neither match the
    // PNG export nor stay on the fast path, and on most machines the glyph is not
    // in a monospace face at all and lands proportional and off-grid.
    this.deferred = text ? text.length / 4 : 0;

    if (text) {
      ctx.textBaseline = 'top';
      ctx.font = `${ch}px ui-monospace, SFMono-Regular, Menlo, monospace`;
      for (let i = 0; i < text.length; i += 4) {
        ctx.fillStyle = hex(text[i + 3]);
        ctx.fillText(String.fromCodePoint(text[i + 2]), text[i], text[i + 1]);
      }
    }
    return done === true;
  }
}

const el = (id) => document.getElementById(id);
const main = new Pane(el('main-canvas'));
const compare = new Pane(el('compare-canvas'));

const state = { w: 100, h: 30, tick: 0, fps: 30, playing: false, cell: 8, compare: false, offset: 0 };
let lastFrameTime = 0, fpsEMA = 0;

function readControls() {
  state.w = Math.max(1, +el('width').value | 0);
  state.h = Math.max(1, +el('height').value | 0);
  state.fps = Math.max(1, +el('fps').value | 0);
  state.cell = Math.max(2, +el('cell').value | 0);
  state.offset = +el('offset').value | 0;
  state.compare = el('compare-toggle').checked;
  el('compare-wrap').style.display = state.compare ? 'block' : 'none';
  el('dims').textContent = `${state.w}×${state.h}`;
}

function paint() {
  const t0 = performance.now();
  const done = main.draw(state.w, state.h, state.tick, state.cell, state.cell * 2);
  if (state.compare) {
    compare.draw(state.w, state.h, state.tick + state.offset, state.cell, state.cell * 2);
  }
  const ms = performance.now() - t0;
  fpsEMA = fpsEMA ? fpsEMA * 0.9 + ms * 0.1 : ms;
  el('tick-label').textContent = state.tick;
  el('perf').textContent = `${fpsEMA.toFixed(1)} ms/frame`;
  return done;
}

function stop() {
  state.playing = false;
  el('play').textContent = '▶ play';
}

function loop(now) {
  requestAnimationFrame(loop);
  if (!state.playing) return;
  if (now - lastFrameTime < 1000 / state.fps) return;
  lastFrameTime = now;
  state.tick++;
  el('tick').value = state.tick % +el('tick').max;
  // A resolving animation reports done; park on the final frame rather than
  // running past the end of it.
  if (paint()) stop();
}

// Fit the pane to the viewport at the current cell size — the fastest way to
// check that an animation actually reflows across the resolution ladder.
function fitToWindow() {
  const pad = 48;
  el('width').value = Math.max(20, Math.floor((window.innerWidth - pad) / state.cell));
  el('height').value = Math.max(8, Math.floor((window.innerHeight - 220) / (state.cell * 2)));
  readControls();
  paint();
}

for (const id of ['width', 'height', 'fps', 'cell', 'offset', 'compare-toggle']) {
  el(id).addEventListener('input', () => { readControls(); paint(); });
}
el('tick').addEventListener('input', (e) => {
  stop();
  state.tick = +e.target.value;
  paint();
});
el('play').addEventListener('click', () => {
  state.playing = !state.playing;
  el('play').textContent = state.playing ? '⏸ pause' : '▶ play';
});
el('fit').addEventListener('click', fitToWindow);
el('step').addEventListener('click', () => { state.tick++; paint(); });
window.addEventListener('keydown', (e) => {
  if (e.key === ' ') { e.preventDefault(); el('play').click(); }
  if (e.key === 'ArrowRight') { state.tick++; paint(); }
  if (e.key === 'ArrowLeft') { state.tick = Math.max(0, state.tick - 1); paint(); }
});

// Called from Go once the runtime is up and renderFrame is installed.
window.onWasmReady = () => {
  el('status').textContent = 'ready';
  el('status').className = 'ok';
  readControls();
  paint();
  requestAnimationFrame(loop);
};

// Which animation to load: ?anim=<name> resolves to <name>.wasm, so one harness
// serves every animation built by scripts/harness.sh.
const anim = new URLSearchParams(location.search).get('anim') || 'nebula';
document.title = `terminal-animations — ${anim}`;
el('anim-name').textContent = anim;

// animations.json is written by scripts/harness.sh and by the pages workflow.
// It's what lets a hosted visitor discover the other animations; when it's
// absent (a bare `python3 -m http.server` in web/) the picker just stays hidden.
fetch('animations.json')
  .then((r) => (r.ok ? r.json() : Promise.reject()))
  .then((names) => {
    if (!Array.isArray(names) || names.length < 2) return;
    const picker = el('anim-picker');
    for (const name of names) {
      const opt = document.createElement('option');
      opt.value = opt.textContent = name;
      opt.selected = name === anim;
      picker.append(opt);
    }
    picker.hidden = false;
    picker.addEventListener('change', () => {
      location.search = `?anim=${encodeURIComponent(picker.value)}`;
    });
  })
  .catch(() => {});

const go = new Go();
WebAssembly.instantiateStreaming(fetch(`${anim}.wasm`), go.importObject)
  .then((res) => go.run(res.instance))
  .catch((err) => {
    el('status').textContent = `failed to load ${anim}.wasm: ${err.message}`;
    el('status').className = 'err';
  });
