# The craft of terminal motion

Universal heuristics for any terminal animation — a generative field, a sim, or a
one-shot. These are the general statement; fresco's `new-variant` skill states the
fresco-specific *application* of the same craft. Read this before you tune, not
after.

## The contract every frame keeps

- A frame is **exactly `h` lines of exactly `w` visible cells** — or `""` for a
  degenerate pane. Anything else corrupts the caller's layout.
- **Width-1 glyphs only.** A terminal cell is one column wide; a glyph the terminal
  draws double-width (much of CJK, many emoji, some symbols — the East-Asian-width
  trap) shoves every following cell out of place and breaks the `w` count. When in
  doubt, restrict to a known width-1 set.
- **Count and index by rune, not byte.** Use `[]rune`, never `string`, for any
  multibyte glyph set — indexing a multibyte string by byte tears glyphs apart.

## Determinism

For a **pure** animation (a function of the frame), keep it pure: animation enters
*only* through the frame/tick counter, and randomness *only* through an integer
lattice hash of the cell coordinates — never `math/rand`, never a wall clock. A
pure frame is snapshot-testable: the same `(w, h, tick)` yields the same bytes on
every machine, so a golden test can pin it exactly.

A **stateful** animation can't be pure over `tick` alone, but it can still be
deterministic given a fixed seed and a fixed update order — pin *that* instead, so
its golden test is stable.

## Making motion read as motion

- **A bright leading edge with a decaying trail** is the canonical "this is moving"
  signal — a rain streak, a ripple ring, a comet. A pattern that merely flickers in
  place reads as static grain, not motion.
- **Negative space is required.** A glyph in *every* cell is texture, not weather —
  there is nothing for the eye to watch the motion move *through*. If your reference
  effect canonically fills the screen, reinterpret it into lit shapes over dark
  space.
- **Fixed bright points over a moving field read as stuck pixels.** A twinkling
  starfield is right only over a *still* field; the moment the field travels, the
  eye tracks it and the fixed points look broken.
- **Coherent global motion** — a whole texture translating or receding one way — is
  what makes the eye read *self-motion* rather than shimmer.

## Two brightness channels

Brightness can ride two independent channels, and how you split it is most of the
look:

- **glyph density** — a heavier glyph for a brighter cell (`·` → `o` → `O` → `@`).
  Carries *texture*; but a smoothly fading region degenerates into a scatter of
  isolated dots.
- **colour luminance** — a lighter colour for a brighter cell. Carries a *smooth
  gradient* without breaking into dots.

Gradients want luminance; stipple and texture want density. Most fields with a
smooth falloff keep the bulk of their brightness on luminance, so a dim region
stays a dim wash rather than confetti.

## Composition

- **An edge vignette** — fade the outermost rows and columns to nothing — so the
  animation never meets a hard rectangular border. A hard edge reads as a *box*;
  a vignette reads as a window onto something larger.
- **Anchor to a focal point.** Motion that emanates from, orbits, or recedes toward
  a point reads as intentional; motion with no origin reads as noise.

## Tune by looking, not by arithmetic

The single most repeated lesson. You cannot *compute* the right sharpness, speed,
or brightness split — you **render a sweep of candidate values and pick by eye**,
in colour, watching it move. Reasoning a constant out, or copying one from another
animation by analogy, is not tuning. Build the preview first; decide every taste
constant against what you actually see. A constant with no live knob (a package
`const`) is worth lifting to a temporary `var` or env read while you sweep it, then
folding back once chosen.

## The tuning loop

- **Inner loop (fast):** render N frames to the terminal, or to text/PNG, and check
  *structure* — the `h×w` contract, glyph widths, enough negative space, that
  consecutive frames actually differ.
- **Outer loop (the beauty gate):** record a short GIF and **watch the motion, in
  colour**. Does it read as motion? Enough dark space? No stuck pixels or width
  bugs? Legible on a dark background? Tune, and repeat until it reads right. The
  `scripts/` harness runs both loops.
