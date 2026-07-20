# torus — a spinning 3D wireframe torus in braille

![a tumbling wireframe torus — braille truecolor, seamless loop](../../docs/torus.gif)

> **Full fidelity:** the GIF above is 256-colour; the truecolor 24-bit capture is
> [`docs/torus.mp4`](../../docs/torus.mp4) — what the live terminal actually shows.

<video src="../../docs/torus.mp4" width="600" autoplay loop muted playsinline>
  Inline video isn't supported here —
  <a href="../../docs/torus.mp4">watch or download <code>docs/torus.mp4</code></a>.
</video>

A standalone splash-screen animation: a pure, deterministic **braille** wireframe torus
that tumbles about two axes, removes its own hidden lines, and loops forever with no
seam. It follows the skill's §B convention (a pure `Frame(w, h, tick)`) and is this
repo's worked example of the **top rung of the resolution ladder**.

## What it demonstrates

- **Fidelity tier — braille.** Every cell is a `U+2800`-block glyph carrying a **2×4
  grid of individually addressable dots**, so the visible grid is `2w × 4h` — the finest
  *monochrome* rung from `references/techniques.md`. That reference argues the rungs
  above half-block buy sharper hard **edges** rather than smoother colour, which is
  exactly why a **wireframe** belongs up here and a smooth colour field does not. A
  terminal cell is roughly 1×2, so a braille dot is very nearly **square** — both axes
  are scaled by the same factor and the torus stays circular.
- **The two brightness channels, split cleanly.** A braille cell is *monochrome* —
  eight dots share one foreground colour — so the **dot mask carries pure geometry** and
  **colour carries all the brightness** (`references/craft.md`). Dimming a wire by
  dropping dots would shatter thin lines into noise, so it is never done.
- **Hidden-line removal.** The opaque surface is rasterized into a **per-dot depth
  buffer** that is never drawn; wires survive only where they are in front of it, backed
  by an analytic back-face cull. Without it a wireframe torus is famously ambiguous (the
  Necker-cube effect) and its spin direction visually flips. This is `donut.c`'s
  per-*cell* z-buffer (`references/effects.md`) promoted to per-*dot* by the tier.
- **A designed iridescent palette.** Deep cyan-blue on the receding side → indigo →
  violet → magenta → a hot pink-white near limb, blended with a Lambert `N·L` term on
  the analytic torus normal. **Hue moves with depth**, not just luminance, so the tumble
  reads even where dot density is flat.
- **Composition.** A dim backdrop wash painted as the cell **background** — the one way
  to layer a smooth colour field under a monochrome braille glyph in the same cell —
  times an **edge vignette**, so the splash reads as a window onto something larger.
  The wash is a smooth dim gradient, so it gets motion-stable screen-locked **Bayer**
  dithering; the wires deliberately do not (dithering line art only makes it dashed).
- **A truly seamless forever-loop.** Every time-varying term rides one phase
  `θ = 2π·(tick mod P)/P` at an **integer** harmonic, so `Frame(w,h,0)` and
  `Frame(w,h,P)` are byte-identical — pinned by `TestLoopSeam`.
- **The loop length scales with the pane** (`Period(w, h)`, 720 at the 100×28
  reference). The torus turns a fixed *angle* per tick, but the eye sees *dots crossing
  a grid*, and a bigger pane scales the object up so the same angle carries every dot
  further. Past ~1 dot of travel per frame the whole pattern rewrites each frame and it
  visibly flickers — with no fix available, because a braille cell is monochrome and a
  single dot cannot be dimmed. Measured share of lit dots changing between consecutive
  frames, at a fixed 24 s loop: **49% at 100×28 but 95% at 210×60**. Stretching the loop
  with the pane holds that near 50% everywhere (54% at 210×60, which runs 51 s).
- **Determinism.** `Frame(w, h, tick)` is pure — no wall clock, no `math/rand`, no
  package-level state. The depth and mask buffers are per-call locals, so it is also
  safe to call concurrently.

