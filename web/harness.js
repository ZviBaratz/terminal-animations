// harness.js — the authoring loop: scrub, resize, compare.
//
// The Go side hands us a packed Int32Array (fg, bg, glyph per cell); this file
// only paints. Block-element glyphs are drawn as filled sub-cell rectangles —
// the same model scripts/ansi2png.py uses — so what you see here matches the
// PNG/GIF export rather than approximating it with a font.

const CELL_INTS = 3;

// Sub-cell coverage for the block elements, as [x, y, w, h] in cell fractions.
// Anything not listed falls back to drawing the glyph with a monospace font.
const BLOCKS = {
  0x2588: [0, 0, 1, 1],       // █ full
  0x2580: [0, 0, 1, 0.5],     // ▀ upper half
  0x2584: [0, 0.5, 1, 0.5],   // ▄ lower half
  0x258c: [0, 0, 0.5, 1],     // ▌ left half
  0x2590: [0.5, 0, 0.5, 1],   // ▐ right half
  0x2596: [0, 0.5, 0.5, 0.5], // ▖
  0x2597: [0.5, 0.5, 0.5, 0.5], // ▗
  0x2598: [0, 0, 0.5, 0.5],   // ▘
  0x259d: [0.5, 0, 0.5, 0.5], // ▝
};

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

    // Glyphs with no block coverage are rare; defer them to a text pass so the
    // fast path stays branch-light.
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

        const box = BLOCKS[glyph];
        if (box === undefined) {
          (text || (text = [])).push(x0, y0, glyph, buf[i]);
          continue;
        }
        const fg = abgr(buf[i]);
        const sy = (box[1] * ch) | 0, sh = Math.ceil(box[3] * ch);
        const sx = (box[0] * cw) | 0, sw = Math.ceil(box[2] * cw);
        for (let y = sy; y < sy + sh; y++) {
          const o = (y0 + y) * pxW + x0 + sx;
          px.fill(fg, o, o + sw);
        }
      }
    }

    ctx.putImageData(this.img, 0, 0);

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
