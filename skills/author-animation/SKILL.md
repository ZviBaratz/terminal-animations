---
name: author-animation
description: >-
  Use when building any terminal / ASCII / ANSI animation — a splash screen, a
  loader or spinner, a generative background, a game-of-life-style sim, a
  typewriter or reveal, plasma, a starfield, digital rain, a "mesmerizing" or
  "jaw-dropping" terminal effect, or any moving terminal graphic — and when
  deciding whether such an animation belongs in fresco or stands on its own.
---

# Authoring a terminal animation

## Overview

A conventional terminal animation is one effect, in flat 1×1 ASCII glyphs, with
functional colour. A *mesmerizing* one is composed: a deliberate fidelity tier, a
designed palette, layered effects, tuned by eye. This skill is the process that gets
you the second thing — **interrogate the vision → choose the target → compose past the
default → build to a testable convention → tune at a beauty gate.**

Pull in the references as each stage needs them — don't preload them:

- `references/craft.md` — the universal motion/beauty rubric. Read it before you tune.
- `references/techniques.md` — the resolution ladder, colour depth, dithering.
- `references/effects.md` — the catalog of effects, to *combine* not copy.
- `references/tools.md` — providers and build-time tools; **fresco is one provider**.

## The process

1. **Brief** — interrogate the vision (below).
2. **Select** the target: a fresco field variant, or a standalone animation.
3. **Compose** the piece — combine technique and style past the conventional default.
4. **Build** to the convention (§B), or hand off to fresco (§A).
5. **Tune** at the beauty gate — watch it move, in colour.

## 1 · Brief — interrogate the vision

You will already ask the logistics (language, size, loop-vs-resolve, colour support,
skippable) — those are cheap. The expert questions, the ones that change what you
build, are about **taste**, and they are the ones usually skipped:

- **The feeling & the reference.** What should it *evoke*? Is there a reference image,
  a demo, an effect to echo — or to subvert? "Mesmerizing" is not a spec; a mood or a
  reference is.
- **The fidelity intent.** Typographic (glyph ramp), colour raster (half-block →
  sextant), or fine mono detail (braille)? This is a design decision from
  `techniques.md`, not a default — name it now.
- **What makes *this* one special.** The transcendent pieces have one idea the generic
  version doesn't. Find it before writing code.

Ask **few and sharp** — only where the answer changes the build; default the rest and
say so. Then **state the concept back in one line** and build to it.

## 2 · Select the target

Decide before writing code. Ask, in order:

1. **Is it field-shaped?** — full-pane, a pure function of `(position, frame)`,
   palette-gradient coloured, loops forever, with *no subject or sprite* (plasma, rain,
   aurora, a starfield without streaks).
   - **…and you are inside the fresco repo** → it's a **fresco variant** → §A.
   - **…but you are not in the fresco repo** → build the same idea as a **standalone
     pure field** → §B.
2. **Otherwise** — stateful (carries a grid / particles), or has a subject/sprite, or
   *resolves* (a one-shot that ends), or is non-field motion → **standalone** → §B.

| The animation is… | Target |
|---|---|
| field-shaped, pure `f(pos, frame)`, gradient, loops forever, no subject, **in fresco** | fresco variant (§A) |
| that same field shape, **outside fresco** | standalone, pure `Frame` (§B) |
| stateful / has a subject / resolves / non-field | standalone (§B) |

"Field-shaped" is the *shape*; "fresco variant" is a *destination* you can only reach
from inside fresco. A field shape outside fresco is still a standalone pure `Frame`.

## §A — Fresco variant: hand off, don't re-derive

When the target is a fresco variant and you are in the fresco repo, **invoke fresco's
`new-variant` skill and follow it.** That skill owns fresco's contract — the
`splashPointFn` shape, the registration touchpoints, the test guards. Do not
reconstruct that checklist here: duplicating it is exactly how the two drift apart.
This skill's whole job for a variant is to route you there. (fresco is also a *provider*
you can use from outside — see `tools.md` — but authoring a new variant happens in the
repo.)

## §B — Standalone animation: the convention

Author to this deliberately small, framework-free convention — chosen over pulling in a
TUI framework so the animation stays snapshot-testable and portable, and as the seed of
a possible future library:

```go
// pure & free-running (a plasma, a starfield): frame N from N alone
func Frame(w, h, tick int) string

// stateful or resolving (game of life, a typewriter, a wipe):
type Animation interface {
    Update(tick int)      // advance carried state to absolute frame `tick`
    View(w, h int) string // render current state: exactly h lines of w cells
    Done() bool           // true once a one-shot resolves; always false for a loop
}
```

**Which shape.** A pure `Frame` requires *all* of: closed-form of `tick`, loops
forever, and carries no state. If the animation **resolves** (it needs `Done()`) or
**carries state**, it is an `Animation` — even a one-shot wipe whose fill is closed-form
(`filled == tick`), because a pure `Frame` has no channel to signal completion.

