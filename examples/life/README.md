# life — Conway's Game of Life as a symmetric kaleidoscope

![Conway's Game of Life as a glowing rose window of cathedral glass — a symmetric kaleidoscope that blooms, cools, and reblooms](../../docs/life.gif)

> **Full fidelity:** the GIF above is 256-colour; the truecolor 24-bit capture is
> [`docs/life.mp4`](../../docs/life.mp4) — what the live terminal actually shows.

<video src="../../docs/life.mp4" width="600" autoplay loop muted playsinline>
  Inline video isn't supported here —
  <a href="../../docs/life.mp4">watch or download <code>docs/life.mp4</code></a>.
</video>

A standalone splash-screen animation: Conway's Game of Life, seeded with **8-fold
symmetry** and rendered as **glowing cathedral glass**, so the simulation runs as a
breathing **kaleidoscopic mandala** that **fills the frame** — a rose window whose cells
bleed light into one another, sculpted into a circle by a radial vignette, running cobalt
and violet through ruby and gold to a white core as it blooms, cools and reblooms. It follows the skill's **§B convention** and is this
repo's first worked example of the **`Animation` interface** (`Update` / `View` /
`Done`) rather than a pure `Frame(w, h, tick)` — every other example (`plasma`,
`nebula`, `torus`, `saucer`, `bust`) is a pure `Frame`. Life earns the stateful shape
honestly: a cell's next value depends on its neighbours, not on the tick, so there is no
closed form to fold into a `Frame`.

## Why symmetry — the design that makes Life beautiful

The honest problem, and the lesson of this example: **a random-soup Game of Life is
intrinsically noise.** Every cell is independent, so the field is busy everywhere — no
focal point, no negative space, no direction. `references/craft.md` names the failure
exactly: *"a glyph in every cell is texture, not weather… motion with no origin reads as
noise."* The first two cuts of this piece — braille cells, then ember-coloured soup —
were both rejected at the beauty gate for precisely that. **No colour treatment fixes
noise; the fix is to change what is simulated.**

Conway's rules are **isotropic** — rotate or reflect a board and its next generation is
the same rotation/reflection of the original. So a seed built with a symmetry group
stays symmetric *for every later generation, for free*. Seed the soup with the full
**8-fold dihedral symmetry of the square (D4)** and the identical churn that read as
noise now reads as a **mandala**: anchored to a focal centre, ornamental, symmetric —
intentional rather than random.

## What it demonstrates

- **The `Animation` interface, finally worked.** `Update(tick)` advances a carried board
  (and a per-pixel heat field) to the absolute frame; `View(w, h)` renders it; `Done()`
  reports resolution. Life is `Update`-idempotent: it advances frame by frame and only
  ever moves forward, so a repeated or replayed tick never double-steps.
- **Symmetry that Life preserves.** The seed samples only a **fundamental wedge** (an
  eighth of the disc) and mirrors each hit into all eight images, so it is *exactly*
  symmetric — and B3/S23 keeps it that way for ever. The symmetry is never enforced in
  the step (that would corrupt the physics); it rides the rules' own equivariance. A test
  pins it: the live set equals all of its mirror and diagonal images across many ticks.
- **A disc that fills the frame, sculpted by the vignette.** The sim runs on a **square
  board larger than the pane** and `View` shows a centred window into it, so the confined
  **disc grows to cover the whole frame** — a wide pane sees a full-width band through the
  middle of the mandala. A **radial vignette** fades the corners (which fall outside the
  disc) to black, so the circle is made by the *fade*, not left as a small medallion in a
  void (`references/craft.md`: *lit shapes over dark space* — here the dark is the corners).
  The square board is also what keeps the diagonal reflection exact: it is a lattice symmetry
  only about the **integer** centre, which a square gives for any pane — a non-square board's
  diagonal would map cells off-grid — and confining growth to the interior disc keeps it there.
  Its side is rounded up to **odd**, so the board is exactly symmetric about that centre pixel:
  the cells never notice (they are confined well inside), but the halo's blur reads the whole
  board, and on an even side a tap present on one edge is missing on the other.