## The trap this animation exists to document

A torus tumbled by **integer harmonics about two coordinate axes secretly repeats at
`period/2`.** At that tick the accumulated rotation is a product of π-rotations about
coordinate axes, and *every one of those is a symmetry of the torus* — so the frame
comes back identical and the "24-second loop" is really a 12-second one played twice.
Verified numerically for harmonics 1:2, 1:3, 2:3 and 3:5.

The fixed oblique pre-tilt **`tiltY`** breaks the degeneracy. It is load-bearing, not
decoration, and `TestPeriodIsMinimal` pins it.

That test needs two subtleties to have any teeth, both learned the hard way:

1. It compares the **dot grid with SGR stripped**, not the raw frame. The backdrop wash
   also varies with θ, so a whole-frame comparison differs on the *background* alone and
   passes no matter what the torus does.
2. It asserts a large **fraction** of cells differ, not mere inequality. `sin(π)` is
   `1.22e-16`, not `0`, so even a perfectly degenerate half-period render differs in a
   handful of dots. Measured: the degenerate case differs in **3.8%** of lit cells, the
   real one in **96%** — so a 25% floor separates them with room to spare.

## Run it

```sh
cd examples/torus
go run ./cmd/preview            # live, in colour (Ctrl-C to quit — cursor is restored)
go run ./cmd/preview frames 5   # dump 5 frames (structure + colour check)
go test ./...                   # shape, no-panic, determinism, seam, true period, fit, golden

# headless colour gate (no TTY needed): rasterize frames to a PNG and look at it
go run ./cmd/preview frames 6 120 100 28 | ../../scripts/ansi2png.py --cw 6 --ch 12 > /tmp/torus.png
```

## How the demo GIF was made

`docs/torus.gif` was produced with the plugin's own headless pipeline — no `vhs`
required:

```sh
cd examples/torus
../../scripts/record-headless.sh -o ../../docs/torus --fps 20 --width 600 --cw 6 --ch 12 -- \
  go run ./cmd/preview frames 120 6 100 28
```

`120 × 6 = 720 = Period(100, 28)`, so the dump spans exactly one loop and the GIF closes
with no ping-pong. **The pane size and the tick count are linked** — `Period` scales with
the pane, so recording a different size means recomputing the dump to match, or the GIF
will not close.

**Note the small pane and the matching `--width`.** Braille is the one tier where the
recording size really matters: the wireframe *is* the dot pattern, so any downscale
destroys it. A 100×28 pane at 6×12px cells is exactly 600px wide, so `--width 600`
means **no rescale at all**. Recording at nebula's 220×56 and letting ffmpeg scale to
640 would shrink each dot below a pixel and the torus would arrive as a grey haze.

This animation is also why `ansi2png.py` now understands braille at all — see the
CHANGELOG. Before that it collapsed every braille cell to a solid foreground block, so
the headless gate (**and this GIF, which is built through it**) showed a filled blob
instead of a wireframe.

## What the headless gate cannot see

Two things about this animation are invisible to `ansi2png.py` and were only caught by
running it in a real terminal. Both are worth knowing before you trust a PNG:

- **Braille dots render as separated points, not filled rectangles.** `ansi2png.py`
  fills each dot's sub-rectangle, so a wire reads as a solid line in the PNG. A real
  terminal font draws each dot as a small mark with space around it, so the same wire
  reads as a *dotted* line. **The PNG gate flatters braille line art**; judge solidity
  live, and read *Fonts and line height* below first — "live" is only meaningful once the
  terminal is actually rendering braille.

  Do **not** use sample contiguity to argue the wires are solid. An earlier version of
  this note claimed "consecutive samples land within one dot 99.7% of the time," which is
  Chebyshev distance ≤ 1 and therefore counts a *diagonal* step — two dots touching only
  at a corner, which no font draws as connected — as contiguous. The metric cannot see
  the defect it was being used to rule out. Measured properly, along each wire's run of
  distinct dots: **88.3% orthogonal, 11.7% diagonal, 0.00% gaps** at 100×28 (84.2 / 15.5 /
  0.29 at 210×60). Genuine breaks really are absent; the residual staircase is the
  diagonal fraction, and it is small enough that 4-connected rasterization was measured
  and rejected as not worth the extra ink.
