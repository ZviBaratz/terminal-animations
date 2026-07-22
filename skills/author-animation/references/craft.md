# The craft of terminal motion

Universal heuristics for any terminal animation — a generative field, a sim, or a
one-shot. These are the general statement of *why* motion reads; the concrete
mechanics it leans on — the sub-cell resolution ladder, colour depth, dithering — are
in `techniques.md`. fresco's `new-variant` skill states the fresco-specific
*application* of the same craft. Read this before you tune, not after.

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
lattice hash of the cell coordinates — never `math/rand`, never a wall clock. A pure
frame is snapshot-testable — but be precise about *what* is portable: `math.Cos/Sin`
and float64 rounding are not bit-identical across arch/OS, so a golden pins the exact
bytes only on the machine that generated it. The portable guarantees are the **shape**,
the **no-panic**, and — for a loop — the **seam**; assert those, and keep the golden
same-machine (regenerate with `-update`).

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

## Making a subject move in 3D

The heuristics above are about a *field*. A **subject** — a bust, a sprite, a logo, any
object with a silhouette — has a harder default to escape: the generic "motion" for a
subject is a **translate**. A pan, a bob, a slow zoom, or the two-quarter-phase-sinusoid
ellipse (`x = A·sinθ, y = B·cosθ`) that reads as drifting in a circle. Add a global
brightness "breathe" and you have described a photograph with a Ken Burns move. It is flat
because it *is* flat: one plane sliding in its own frame, no new information revealed. This
is the RED baseline. Climb it with any of:

- **A pseudo-3D turn.** A gentle perspective keystone about the vertical axis (yaw = A·sinθ),
  applied to the subject, reveals a little of each side and reads as *rotating*, not sliding
  — the honest stand-in for a spin when a single still can't show the back. (Baked with
  `perspective`/`Image.transform`; see `tools.md` §Baking. `examples/bust` does exactly this.)
- **Parallax.** Separate the scene into depth planes and move them at *different* rates —
  the subject one amount, the mist/backdrop another. Differential motion is the strongest
  monocular depth cue there is; it turns a slide into a space.
- **A relighting sweep.** Hold the geometry still and move the *light* — a warm key that
  orbits the subject, a rim that rakes the silhouette, a specular that travels. A moving
  light on a static object reads as far more alive than the object translating under a fixed
  one, and it is "dramatic lighting" almost by definition. (`atmosphere-kit.md` has the term.)
- **Atmospheric depth.** Haze that thickens with distance, mist drifting across and pooling
  at the base, dust catching the light — depth cues that also *move*, at their own rate, so
  the subject sits *in* something rather than on top of it.

Compose two or three of these — a turn under a sweeping light in drifting mist — and a flat
cut-out becomes a lit object in space. The machinery is `atmosphere-kit.md`; the motion
verb you named on the Vision Card (SKILL §1) is what tells you which of these you owe.

**Or don't move the geometry at all — move the color.** There is a second escape from the
translate, and in a terminal it is often the stronger one: reduce the subject to a *graphic*
and animate its palette. Posterize its luminance into a few flat bands and recolor them,
cycling the colorways so a wave of recoloring sweeps across it — a Warhol silkscreen. This
works *because* of the medium: a terminal is bad at subtle gradients and spectacular at bold
flat color, so a subject whose realism is fighting the resolution gets *stronger* the moment
you stop rendering it accurately. The machinery is `palette-cycle-kit.md`; `examples/bust` is
the worked piece. Reach for it when the Vision Card's appeal is graphic or iconic; reach for
the 3D and lighting moves above when the appeal is form and light.

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

## A per-cell field is a mosaic until you diffuse it

The channel rule above is about one cell. This one is about *neighbours*, and it is
the thing that separates a field that glows from one that tiles.

If the signal is **one value per cell** — a simulation grid, a particle count, a
Game-of-Life board, anything sampled rather than swept — then adjacent cells carry
independent values and the colour **jumps** at every cell boundary. The result reads
as a mosaic of hard-edged blocks no matter how good the palette is, and no amount of
palette work fixes it, because the discontinuity is in the *field*, not the colours.