- **Fidelity tier — half-block.** Each cell is a `U+2580` (`▀`) glyph carrying **two
  independent truecolor pixels** (fg = top, bg = bottom), so the board is `w × 2h` — the
  portable workhorse of `references/techniques.md`, and (at the poster's 6×12px cells)
  two *square* pixels, so the disc renders as a true circle.
- **Motion read as motion.** This does not draw alive/dead directly: it carries a **heat
  field** that a birth **ignites** toward white-hot over a few frames and a death leaves to
  **decay**, so every change drags a cooling trail — the canonical *"this is moving"* signal
  (`references/craft.md`). Igniting a birth rather than flashing it on in a single frame is
  what smooths the motion: an instant white pop every few frames read as *jumpy*, but a bloom
  reads as a **breath**. In a symmetric field this turns flicker into a **pulse**: the whole
  mandala breathes.
- **It glows rather than tiles.** A Life cell is *one pixel*, so painting the heat field
  straight gives hard-edged blocks whose colour jumps from neighbour to neighbour — a
  mosaic, and the reason an earlier cut still read as speckled however good the palette
  was. The heat is therefore also blurred into a **halo** and composited back over itself
  (a screen blend, so it can only ever lift toward white): light bleeds between cells, the
  gaps light up, and a hot cell's falloff is a *continuous gradient* instead of stopping at
  the pixel border. This is what carries the smooth colour — the ramp only decides which
  hues it passes through. The blur is a sliding-window box blur run twice, O(1) per pixel,
  with the same kernel on both axes so the mandala's symmetry survives it.
- **Colour on a designed ramp of cathedral glass.** A fresh birth is a white spark, a
  living cell a steady gold, and a cooling trail runs ruby → magenta → violet → cobalt →
  black. Blended in **OKLab** rather than sRGB (a straight lerp between two saturated stops
  dips through a muddy, darker midpoint, which the wide gradients the halo paints would
  show plainly), **baked to a lookup table** at init — the conversion costs a dozen
  transcendentals a sample, far too much per pixel per frame — and **dithered** with the
  screen-locked Bayer matrix the other examples use, since a smooth wash bands at 8 bits.
  The sustain also **cools with age**, so a settled region dims toward a low violet ember
  instead of sitting as a fixed bright dot (another `craft.md` rule — fixed bright points
  read as stuck pixels). Recency of activity reads as brightness.
- **The vignette fades the ramp, not the colour.** Multiplying the finished colour toward
  black just dims it; instead the vignette scales the *ramp coordinate*, so fading toward
  the rim walks the palette down through gold, ruby, magenta and violet to cobalt. That is
  what gives the disc its concentric bands of glass — and a gentle luminance falloff on top
  still carries the corners to true black.
- **It breathes, never dies.** A symmetric soup cascades beautifully, then thins. Rather
  than let it decay to a near-empty field (or freeze into a static still-life), it
  **reblooms**: when the disc's live *population* drops below a fill floor — the honest
  signal, since cells culled at the rim keep the raw change-count high — a fresh symmetric
  soup is seeded, and it **fades in over the existing embers** rather than snapping, so the
  rebloom reads as a breath. `Done()` is always false — it is ambient, not a one-shot.
- **Determinism.** The symmetric soup is a fixed integer-coordinate hash (the `hash2`
  pattern the other examples use), never `math/rand` or a wall clock, and the rebloom is
  keyed on the **generation count** — so a given `(size, tick)` always replays to the same
  mandala. Standard **B3/S23**, synchronous, **no wraparound**.

## Run it

```sh
cd examples/life
go run ./cmd/preview                  # live, in colour (Ctrl-C to quit — cursor is restored)
go run ./cmd/preview frames 5         # dump 5 frames (structure + colour check)
go test ./...                         # shape, half-block, symmetry, blinker, glider, idempotence, determinism, golden

# headless colour gate (no TTY): rasterize frames to a PNG and look at it
go run ./cmd/preview frames 6 12 100 28 | ../../scripts/ansi2png.py --cw 6 --ch 12 > /tmp/life.png

# window in on a later moment (e.g. across a rebloom): frames N stride W H start
go run ./cmd/preview frames 16 1 100 28 376 | ../../scripts/ansi2png.py --cw 6 --ch 12 > /tmp/life_rebloom.png
```

The `frames N stride [W H [start]]` form strides the ticks from an optional start; with
the heat trails, a small stride (or `stride 1`) shows the glow cooling smoothly between
generations, which is what makes the motion read.

## How the demo GIF was made

`docs/life.gif` was produced with the plugin's own headless pipeline — no `vhs`
required:

```sh
cd examples/life
../../scripts/record-headless.sh -o ../../docs/life --fps 30 --width 480 --cw 6 --ch 12 -- \
  go run ./cmd/preview frames 120 1 80 22
```

Every frame is dumped (`stride 1`) at 30fps, because the smoothness *is* the subject. The
pane and width are smaller than the terminal actually runs at for one reason: a field of
continuous gradients compresses far worse as a GIF than the sparse one this used to be, and
the pixel count × frame count is what sets the file size. This recipe lands ~6MB, in family
with `docs/nebula.gif`; the truecolor `docs/life.mp4` from the same run is a tenth of that
and is the honest capture.

`stride 1` at **30 fps** records every frame and plays them fast, so each birth's ignition
and each trail's cooling read smoothly rather than jumping a whole generation between GIF
frames — the smoothness is *frames per second*, not a faster sim. A 100×28 pane at 6×12px
cells is exactly 600px wide, so `--width 600` means no rescale. Life is *ambient*, not a
seamless loop, so the GIF restarts from a fresh symmetric soup rather than closing on
itself — that is the piece, not a seam.
