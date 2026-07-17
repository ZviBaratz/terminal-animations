---
name: author-animation
description: >-
  Use when building any terminal / ASCII / ANSI animation — a splash screen, a
  loader or spinner, a generative background, a game-of-life-style sim, a
  typewriter or reveal, plasma, a starfield, digital rain, or any moving terminal
  graphic — and when deciding whether such an animation belongs in fresco or
  stands on its own.
---

# Authoring a terminal animation

## Overview

Terminal motion has no single right shape — a generative field and a game-of-life
sim want different code. So this skill **routes first**, builds to the convention
that fits, and finishes at a beauty gate: every animation ships *tested and
visually tuned*.

Two companions: `references/craft.md` is the universal craft — read it, it's the
rubric you tune against. For a fresco field variant, fresco's own `new-variant`
skill owns the contract and this skill hands off to it.

## Route first

Decide the target before writing code. Ask, in order:

1. **Is it a field?** — full-pane, a pure function of `(position, frame)`,
   palette-gradient coloured, loops forever, with *no subject or sprite* (plasma,
   rain, aurora, a starfield).
   - **…and you are inside the fresco repo** → it's a **fresco variant** → §A.
   - **…but you are not in the fresco repo** → you can't author a fresco variant
     from outside it, so build the same idea as a **standalone pure field** → §B.
2. **Otherwise** — it's stateful (carries a grid / particles), or has a
   subject/sprite, or *resolves* (a one-shot that ends), or is non-field motion →
   **standalone** → §B.

| The animation is… | Target |
|---|---|
| full-pane, pure `f(pos, frame)`, gradient, loops forever, no subject, **in fresco** | fresco variant (§A) |
| that same field shape, **outside fresco** | standalone, pure `Frame` (§B) |
| stateful / has a subject / resolves / non-field | standalone (§B) |

The dividing question: **"is frame N a closed-form function of N, with no
subject?"** Yes → a field. No → a stateful standalone.

## §A — Fresco variant: hand off, don't re-derive

When the target is a fresco variant and you are in the fresco repo, **invoke
fresco's `new-variant` skill and follow it.** That skill owns fresco's contract —
the `splashPointFn` shape, the registration touchpoints, the test guards. Do not
reconstruct that checklist here: duplicating it is exactly how the two drift
apart. This skill's whole job for a variant is to route you there.

## §B — Standalone animation: the convention

Author to this deliberately small, framework-free convention — chosen over pulling
in a TUI framework so the animation stays snapshot-testable and portable, and as
the seed of a possible future library:

```go
// pure & free-running (a plasma, a starfield): frame N from N alone
func Frame(w, h, tick int) string

// stateful or resolving (game of life, a typewriter, a wipe):
type Animation interface {
    Update(tick int)      // advance carried state
    View(w, h int) string // render current state: exactly h lines of w cells
    Done() bool           // true once a one-shot resolves; always false for a loop
}
```

The animation's *own shape* is this and only this. Driving it inside an
interactive program (Bubble Tea, a splash harness, a cycling menu) is a separate
integration step, out of scope here — keep it out of the animation's core.

A stateful `Animation` **owns its own dimensions**: construct it with the size it
should simulate, advance it with `Update(tick)` (which takes no size), and let
`View(w, h)` render the current state into the requested pane — re-seeding or
clamping if that size differs from the one it was built with.

**Deliverables:** the `Frame`/`Animation` code, a `cmd/preview/main.go` (copy
`scripts/preview.go.tmpl` and wire `render()`), and a test (below).

## The contract every animation keeps

From `craft.md` — hold every animation to it, field or standalone:

- **Shape:** exactly `h` lines of exactly `w` visible cells (`""` for a degenerate
  pane). **Width-1 glyphs only**; index glyph sets as `[]rune`, never `string`. For
  a smooth luminance field, the width-1-safe cell is a *space painted with a
  background colour* — no glyph set, no width traps; keep glyph ramps for texture.
- **Determinism:** a pure `Frame` takes animation only through `tick` and
  randomness only through an integer coordinate hash — so it is snapshot-testable.
  A stateful `Animation` can't be pure over `tick`, but pin a fixed seed and a
  fixed update order so its goldens are stable.
- **Craft:** motion wants a leading edge + trail; real negative space (a glyph in
  *every* cell is texture, not weather); no fixed bright points over moving parts;
  brightness split across glyph density vs colour luminance. **Tune by rendering a
  sweep and looking, not by arithmetic.**

## Test it

- **Bounds & safety:** exactly `h`×`w` across a spread of sizes; no panic on any
  `(w, h, tick)`, including tiny and zero-area panes.
- **Determinism:** a pure `Frame` → a golden frame is byte-stable, and
  `Frame(w,h,t) == Frame(w,h,t)`.
- **Stateful → canonical goldens:** the crisp assertions the thing admits — a Life
  blinker oscillates period-2, a glider returns to its shape shifted by (1,1) after
  4 steps, a still-life is fixed; a typewriter's `Done()` flips exactly when the
  last rune shows, and never splits a multibyte rune.

## Tune — the beauty gate

Do not ship on "tests pass": **watch it move, in colour.** Use `scripts/` —
`preview.sh` (live), the `frames` mode for a headless structure + SGR-bytes check,
`record.sh` for the GIF. Sweep each taste constant and pick by eye (see
`craft.md`). The optional **`tuner`** subagent drives this loop for you.

## Red flags

- Reaching for **Bubble Tea / `tea.Model` as the animation's shape.** That's a run
  loop, not the animation — the animation is a `Frame`/`Animation` (§B); wiring it
  into a TUI is a later, separate step.
- **Re-deriving fresco's variant checklist** while in the fresco repo. Hand off to
  `new-variant` (§A); don't duplicate the contract.
- "It's a field, but I'm not in fresco, so I'm stuck." → Build it standalone as a
  pure `Frame` (§B). fresco is where variants *live*, not the only way to make a
  field.
- "Bounds pass, the tests are green." → You haven't seen the colour move. Run the
  beauty gate.
- Motion from a wall clock, or randomness from `math/rand`, in a pure animation. →
  Breaks determinism; drive motion from `tick`, randomness from a coordinate hash.