**The fix is spatial: blur the field into a halo and composite it back over itself.**
Light then bleeds between cells, the gaps light up, and a bright cell's falloff
becomes a continuous gradient instead of stopping at its border. It is the single
highest ratio of beauty gained to code changed in this whole file.

- **Composite with a screen blend** — `out = 1 - (1-field)·(1-gain·halo)` — not a
  clamped sum. A sum blows the core to a flat white disc as soon as the gain is
  interesting; a screen blend can only lift toward white, so raising the gain deepens
  the glow instead of destroying the structure.
- **Blur the scalar field, before the palette.** It is an emission field; blurring
  the finished RGB instead smears hues together into mud.
- **Two passes of a box blur ≈ a Gaussian**, and a **sliding-window** box blur is O(1)
  per pixel whatever the radius — carry a running sum and step it. The obvious
  re-add-every-tap version is what makes people believe a blur is too expensive for a
  frame budget; it isn't.
- **If the field is symmetric, make the grid odd-sided.** A blur reads the whole grid,
  and on an even side the mirror of column 0 falls off the far edge, so edge taps differ
  between the two halves and the light tilts — even while the underlying cells stay
  perfectly symmetric. Cells confined to the interior never notice; the halo does.
- **Guard the negative space.** Diffusion fills the gaps that were carrying the "lit
  shapes over dark space" rule above. Sweep the gain and stop at the value where the
  darks are still dark.

## Composition

- **An edge vignette** — fade the outermost rows and columns to nothing — so the
  animation never meets a hard rectangular border. A hard edge reads as a *box*;
  a vignette reads as a window onto something larger.
- **Fade the ramp coordinate, not the colour.** The obvious vignette multiplies the
  finished RGB toward black, which only *dims* — the edge is the same hue, darker.
  Scale the palette's **input** instead (`palette(value · vignette)`) and the fade
  walks down the ramp through its cooler end, so the falloff is a colour *arc*: the
  piece gains concentric colour zones for nothing, and the ramp's cold stops — usually
  the least-visited part of a palette — finally get shown. Keep a gentle luminance
  multiply on top if you still need a true black at the very edge.
- **Anchor to a focal point.** Motion that emanates from, orbits, or recedes toward
  a point reads as intentional; motion with no origin reads as noise.
- **Choose the subject to the medium.** A cell grid is coarse, low-colour-depth, and
  square — it renders *bold contrast, saturated colour, crisp shapes, and directional
  motion* beautifully, and *smooth, subtle, low-contrast gradients* as mud. So pick a
  subject the medium flatters. A smooth astronomical glow (a galaxy) fights the grid; a
  directional field (aurora, rain, a tunnel) and a crisp bold sprite embrace it. When a
  composition reads as a muddy smudge, the fix is usually not another constant-sweep — it
  is a subject or field the grid can actually draw. (`examples/saucer` began as a fresco
  *galaxy* and had to be rebuilt around a fresco *aurora* plus a cartoon sprite for exactly
  this reason.)

## The forever-loop seam

"Loops forever" means two different things, and the difference is testable. A
**free-running** field advances time linearly (`t := tick*speed`); it never stops, but
nothing constrains it to return to an exact earlier frame: its mixed rates share no short
common period, and float rounding makes any long one unreliable. So it does not seamlessly
loop — though it can still be *ping-ponged* (played forward then reversed) into a seamless
recording. A **true loop** drives every time-varying term through a single phase
`θ = 2π·(tick mod period)/period`, so tick 0 and tick `period` feed identical inputs and
render byte-identical frames — a seam with no jump. Prefer the θ form whenever the piece
must close on itself (a splash, an idle screen), and pin it with a seam test
(`Frame(…,0) == Frame(…,period)`): it is the one loop guarantee a same-machine golden
can't give you.

### The seam closing does not mean `period` is the *true* period

A seam test proves the loop closes. It cannot tell you the loop closed **early** — that
your advertised `period` is two or three times the real one, so the "24-second loop" is a
12-second one played twice and the recording wastes half its frames on a repeat.

