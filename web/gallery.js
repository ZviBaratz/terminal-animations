// gallery.js — the index.
//
// Ships no WASM. The ladder samples are painted with the same sub-cell tables the
// real painter uses (glyphs.js), which is why they can show rungs no font can
// set: nothing on this machine has sextant, octant, or braille coverage, so a
// typeset ladder would have been three rows of tofu.

import { bandEdges, BRAILLE_MASK, BLOCK_MASK } from './glyphs.js';

// The resolution ladder from references/techniques.md, in full — including the
// rungs nothing has shipped on yet. The gaps are the point: they say what the
// project has not done, which is how the rest of the repo talks.
const LADDER = [
  {
    rung: 1, name: 'half block', glyphs: '▀▄▌▐',
    detail: '1×2 cell · two independent 24-bit pixels',
    note: 'the sweet spot — works everywhere',
  },
  {
    rung: 2, name: 'quadrant', glyphs: '▖▗▘▝▚▞',
    detail: '2×2 cell · two colours',
  },
  {
    rung: 3, name: 'sextant', glyphs: '🬀🬁🬂🬃',
    detail: '2×3 cell · two colours',
    note: 'the headless gate cannot see it — it collapses to solid fg',
  },
  {
    rung: 4, name: 'octant', glyphs: '',
    detail: '2×4 cell · two colours',
    note: 'the headless gate drops it entirely, shearing the row',
  },
  {
    rung: 5, name: 'braille', glyphs: '⠿⠇⠙⣿',
    detail: '2×4 dots · monochrome',
    note: 'the finest grid, at the cost of colour',
  },
];

// Draw a sample of a rung using the painter's own model, at a size that shows the
// sub-cell structure rather than hiding it.
function drawSample(canvas, rung) {
  const CW = 13, CH = 26, COLS = 8;
  const dpr = Math.min(2, window.devicePixelRatio || 1);
  canvas.width = COLS * CW * dpr;
  canvas.height = CH * dpr;
  canvas.style.width = `${COLS * CW}px`;
  canvas.style.height = `${CH}px`;

  const ctx = canvas.getContext('2d');
  ctx.scale(dpr, dpr);
  const accent = getComputedStyle(canvas).getPropertyValue('--sample').trim() || '#c8c2d8';

  const cells = sampleCells(rung, COLS);
  const xm = bandEdges(CW, 2)[1];

  for (let c = 0; c < COLS; c++) {
    const { mask, rows } = cells[c];
    if (!mask) continue;
    const edges = bandEdges(CH, rows);
    ctx.fillStyle = accent;
    for (let r = 0; r < rows; r++) {
      const b = (mask >> (r * 2)) & 3;
      if (b === 0) continue;
      const x0 = c * CW + (b === 2 ? xm : 0);
      const w = b === 3 ? CW : xm;
      ctx.fillRect(x0, edges[r], w, edges[r + 1] - edges[r]);
    }
  }
}

// What each rung's sample shows. Rungs the painter does not model are drawn as
// what they actually are — nothing — rather than faked with a stand-in.
function sampleCells(rung, cols) {
  const out = [];
  if (rung === 1 || rung === 2) {
    const cps = rung === 1
      ? [0x2580, 0x2584, 0x258c, 0x2590, 0x2588, 0x2580, 0x2584, 0x258c]
      : [0x2598, 0x259d, 0x2596, 0x2597, 0x259a, 0x259e, 0x259b, 0x259f];
    for (let i = 0; i < cols; i++) out.push({ mask: BLOCK_MASK[cps[i % cps.length] - 0x2580], rows: 2 });
  } else if (rung === 5) {
    const bits = [0xff, 0x07, 0x38, 0x5a, 0xa5, 0xc3, 0x0f, 0xf0];
    for (let i = 0; i < cols; i++) out.push({ mask: BRAILLE_MASK[bits[i % bits.length]], rows: 4 });
  } else {
    for (let i = 0; i < cols; i++) out.push({ mask: 0, rows: 2 });
  }
  return out;
}

const esc = (s) => String(s).replace(/[&<>"]/g, (c) =>
  ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

function render(manifest) {
  const byRung = new Map();
  for (const [name, m] of Object.entries(manifest)) {
    if (!byRung.has(m.rung)) byRung.set(m.rung, []);
    byRung.get(m.rung).push({ name, ...m });
  }

  const list = document.getElementById('ladder');
  list.innerHTML = '';

  for (const step of LADDER) {
    const animations = byRung.get(step.rung) || [];
    const row = document.createElement('section');
    row.className = 'rung' + (animations.length ? '' : ' empty');

    row.innerHTML = `
      <div class="rung-head">
        <span class="rung-no display">${String(step.rung).padStart(2, '0')}</span>
        <canvas class="sample" aria-hidden="true"></canvas>
        <span class="rung-name">${esc(step.name)}</span>
        <span class="rung-detail">${esc(step.detail)}</span>
      </div>
      ${step.note ? `<p class="rung-note">${esc(step.note)}</p>` : ''}
      ${animations.length
        ? `<ul class="anims">${animations.map((a) => `
            <li style="--accent:${esc(a.accent || '#c8c2d8')}">
              <a href="./view.html?anim=${encodeURIComponent(a.name)}">
                <span class="anim-title display">${esc(a.title || a.name)}</span>
                <span class="anim-blurb">${esc(a.blurb || '')}</span>
                <span class="anim-loop">${esc(a.loop || '')}</span>
              </a>
            </li>`).join('')}</ul>`
        : `<p class="nothing">nothing shipped here yet</p>`}
    `;
    list.append(row);
    drawSample(row.querySelector('.sample'), step.rung);
  }
}

fetch('animations.json')
  .then((r) => (r.ok ? r.json() : Promise.reject()))
  .then((m) => render(Array.isArray(m) ? Object.fromEntries(m.map((n) => [n, { rung: 1, title: n }])) : m))
  .catch(() => {
    // No manifest means nothing was built. Say that, rather than showing an empty
    // ladder that looks like the project has no animations.
    document.getElementById('ladder').innerHTML =
      '<p class="nothing">no animations built — run scripts/harness.sh, or see the workflow</p>';
  });