- **Temporal aliasing does not exist in a still.** A PNG has no next frame, so nothing
  about flicker, shimmer or dot-popping can appear in it. That is how a fixed angular
  rate shipped despite flickering badly on a large pane (see `Period`). For any animation
  on this rung, the live terminal is the only gate for motion.

## Fonts and line height — check these before judging anything

Braille is the one tier where **your terminal font decides how the output looks**, and
two failure modes are common enough that you should rule them out before concluding
anything about the animation itself. Both were hit on the development machine, and both
were mistaken for rendering bugs.

**1. Your monospace font may not contain braille at all.** U+2800–28FF is widely
*supported* but not widely *included*. MesloLGS NF, JetBrains Mono and DejaVu Sans **Mono**
all lack it. When the primary font has no glyph, fontconfig silently falls back to
whatever does — typically **proportional** DejaVu Sans — and you get dots at the wrong
pitch inside a monospace cell. Check before trusting your eyes:

```sh
fc-list ':charset=2800' family | sort -u        # who has braille at all
fc-match "YourFont:charset=2800"                # what YOU actually get
```

If the second command names a different family than the first argument, you are looking
at a fallback. Note `fc-list ':charset=2800:spacing=100'` is **not** a reliable filter —
Iosevka carries braille but is tagged `spacing=90`, so that query hides it.

Measured dot geometry, at em=256 (gap is edge-to-edge, in dot widths):

| font | h-gap | v-gap | ink fill | `⣿` vs `M` advance |
|---|---|---|---|---|
| DejaVu Sans (a fallback, proportional) | 1.03 | 0.92 | 25.7% | 187.5 vs 220.9 — mismatched |
| Iosevka | 0.78 | 0.72 | 27.4% | 128.0 vs 128.0 |
| Cascadia Mono | **0.70** | **0.49** | **31.9%** | 150.0 vs 150.0 |

**2. Even with a good font, the line box is usually taller than four dot rows.** The dot
pitch inside a cell is set by the glyph; the cell height is set by the font's ascent plus
descent. They do not match, and the leftover lands as a blank band **every 4 dot rows**,
locked to the screen. A rotating object drifting through those fixed bands has dots wink
out and back, which reads as jitter — it is easy to blame on the animation. Seam versus
intra-cell gap, same fonts:

| font | intra-cell gap | inter-line seam | ratio |
|---|---|---|---|
| DejaVu Sans | 34 | 49 | 1.44× |
| Iosevka | 26 | 97 | 3.73× |
| Cascadia Mono | 18 | 97 | **5.39×** |

Note the ordering: **tighter dots make the banding worse**, because shrinking the gap
inside the cell does nothing to the cell height. The best braille font is the worst
offender here.

The fix is to shrink the line box to exactly four dot pitches. In Alacritty that is
`font.offset.y`, and you can test it per-launch without touching your config:

```sh
alacritty -o 'font.normal.family="Cascadia Mono"' -o 'font.size=18' -o 'font.offset.y=-9' \
  -e sh -c 'cd examples/torus && go run ./cmd/preview'
```

The correction scales with font size, so `size` and `offset.y` move together; nudge by ±1
until the bands snap out. Prefer a larger size regardless — below ~16px the dot pitch is
2–3px and the lattice quantizes badly whatever you do.

## Tuning notes

Every constant at the top of `torus.go` was swept against the `ansi2png.py` filmstrip
and picked **by eye**. What the sweeps actually rejected:

- **`baseRingsU` / `baseRingsV` (12 / 22, at the 100×28 reference)** — the first and most
  important sweep. `16/28` was the initial guess and collapses into an unreadable solid
  mesh below ~60 columns; `10/18` reads as loose disconnected bands and the object stops
  feeling solid. 12/22 is the pair that stays legible at 44×13 and still looks like a
  machined object at 100×28.

  These **scale with the pane** (`paneScale`, shared with `Period`). Held fixed, a large
  terminal spreads the same 12/22 rings over twice as many dots and the mesh thins into a
  sparse lattice — mean lit neighbours per lit dot falls from **4.02 at 100×28 to 3.19 at
  210×60**, and an isolated dot popping between frames reads as a blink rather than as
  motion. Scaling restores 4.06 and drops churn 54.5% → 47.8%. The factor is capped at
  `maxRingScale` because rings cost frame time: uncapped, 400×110 lands at ~27 ms against
  a 33 ms budget, and a stutter is just a slower flicker.
- **`shadeGamma` (1.3)** — shapes the mids down into the indigo so the hot pink stays a
  rare near-limb highlight instead of the body colour. It wants a light touch. This note
  previously justified **2.2** by claiming the raw shade "piled up at 0.6–0.9" and spent
  the whole palette in the magenta band; **that does not reproduce.** Measured over 16
  frames at matched phase at 100×28, the raw shade is already well spread — p10 0.17,
  median 0.52, p90 0.84. Applying 2.2 to that crushed the median to **0.24**, left only
  **10.8%** of lit cells reaching violet and **none at all** reaching the pink-white top
  of the ramp, so the torus rendered as a near-uniform dark indigo. That mattered more
  than it sounds: braille dots are *separated marks*, and dim low-contrast dots stop
  grouping into lines at all — the "wires look dotty" complaint was substantially a
  contrast problem, not a geometry one. At 1.3 the median is 0.43 and 23.2% reach violet.
- **`depthNear` / `depthFar`** — same root cause. Because the back-face cull removes
  everything facing away, the visible surface only spans the *near* part of the object;
  mapping the palette across the full `[-maxR, +maxR]` wastes half the ramp.
- **`fitFrac` (0.86)** and the exact projected-radius bound. The first fit divided by
  the *nearest-point* magnification `persp/(persp−maxR)`, which is far too conservative
  and left the torus filling only ~50% of the pane. The exact worst case over every
  attitude is `maxR·persp/√(persp²−maxR²)` — confirmed against a numeric sweep — which
  recovers that 1.66× and still guarantees `TestFitsPane`.
- **`depthBias` (0.40)** — a wire lies exactly *on* the occluder, so its depth error
  grows with surface obliquity: the classic shadow-map acne shape. The original `0.18`
  was rejecting on-surface samples in runs of 1–3, punching holes that migrate as the
  object turns. Measured over the whole loop at 100×28, those dropouts fall **217 → 17**
  going `0.18 → 0.40`, and flatten after. An earlier note here claimed `0.25` lets the
  far side fringe through at the edge-on attitude; that is **not reproducible** — the far
  side sits ~`2·smallR` back, and across all 720 ticks no drawn sample exceeds a depth
  delta of 0.5 until the bias passes 0.7. There is no far-side population to leak at
  these values, and hidden-line removal is untouched.

Measured cost, since a two-pass rasterizer invites suspicion: **9.8 ms/frame at 220×56**
and **2.5 ms at 100×28**, comfortably inside the 33 ms budget of the preview's 30 fps —
but only because the occluder's `cos`/`sin` are hoisted into per-frame tables, so the
inner loop is multiply-add with no transcendentals.

There is deliberately **no `TestHiddenLineRemoval`**; `torus_test.go` explains why at
length. The short version: summed over the whole loop at 100×28 the occlusion pipeline
only changes the lit-cell count by ~19%, and its two stages are redundant enough that
disabling either alone moves it ~4% (and on some frames not at all), so any threshold
tight enough to catch a regression would be flaky. Occlusion is a visual property,
checked at the beauty gate.