This bites hard whenever a **symmetric subject** is rotated by integer harmonics. Every
π-rotation about a coordinate axis is a symmetry of a torus, a sphere, a cube, a regular
polyhedron — so at `tick = period/2` the accumulated rotation lands back on a symmetry of
the object and the frame is *identical*. Verified for a torus at harmonic ratios 1:2, 1:3,
2:3 and 3:5: all of them secretly repeat at `period/2`. A fixed **oblique pre-tilt** (an
angle that is not a multiple of π/2, applied before the animated rotation) breaks the
degeneracy; see `examples/torus`.

So add a **minimal-period** test alongside the seam test — assert `Frame(…,0)` differs
from `Frame(…, period/2)`, `period/3`, `period/4`. Two subtleties decide whether it has
any teeth at all:

- **Compare the glyph grid with SGR stripped, not the whole frame.** If any *other* layer
  varies with θ — a background wash, a palette drift — a whole-frame comparison differs on
  that layer alone and passes no matter what the subject does.
- **Assert a large fraction of cells differ, not mere inequality.** `sin(π)` is `1.22e-16`,
  not `0`, so a perfectly degenerate half-period render still differs in a handful of dots.
  Measured on the torus: **3.8%** of lit cells differ in the degenerate case vs **96%** in
  the healthy one — so a 25% floor separates them with room to spare, and exact `!=` does
  not separate them at all.

## Tune by looking, not by arithmetic

The single most repeated lesson. You cannot *compute* the right sharpness, speed,
or brightness split — you **render a sweep of candidate values and pick by eye**,
in colour, watching it move. Reasoning a constant out, or copying one from another
animation by analogy, is not tuning. Build the preview first; decide every taste
constant against what you actually see. A constant with no live knob (a package
`const`) is worth lifting to a temporary `var` or env read while you sweep it, then
folding back once chosen.

**But measure when looking isn't finding it.** Judging by eye decides *which* value
is better. It is bad at answering "this looks flat and I can't say why" — you can
stare at a frame for a long time without noticing that three quarters of it is nearly
black, or that a seven-stop palette is only ever rendering two of its stops. So when a
piece is disappointing and the cause won't name itself, take one numeric read of a
frame (`ansi2png.py --stats`: luminance histogram, dark fraction, hue spread of the
lit pixels) before sweeping anything.

The diagnosis to look for: **most pixels in the bottom luminance bin, with the lit
ones piled into one or two hue buckets.** That says the designed ramp is not being
*reached* — the field feeding the palette never visits the palette's middle — so the
fix is in the field (longer trails, spatial diffusion, a different value mapping), and
every hour spent re-picking colour stops is wasted. Sweeping constants cannot find
this, because no value of any constant is wrong; the signal is.

## The tuning loop

- **Inner loop (fast):** render N frames to the terminal, or to text/PNG, and check
  *structure* — the `h×w` contract, glyph widths, enough negative space, that
  consecutive frames actually differ.
- **Outer loop (the beauty gate):** record a short GIF and **watch the motion, in
  colour**. Two passes. First, *craft*: does it read as motion? Enough dark space? No
  stuck pixels or width bugs? Legible on a dark background? Then, *vision*: hold it up to
  the Vision Card (SKILL §1) slot by slot — is the motion the **verb** you named (does it
  turn, or slide?), is the **light** where you said and moving, is the **atmosphere** there,
  did **the one special idea** land? The craft pass rejects broken; the vision pass rejects
  *merely competent*, and it is allowed to. Tune, and repeat until it reads right. The
  `${CLAUDE_PLUGIN_ROOT}/scripts/` harness runs both loops.
- **When the author is the judge, show candidates — don't guess.** A revision request
  like *"nicer colours"* or *"make it prettier"* has no single right answer, and
  guessing spends a whole build to find out. Prototype **in a throwaway copy** (so the
  real tree stays clean while every knob is an env var), render **two to four labelled
  directions** as one comparison image plus a short GIF of the front-runner, and let
  the author pick. One round of that ended a piece that had already been rejected three
  times on taste. It also converts vague dissatisfaction into a decision you can build
  against, which is worth more than the render time.
