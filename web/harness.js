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
    // Timed separately from the paint below. Frame() and the painter fail in
    // different ways and are tuned by different people; a single combined number
    // cannot tell you which side moved.
    const tRender = performance.now();
    const done = renderFrame(w, h, tick, buf);
    this.renderMs = performance.now() - tRender;

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

// --- the viewer -------------------------------------------------------------
//
// One page serves both audiences. A visitor gets the animation full-bleed with
// chrome that fades; `~` (or ?dev) slides the authoring panel up over the same
// canvas, driven by the same Pane above. There is deliberately no second painter:
// what gets tuned here is what a visitor sees, down to the pixel.

const el = (id) => document.getElementById(id);
const main = new Pane(el('main-canvas'));
const compare = new Pane(el('compare-canvas'));

const state = { w: 100, h: 30, tick: 0, fps: 30, playing: false, cell: 8, compare: false, offset: 0, period: -1 };
let lastFrameTime = 0, renderEMA = 0, paintEMA = 0;

// Set once the user types a Δ of their own, so the auto-derived value stops
// overwriting it. Without this, torus's per-resize recompute would silently stomp
// a hand-entered Δ the moment you drag the pane — which is exactly the workflow
// the compare pane exists for.
let deltaTouched = false;

// The module may be older than this page: web/*.wasm is gitignored and a browser
// can serve a cached one. Without this guard a missing export throws inside
// readControls and the page renders nothing at all.
const periodFor = (w, h) =>
  typeof animPeriod === 'function' ? (animPeriod(w, h) | 0) : -1;

// Drive the tick slider's range and the compare Δ from the animation itself,
// rather than the nebula-shaped constants this page used to hardcode.
function applyPeriod() {
  const p = state.period;
  const toggle = el('compare-toggle'), offset = el('offset');

  if (p > 0) {
    el('tick').max = p;
    toggle.disabled = offset.disabled = false;
    el('loop-note').textContent = `loops every ${p} ticks`;
    if (!deltaTouched) offset.value = p;
  } else if (p === 0) {
    // Free-running. Say so, rather than leaving a Δ that can never match.
    toggle.checked = false;
    toggle.disabled = offset.disabled = true;
    el('loop-note').textContent = 'free-running, never repeats';
  } else {
    el('loop-note').textContent = '';
  }
}

function readControls() {
  state.w = Math.max(1, +el('width').value | 0);
  state.h = Math.max(1, +el('height').value | 0);
  state.fps = Math.max(1, +el('fps').value | 0);
  state.cell = Math.max(2, +el('cell').value | 0);

  // Recomputed on every pane change: torus's loop scales with the pane, so a Δ
  // that was right at one size is wrong at the next.
  state.period = periodFor(state.w, state.h);
  applyPeriod();

  state.offset = +el('offset').value | 0;
  state.compare = el('compare-toggle').checked && state.period !== 0;
  el('compare-wrap').style.display = state.compare ? 'block' : 'none';
  el('dims').textContent = `${state.w}×${state.h}`;
}

function paint() {
  const t0 = performance.now();
  const done = main.draw(state.w, state.h, state.tick, state.cell, state.cell * 2);
  if (state.compare) {
    compare.draw(state.w, state.h, state.tick + state.offset, state.cell, state.cell * 2);
  }
  // main.renderMs is the Go call; the rest of the wall time is paint.
  const total = performance.now() - t0;
  const render = main.renderMs || 0;
  renderEMA = renderEMA ? renderEMA * 0.9 + render * 0.1 : render;
  paintEMA = paintEMA ? paintEMA * 0.9 + (total - render) * 0.1 : total - render;

  el('tick-label').textContent = state.tick;
  el('perf').textContent =
    `render ${renderEMA.toFixed(1)} / paint ${paintEMA.toFixed(1)} ms`;

  // Any cell on the font path neither matches the PNG export nor stays on the
  // fast path, so make it loud rather than letting it show up as a mystery stall.
  const d = main.deferred || 0;
  el('deferred').textContent = d ? `⚠ ${d} glyphs via font` : '';
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
  // `|| 1` guards the free-running case, where max is 0 and the modulo is NaN.
  el('tick').value = state.tick % (+el('tick').max || 1);
  // A resolving animation reports done; park on the final frame rather than
  // running past the end of it.
  if (paint()) stop();
}

const availableHeight = () => window.innerHeight -
  (document.body.classList.contains('dev') ? el('dev').getBoundingClientRect().height : 0);

// Fill the viewport at the current cell size — the authoring check that an
// animation reflows across the resolution ladder, unchanged.
function fitToWindow() {
  el('width').value = Math.max(20, Math.floor(window.innerWidth / state.cell));
  el('height').value = Math.max(8, Math.floor(availableHeight() / (state.cell * 2)));
  readControls();
  paint();
}

