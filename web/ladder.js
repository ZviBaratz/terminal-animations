// ladder.js — the resolution ladder, as a label dimension.
//
// The sub-cell resolution of a cell (half-block → braille) is one technique an
// animation *uses*, not a bucket it *is*. So each animation carries a list of
// `resolutions`, and the ladder is the view of that one dimension: an animation
// lists under every rung it uses, and the rungs nothing has shipped on stay as
// visible gaps. This module is the single source of truth for that dimension —
// the rung table, the grouping, and the display names — shared by the gallery
// (rendering), the viewer (its caption), and the tests. It is pure: no DOM, no
// fetch, so node --test can exercise the grouping without a browser.
//
// The rungs come from skills/author-animation/references/techniques.md.

export const LADDER = [
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

const NAME_BY_RUNG = new Map(LADDER.map((s) => [s.rung, s.name]));

// The resolutions an animation uses. A list is the model; a legacy scalar `rung`
// is read as a one-element list, and a meta with neither defaults to rung 1 — the
// same default scripts/manifest.py writes for an animation with no meta.json, so a
// missing field lands an animation on the workhorse rung rather than nowhere.
export function resolutionsOf(meta) {
  if (Array.isArray(meta.resolutions)) return meta.resolutions;
  if (typeof meta.rung === 'number') return [meta.rung];
  return [1];
}

// Group a manifest into the ladder. Returns a bucket per known rung (empty if
// nothing shipped there) plus `unfiled` — animations whose resolutions are all off
// the ladder. Unfiled is the point: the old scalar model dropped an unknown rung
// with no row and no warning, so an animation could vanish from the index entirely.
export function groupByResolution(manifest, ladder = LADDER) {
  const byRung = new Map(ladder.map((s) => [s.rung, []]));
  const known = new Set(byRung.keys());
  const unfiled = [];

  for (const [name, meta] of Object.entries(manifest)) {
    const anim = { name, ...meta };
    let filed = false;
    for (const r of resolutionsOf(meta)) {
      if (known.has(r)) {
        byRung.get(r).push(anim);
        filed = true;
      }
    }
    if (!filed) unfiled.push(anim);
  }

  return { byRung, unfiled };
}

// The display name(s) for a set of resolutions, for the viewer's caption:
// [1] → "half block", [1,5] → "half block + braille". Off-ladder values are
// skipped, so an unknown resolution yields "" and the caption stays blank rather
// than showing a stray "undefined".
export function rungNames(resolutions) {
  const list = Array.isArray(resolutions)
    ? resolutions
    : (typeof resolutions === 'number' ? [resolutions] : []);
  return list.map((r) => NAME_BY_RUNG.get(r)).filter(Boolean).join(' + ');
}
