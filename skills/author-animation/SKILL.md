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
designed palette, layered effects, tuned by eye. This skill is the process that gets the
second thing — **interrogate the vision → choose the target → compose past the default →
build to a testable convention → tune at a beauty gate.** Pull in the references as a
stage needs them: `craft.md` (the motion/beauty rubric), `techniques.md` (resolution
ladder, colour, dithering), `effects.md` (effects to *combine*, not copy), `tools.md`
(providers and build-time tools — **fresco is one provider**).

> **Where the bundled files live.** The references and the tuning harness ship *with the
> plugin*, not in the user's project — reach them by absolute path, never a bare relative
> one (a bare path resolves against the user's cwd, where these files do not exist).
> References are in `${CLAUDE_PLUGIN_ROOT}/skills/author-animation/references/`; the harness
> is in `${CLAUDE_PLUGIN_ROOT}/scripts/`. Paths like `./cmd/preview` *are* the user's
> project and stay relative.

## 1 · Brief — interrogate the vision

You will already ask the logistics (language, size, loop-vs-resolve, colour support,
skippable) — those are cheap. The expert questions, usually skipped, are about **taste**:

- **The feeling & the reference.** What should it *evoke*? A reference image, a demo, an
  effect to echo or subvert? "Mesmerizing" is not a spec; a mood or a reference is.
- **The fidelity intent.** Typographic (glyph ramp), colour raster (half-block →
  sextant), or fine mono detail (braille)? A design decision from `techniques.md` — name
  it now, don't default it.
- **What makes *this* one special.** The transcendent pieces have one idea the generic
  version lacks. Find it before writing code.

Ask **few and sharp** — only where the answer changes the build; default the rest and
say so. Then **state the concept back in one line** and build to it.

## 2 · Select the target

Ask, in order:

1. **Is it field-shaped?** — full-pane, a pure function of `(position, frame)`,
   gradient-coloured, runs forever (free-running or a seamless loop — see §B), *no subject
   or sprite* (plasma, rain, aurora, a starfield without streaks). **Inside the fresco
   repo** → a **fresco variant** (§A);
   **outside it** → the same idea as a **standalone pure field** (§B).
2. **Otherwise** — stateful (a grid / particles), a subject/sprite, *resolves* (a
   one-shot that ends), or non-field motion → **standalone** (§B).

"Field-shaped" is the *shape*; "fresco variant" is a *destination* you can only reach
from inside fresco. A field shape outside fresco is still a standalone pure `Frame`.

## §A — Fresco variant: hand off, don't re-derive

Inside the fresco repo, **invoke fresco's `new-variant` skill and follow it.** It owns
fresco's contract — the `splashPointFn` shape, the registration touchpoints, the test
guards. Do not reconstruct that checklist here; duplicating it is how the two drift
apart. Routing you there is this skill's whole job for a variant. (fresco is also a
*provider* usable from outside — see `tools.md` — but a *new* variant is authored in the
repo.)

## §B — Standalone animation: the convention

A small, framework-free convention — so the animation stays snapshot-testable and
portable, and seeds a possible future library:

```go
// pure, runs forever (a plasma, a starfield, a nebula): frame N from N alone
func Frame(w, h, tick int) string

// stateful or resolving (game of life, a typewriter, a wipe):
type Animation interface {
    Update(tick int)      // advance carried state to absolute frame `tick`
    View(w, h int) string // render current state: exactly h lines of w cells
    Done() bool           // true once a one-shot resolves; always false for a loop
}
```

**Which shape.** A pure `Frame` requires *all* of: closed-form of `tick`, runs forever,
carries no state. If it **resolves** (needs `Done()`) or **carries state**, it is an
`Animation` — even a one-shot wipe whose fill is closed-form (`filled == tick`), because
a pure `Frame` has no channel to signal completion.

**Update / View / size.** `Update(tick)` takes the **absolute** frame counter (the same
`tick` a pure `Frame` gets, not a delta), so it is idempotent for a given `tick`. An
`Animation` **owns its dimensions**: construct it with the size to simulate, advance with
`Update(tick)` (no size), and let `View(w, h)` render into the requested pane, re-seeding
or clamping if it differs. A **resolving** animation anchors its timeline and `Done()` to
the constructed size, not the `View` pane, so completion never shifts with view size.