// The visitor's view targets a column count rather than a cell size, which keeps
// both the density and the frame budget stable across screens.
//
// Measured on nebula at 1920×960, render+paint per frame:
//
//   cell  8 → 240×60 (14400 cells)  31.4 ms → 32 fps ceiling
//   cell 10 → 192×48  (9216 cells)  22.8 ms → 44 fps
//   cell 12 → 160×40  (6400 cells)  15.1 ms → 66 fps
//
// Cost tracks cell count almost linearly, so filling a large screen at cell 8
// leaves no headroom at all for a 30 fps target — on any machine slower than this
// one it drops frames. ~190 columns is both the wider end of a real terminal and
// comfortably inside budget.
const TARGET_COLS = 190;

function fitViewport() {
  state.cell = Math.max(6, Math.min(16, Math.round(window.innerWidth / TARGET_COLS)));
  el('cell').value = state.cell;
  fitToWindow();
}

// The one place that decides which of the two fits applies. In dev the cell size
// is the author's choice, so only reflow the pane; on the visitor view re-derive
// the cell so density holds across window sizes. Every caller goes through here —
// a second copy of this branch is how the wasm-ready path came to overwrite the
// author's cell the moment the module landed.
function fitPane() {
  if (document.body.classList.contains('dev')) fitToWindow(); else fitViewport();
}

// --- chrome ----------------------------------------------------------------

let idleTimer = 0;
function wake() {
  document.body.classList.remove('idle');
  clearTimeout(idleTimer);
  // Only hide the chrome while something is actually moving; on a paused frame
  // it is the only thing telling you what you are looking at.
  idleTimer = setTimeout(() => {
    if (state.playing && !document.body.classList.contains('dev')) {
      document.body.classList.add('idle');
    }
  }, 4000);
}
for (const ev of ['pointermove', 'keydown', 'pointerdown']) {
  window.addEventListener(ev, wake, { passive: true });
}

function setDev(on) {
  document.body.classList.toggle('dev', on);
  const url = new URL(location.href);
  if (on) url.searchParams.set('dev', ''); else url.searchParams.delete('dev');
  history.replaceState(null, '', url);
  wake();
  fitToWindow();
}

// --- controls --------------------------------------------------------------

for (const id of ['width', 'height', 'fps', 'cell', 'compare-toggle']) {
  el(id).addEventListener('input', () => { readControls(); paint(); });
}
// Δ is handled on its own rather than in the loop above, because the flag has to
// be set BEFORE readControls runs — otherwise applyPeriod overwrites the value
// being typed in the same event, and a hand-entered Δ silently reverts.
el('offset').addEventListener('input', () => {
  deltaTouched = true;
  readControls();
  paint();
});
el('tick').addEventListener('input', (e) => {
  stop();
  state.tick = +e.target.value;
  paint();
});
el('play').addEventListener('click', () => {
  state.playing = !state.playing;
  el('play').textContent = state.playing ? '⏸ pause' : '▶ play';
  wake();
});
el('fit').addEventListener('click', fitToWindow);
el('step').addEventListener('click', () => { state.tick++; paint(); });

// Keep the slider in step with keyboard scrubbing — otherwise the thumb sits
// wherever it last was and stops telling you where in the loop you are.
function seek(tick) {
  state.tick = Math.max(0, tick);
  el('tick').value = state.tick % (+el('tick').max || 1);
  paint();
}
window.addEventListener('keydown', (e) => {
  if (e.target.matches('input, select, button')) return;
  if (e.key === '`' || e.key === '~') { e.preventDefault(); setDev(!document.body.classList.contains('dev')); }
  if (e.key === 'Escape') location.href = './index.html';
  if (e.key === ' ') { e.preventDefault(); el('play').click(); }
  if (e.key === 'ArrowRight') seek(state.tick + 1);
  if (e.key === 'ArrowLeft') seek(state.tick - 1);
});

let resizeTimer = 0;
window.addEventListener('resize', () => {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(fitPane, 120);
});

// --- boot -------------------------------------------------------------------

const anim = new URLSearchParams(location.search).get('anim') || 'nebula';
document.title = `terminal-animations — ${anim}`;
el('anim-name').textContent = anim;

// Paint the still immediately. A missing poster is not an error — scripts/harness.sh
// does not render them by default, so a local authoring run simply starts on black.
// Two shapes exist because the live canvas reflows rather than scales; pick the
// one whose aspect matches this viewport, so the hand-off to the live canvas is a
// change of resolution rather than of composition.
let posterReady = false;
el('poster').addEventListener('load', () => { posterReady = true; });
el('poster').addEventListener('error', () => { el('poster').style.display = 'none'; });
const posterShape = window.innerHeight > window.innerWidth ? '-portrait' : '';
el('poster').src = `./posters/${anim}${posterShape}.png`;