**Update / View / size.** `Update(tick)` takes the **absolute** frame counter — the same
`tick` a pure `Frame` gets, not a per-call delta — so it is idempotent for a given
`tick`. A stateful `Animation` **owns its own dimensions**: construct it with the size
it should simulate, advance it with `Update(tick)` (no size), and let `View(w, h)`
render into the requested pane, re-seeding or clamping if that size differs. A
**resolving** animation anchors its timeline and `Done()` to the size it was
constructed with, not to the `View` pane — so completion never shifts when it's viewed
at a different size.

Driving the animation inside an interactive program (Bubble Tea, a splash harness) is a
separate integration step, out of scope here — keep it out of the animation's core.

**Deliverables:** the `Frame`/`Animation` code, a `cmd/preview/main.go` (copy
`scripts/preview.go.tmpl` and wire `render()`), and a test (below).

## 3 · Compose — combine past the default

This is where a piece stops being conventional. The generic answer to any brief is one
effect in flat ASCII; compose past it:

- **Pick a fidelity tier deliberately** (`techniques.md`) — half-block raster and
  braille detail are different, higher objects than the `.·+*#@` ramp. Don't settle for
  1×1 ASCII by reflex.
- **Design the palette**, don't pick colours functionally. Split brightness across the
  two channels — glyph density vs colour luminance (`craft.md`) — and dither smooth
  gradients (Bayer for motion, `techniques.md`).
- **Layer.** A slow field wash (luminance) *behind* a subject/particles, with a focal
  **vignette** tying them together, reads as intentional depth, not noise.
- **Reach the ecosystem** (`tools.md`, `effects.md`) — use the **fresco** provider for a
  rain/tunnel/ripple/galaxy field; bake a **chafa/ffmpeg** source at build time; drive
  one effect with another. Don't rebuild what exists.
- **Keep it deterministic** so it stays testable (below).

## The contract every animation keeps

From `craft.md` — hold every animation to it, field or standalone:

- **Shape:** exactly `h` lines of exactly `w` visible cells (`""` for a degenerate
  pane). **Width-1 glyphs only**; index glyph sets as `[]rune`, never `string`. For a
  smooth luminance field, the width-1-safe cell is a *space painted with a background
  colour*.
- **Determinism:** a pure `Frame` takes animation only through `tick` and randomness
  only through an integer coordinate hash — so it is snapshot-testable. A stateful
  `Animation` can't be pure over `tick`, but pin a fixed seed and update order so its
  goldens are stable.
- **Craft:** leading edge + trail, real negative space, no fixed bright points over
  moving parts, brightness split across the two channels, **tune by looking not
  arithmetic** — the full rubric is `craft.md`.

## Test it

- **Bounds & safety:** exactly `h`×`w` across a spread of sizes; no panic on any
  `(w, h, tick)`, including tiny and zero-area panes.
- **Determinism:** a pure `Frame` → a golden frame is byte-stable, and
  `Frame(w,h,t) == Frame(w,h,t)`.
- **Stateful → canonical goldens:** the crisp assertions the thing admits — a Life
  blinker oscillates period-2, a glider returns to its shape shifted by (1,1) after 4
  steps, a still-life is fixed; a typewriter's `Done()` flips exactly when the last rune
  shows, and never splits a multibyte rune.

## 5 · Tune — the beauty gate

Do not ship on "tests pass": **watch it move, in colour.** Use `scripts/` — `preview.sh`
(live), the `frames` mode for a headless structure check, and `scripts/ansi2png.py` to
rasterize `frames` into a PNG you can actually look at when there's no TTY (a sandbox,
CI, an agent); `record.sh` for the GIF. Sweep each taste constant and pick by eye (see
`craft.md`). The optional **`tuner`** subagent drives this loop for you.

## Red flags

- **Settling for 1×1 ASCII glyphs** when the resolution ladder (`techniques.md`) would
  look far better. The flat `.·+*#@` starfield is the conventional default — climb.
- **Functional colour** (grey→white→cyan by reflex) instead of a *designed* palette.
- **Rebuilding rain / tunnel / ripple / galaxy from scratch** instead of using the
  fresco provider (`tools.md`).
- **Interrogating the author** about things you could just default — ask only what
  changes the build.
- Reaching for **Bubble Tea / `tea.Model` as the animation's shape.** That's a run loop,
  not the animation — the animation is a `Frame`/`Animation` (§B).
- **Re-deriving fresco's variant checklist** while in the fresco repo. Hand off (§A).
- "Bounds pass, the tests are green." → You haven't seen the colour move. Run the
  beauty gate.
- Motion from a wall clock, or randomness from `math/rand`, in a pure animation. →
  Breaks determinism; drive motion from `tick`, randomness from a coordinate hash.