**Free-running vs a seamless loop.** A pure field runs forever, but "forever" has two
honest shapes. A **free-running** field drives time linearly (`t := tick*speed`): it never
resolves, and nothing pins a period, so no tick is guaranteed to reproduce an earlier frame
(`examples/plasma`: its mixed sine rates share no short common period, and float rounding
means you can't count on landing back on one). A **seamless loop** instead drives every
time-varying term through one phase `θ = 2π·(tick mod period)/period`, so `Frame(w,h,0)`
and `Frame(w,h,period)` get identical inputs and are byte-identical — a provable
forever-loop (`examples/nebula`). Pick deliberately: free-running is fine for a background
that only needs to keep moving (and can be ping-ponged for a seamless *recording*); a true
loop is what a splash or idle screen that must close on itself wants, and it earns a
`TestLoopSeam` (below).

**Deliverables:** the `Frame`/`Animation` code, a `cmd/preview/` (copy the
`${CLAUDE_PLUGIN_ROOT}/scripts/preview/` directory, rename `main.go.tmpl` → `main.go`, and
wire `render()`; the verbatim build-tagged `size_*.go` give the live loop the real terminal
size so it fills the pane), and a test (below).

## 3 · Compose — combine past the default

The generic answer to any brief is one effect in flat ASCII. Compose past it:

- **Pick a fidelity tier deliberately** (`techniques.md`) — half-block raster and braille
  detail are higher objects than the `.·+*#@` ramp. Don't settle for 1×1 ASCII by reflex.
- **Design the palette**, don't pick colours functionally. Split brightness across the
  two channels — glyph density vs colour luminance (`craft.md`) — and dither gradients
  (Bayer for motion, `techniques.md`).
- **Layer.** A slow field wash (luminance) *behind* a subject/particles, with a focal
  **vignette** tying them, reads as intentional depth, not noise.
- **Reach the ecosystem** (`tools.md`, `effects.md`) — the **fresco** provider for a
  rain/tunnel/ripple/galaxy field; a **chafa/ffmpeg** source baked at build time; drive
  one effect with another. Don't rebuild what exists.
- **Keep it deterministic** so it stays testable.

## The contract, and the test

Every animation, field or standalone (full rubric: `craft.md`):

- **Shape:** exactly `h` lines of `w` visible cells (`""` for a degenerate pane).
  **Width-1 glyphs only**, indexed as `[]rune` not `string`; the width-1-safe "pixel" is
  a *space painted with a background colour*.
- **Determinism:** a pure `Frame` takes animation only through `tick`, randomness only
  through an integer coordinate hash — so it is snapshot-testable. A stateful `Animation`
  pins a fixed seed and update order so its goldens are stable.

Test: exact `h`×`w` across a spread of sizes, no panic on any `(w, h, tick)` including
tiny and zero-area panes; a pure `Frame` is byte-stable (`Frame(w,h,t) == Frame(w,h,t)`);
a **seamless loop** also asserts its seam with a `TestLoopSeam` — `Frame(w,h,0) ==
Frame(w,h,period)`, and again at an offset (`examples/nebula`), the one loop guarantee a
same-machine golden can't give you; a **rotating symmetric subject** also asserts its
`period` is *minimal*, since a seam test cannot detect a loop that closed early
(`craft.md`); stateful things get canonical goldens (a Life blinker is period-2, a glider
returns shifted by (1,1) after 4 steps; a typewriter's `Done()` flips exactly when the last
rune shows, never splitting a multibyte rune).

**Then mutate the thing under test and watch the test fail.** Break the constant, delete
the cull, flip the bit table, re-run — if it still passes, the test is decoration. This is
not optional rigour: an animation is mostly float thresholds and near-symmetries, so a
plausible-looking assertion passes for the wrong reason far more often than it does in
ordinary code (exact `!=` defeated by `sin(π) = 1.22e-16`; a whole-frame compare satisfied
by a background layer; a lit-cell count that barely moves when the occluder is removed).
A test that passes with the feature ripped out is **worse than no test** — it reads as
coverage. If a property resists a test with teeth, say so in a comment where the test
would be and check it at the beauty gate instead, as `examples/{plasma,torus}` do.

## Tune — the beauty gate

Do not ship on "tests pass": **watch it move, in colour.** Use `${CLAUDE_PLUGIN_ROOT}/scripts/`
— `preview.sh` (live), `frames` mode for a headless structure check, `ansi2png.py` to
rasterize `frames` into a PNG you can look at with no TTY (a sandbox, CI, an agent),
`record.sh` for the GIF.
Sweep each taste constant and pick by eye. The optional **`tuner`** subagent drives this.

## Red flags

- **Settling for 1×1 ASCII glyphs** when the resolution ladder would look far better — the
  flat `.·+*#@` starfield is the conventional default; climb.
- **Functional colour** (grey→white→cyan by reflex) instead of a *designed* palette.
- **Rebuilding rain / tunnel / ripple / galaxy** instead of using the fresco provider.
- **Interrogating the author** about what you could just default.
- **Bubble Tea / `tea.Model` as the animation's shape** — that's a run loop; the animation
  is a `Frame`/`Animation` (§B), wired into a TUI later.
- **Re-deriving fresco's variant checklist** in the fresco repo — hand off (§A).
- "Bounds pass, tests green" → you haven't seen the colour move. Run the beauty gate.
- Motion from a wall clock, or `math/rand`, in a pure animation → breaks determinism.
- **Claiming a seamless "loops forever"** when the field is free-running (linear `t`, never
  byte-repeats) — either drive it through `θ` and prove it with `TestLoopSeam`, or call it
  free-running and ping-pong it for the recording.