// Once the live canvas is up the poster will never be shown again, so drop it.
// An <img> left in the DOM keeps its decoded bitmap resident even at opacity 0 —
// at 1900x960 RGBA that is 7MB held for nothing, about a third of what this tab
// costs. Waits for the fade so the swap stays seamless.
function releasePosterAfterFade() {
  const poster = el('poster');
  if (!poster || !poster.src) return;
  const drop = () => {
    poster.removeAttribute('src');
    poster.remove();
  };
  poster.addEventListener('transitionend', drop, { once: true });
  // transitionend does not fire if the element was already transparent, or in a
  // background tab where the transition never runs; 1s is well past the 0.35s fade.
  setTimeout(drop, 1000);
}

// Called from Go once the runtime is up and renderFrame is installed.
window.onWasmReady = () => {
  el('status').textContent = '';
  fitPane();

  // Cross-fade off the poster. fitPane() paints synchronously just above, so
  // the canvas already holds a real frame and there is nothing to wait for —
  // deferring this to requestAnimationFrame would strand the poster on screen in a
  // background tab, where rAF is throttled until the tab is focused.
  document.body.classList.add('live');
  releasePosterAfterFade();
  state.playing = true;
  el('play').textContent = '⏸ pause';
  requestAnimationFrame(loop);
  wake();
};

// animations.json carries per-animation metadata (title, blurb, ladder rung,
// accent, loop shape), merged at build time from each examples/<name>/meta.json
// by scripts/manifest.py, which both scripts/harness.sh and pages.yml call.
// A bare array of names is still accepted because that is what harness.sh wrote
// before manifest.py existed, and a stale animations.json may still be cached in
// a browser or sitting in someone's local build directory.
fetch('animations.json')
  .then((r) => (r.ok ? r.json() : Promise.reject()))
  .then((manifest) => {
    const names = Array.isArray(manifest) ? manifest : Object.keys(manifest);
    const meta = Array.isArray(manifest) ? {} : (manifest[anim] || {});

    if (meta.accent) document.documentElement.style.setProperty('--accent', meta.accent);
    if (meta.title) el('anim-name').textContent = meta.title;
    if (meta.blurb) el('anim-blurb').textContent = meta.blurb;
    if (meta.rungName) el('anim-rung').textContent = meta.rungName;

    // A picker offering the one animation already on screen is a control that
    // cannot do anything, so it stays hidden below two. A local single-animation
    // build (scripts/harness.sh examples/nebula) is the common case for this.
    if (names.length < 2) return;
    el('anim-picker-row').hidden = false;

    const picker = el('anim-picker');
    for (const name of names) {
      const opt = document.createElement('option');
      opt.value = opt.textContent = name;
      opt.selected = name === anim;
      picker.append(opt);
    }
    picker.addEventListener('change', () => {
      location.search = `?anim=${encodeURIComponent(picker.value)}`;
    });
  })
  .catch(() => {});

function loadModule() {
  document.body.classList.remove('gated');
  el('status').textContent = 'loading…';
  const go = new Go();
  WebAssembly.instantiateStreaming(fetch(`${anim}.wasm`), go.importObject)
    .then((res) => go.run(res.instance))
    .catch((err) => {
      el('status').textContent = `failed to load ${anim}.wasm: ${err.message}`;
      el('status').className = 'err';
    });
}

// Two reasons to hold the module back, both of which leave the poster on screen
// so the animation is still *seen* — just not moving.
//
//   reduced motion — a public page that autoplays full-screen motion has to take
//     this seriously. We do not merely pause: the ~2MB module is never fetched.
//   small screen   — 2MB unasked on a phone, possibly on cellular, is rude. The
//     poster is 60-90KB and shows the same frame.
//
// Either way playing is an explicit, per-visit choice.
const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
const smallScreen = window.matchMedia('(max-width: 760px), (pointer: coarse)');
el('play-anyway').addEventListener('click', loadModule);

if (reduceMotion.matches) {
  document.body.classList.add('gated');
  el('status').textContent = 'still frame — motion paused';
} else if (smallScreen.matches) {
  document.body.classList.add('gated');
  el('status').textContent = 'still frame';
  el('gate-title').textContent = 'tap to play';
  el('gate-body').textContent =
    'This is a still frame. Playing it downloads a 2 MB animation module.';
  el('play-anyway').textContent = 'play it';
} else {
  loadModule();
}

if (new URLSearchParams(location.search).has('dev')) setDev(true);
