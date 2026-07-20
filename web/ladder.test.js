// ladder.test.js — run with: node --test web/
//
// The resolution ladder is a *view* of one label dimension, not a classifier: an
// animation carries a list of resolutions and appears under each rung it uses.
// These pin the grouping so the two things it must not do — drop an animation whose
// resolution is off the ladder, or file a multi-rung animation under only one rung —
// stay caught. groupByResolution is pure (no DOM), which is why it lives here and
// not in gallery.js.

import test from 'node:test';
import assert from 'node:assert/strict';

import { LADDER, groupByResolution, rungNames } from './ladder.js';

// Names for the assertions below, read from the table itself so a rename can't
// make these pass against stale expectations.
const nameOf = new Map(LADDER.map((s) => [s.rung, s.name]));
assert.equal(nameOf.get(1), 'half block');
assert.equal(nameOf.get(5), 'braille');

const namesIn = (bucket) => (bucket || []).map((a) => a.name);

test('a multi-rung animation appears under every rung it uses', () => {
  const { byRung } = groupByResolution({ hybrid: { resolutions: [1, 5] } });
  assert.ok(namesIn(byRung.get(1)).includes('hybrid'), 'missing from rung 1');
  assert.ok(namesIn(byRung.get(5)).includes('hybrid'), 'missing from rung 5');
});

test('a single-rung animation appears under that rung only', () => {
  const { byRung, unfiled } = groupByResolution({ nebula: { resolutions: [1] } });
  assert.deepEqual(namesIn(byRung.get(1)), ['nebula']);
  assert.deepEqual(namesIn(byRung.get(5)), []);
  assert.deepEqual(unfiled, []);
});

test('an off-ladder resolution goes to unfiled, never silently dropped', () => {
  const { byRung, unfiled } = groupByResolution({ weird: { resolutions: [9] } });
  assert.deepEqual(namesIn(unfiled), ['weird']);
  for (const s of LADDER) assert.deepEqual(namesIn(byRung.get(s.rung)), []);
});

test('a partly-off-ladder animation is filed under its known rung, not unfiled', () => {
  const { byRung, unfiled } = groupByResolution({ mostly: { resolutions: [1, 9] } });
  assert.ok(namesIn(byRung.get(1)).includes('mostly'));
  assert.deepEqual(namesIn(unfiled), []);
});

test('a legacy scalar rung is read as a one-element resolution list', () => {
  const { byRung } = groupByResolution({ old: { rung: 5 } });
  assert.deepEqual(namesIn(byRung.get(5)), ['old']);
});

test('a meta with neither field defaults to rung 1, matching manifest.py', () => {
  const { byRung } = groupByResolution({ bare: { title: 'bare' } });
  assert.deepEqual(namesIn(byRung.get(1)), ['bare']);
});

test('rungNames joins the names of the rungs used', () => {
  assert.equal(rungNames([1, 5]), 'half block + braille');
  assert.equal(rungNames([1]), 'half block');
  assert.equal(rungNames([5]), 'braille');
});

test('rungNames is empty for no known rung, so the viewer caption stays blank', () => {
  assert.equal(rungNames([]), '');
  assert.equal(rungNames([9]), '');
  assert.equal(rungNames(undefined), '');
});
