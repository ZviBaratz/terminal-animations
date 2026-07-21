# bust — a classical bust under a slow, hypnotic color wash

![a classical marble bust silkscreened under a slow diagonal color wash, an invisible 3×3 grid tinting it into coordinated zones as the hue drifts around the wheel — half-block truecolor, seamless loop](../../docs/bust.gif)

> **Full fidelity:** the GIF above is 256-colour; the truecolor 24-bit capture is
> [`docs/bust.mp4`](../../docs/bust.mp4) — closer to what the live terminal shows.

<video src="../../docs/bust.mp4" width="560" autoplay loop muted playsinline>
  Inline video isn't supported here —
  <a href="../../docs/bust.mp4">watch or download <code>docs/bust.mp4</code></a>.
</video>

A looping terminal animation that treats a marble bust as a **silkscreen source**: one bust
fills the pane, an invisible 3×3 grid divides it into nine color zones, and a gentle diagonal
tint gradient drifts slowly around the color wheel forever. It is the author-animation skill's
"**silkscreen the subject, cycle the palette**" pattern (`references/palette-cycle-kit.md`,
`tools.md` §Baking): the bust's luminance is matted and baked once; all the color, and all the
motion, is a pure function of `tick` in Go. Nothing is installed to run it.

## Why this, and how it got here

The first cuts of this example chased photographic realism — a matted still panned in an
ellipse, then a pseudo-3D turn under a moving light — and both fell flat. The lesson is about
the medium: **a terminal is bad at subtle and spectacular at bold.** At half-block resolution
the marble's gentle gradients collapse into banded hair and a muddy color cast. Flat,
high-contrast, saturated color is exactly what truecolor half-blocks do best. So we stop
rendering the marble accurately and *screenprint* it — posterize its luminance into four flat
tones and recolor them, the way Warhol silkscreened Marilyn.

A first pop-art cut tiled **nine clashing busts** in a hard grid. It read, but it was a strobe,
not a spell. This refinement keeps the silkscreen and makes it **hypnotic**: one bust instead of
nine, an invisible color-zone overlay instead of framed panels, and a slow drift through
**analogous** (neighboring) hues instead of clashing pop. All the aggression is gone; all the
motion is a slow, coordinated breath of color.

## Vision Card

- **Subject** — one classical marble bust as a silkscreen source: a single luminance + alpha
  matte, fit once across the whole pane, posterized to four flat tones. No 3D, no relighting.
- **Grid** — a 3×3 **color-zone overlay with no visible seams**. It never moves and never cuts
  the bust; it only chooses which zone's colorway recolors each pixel, so the grid is felt
  purely through color — a subtle segmentation in the field, a gentle gradient across the head.
- **Motion verb** — *breathing*. A gentle diagonal recoloring gradient drifts across the bust
  and, over the loop, rotates through every hue. The geometry never moves.
- **Light** — none. Flat color fields; the marble's own tonal map drives the posterization.
- **Palette** — analogous drift: nine colorways whose base hue steps evenly around the wheel,
  each a small dark→bright ramp of one hue neighborhood at moderate saturation. Any single frame
  sits in a narrow, coordinated hue band; adjacent zones are always close — rich, never clashing.
- **The one special idea** — a single classical bust dissolving through slow, coordinated color
  under an invisible silkscreen grid, a diagonal tint gradient rotating around the wheel forever.

## What it demonstrates

- **Silkscreen the subject, cycle the palette.** `clean.py` bakes *only* the bust's luminance
  and silhouette (`bust_lum.png`). `bust.go` posterizes that luminance into four bands and maps
  each band — and the silhouette's background — to a colorway ink. All the drama is color,
  computed live; the subject is a still. This is the fix for this example's earlier cuts, which
  were the anti-pattern for the medium: a photograph given a camera move, subtle where the
  terminal wanted bold.
- **One subject, a color-zone overlay — not a tiled grid.** The bust is fit *once* across the
  whole pane, so it is one continuous, recognizable head. The 3×3 grid is only a selector: each
  pixel's zone decides which colorway recolors it. Nine color treatments over one image, no
  seams — the grid reads through the color alone.
- **Motion is color, not geometry.** Each zone crossfades through the nine colorways, indexed by
  a continuous phase **plus a small diagonal offset** `rippleSpread·(gx+gy)`, so the recoloring
  reads as a gentle wave travelling diagonally across the bust while the whole thing rotates
  slowly around the hue wheel.
- **Coordinated, not clashing — the mellow lever.** `rippleSpread` (< 1) keeps neighboring zones
  close in hue, so the nine zones read as a coordinated gradient rather than nine fighting
  panels; the long `period` makes the drift a slow breath. Together they are the "hypnotic, not
  seizure-inducing" knobs — spatial contrast and temporal speed, tuned by eye.
- **Hue-aware crossfades.** Blending two colorways in RGB passes through gray mud. So the
  crossfade interpolates in HSV along the shorter hue arc, keeping saturation and value high —
  the transition drifts through clean, vivid hues instead of desaturating at the midpoint.
- **A matte that keeps the whole subject, then a clean luminance ramp.** The bust is white
  marble on a white field — the hard case. `clean.py` floods only near-pure-white (so lit marble
  that touches the frame isn't eaten) and keeps every component above a size floor (so a
  highlight can't split off the shoulders). It then contrast-stretches the marble's own narrow
  tonal range to fill the ramp, so the face reads once posterized, and de-speckles the faint
  stock watermark so it can't surface as a band edge. `TestAsset` guards the matte didn't
  collapse; `TestColorField` guards the palette is alive and coloured.
- **A composition robust to any pane.** The head is *contained* in the pane (letterboxed), so
  the whole face reads whether the pane is wide or tall; the flat background fills the margins as
  a drifting color field.
- **Fidelity tier — half blocks.** Every cell is a `▀`: foreground paints the upper pixel,
  background the lower, so the visible grid is `w × 2h` independent 24-bit pixels.
- **A seamless forever-loop.** Every zone's colorway index advances by exactly `len(colorways)`
  over one `period`, so `Frame(w,h,0)` and `Frame(w,h,period)` are **byte-identical** — pinned by
  `TestLoopSeam`.
- **Determinism.** `Frame(w, h, tick)` is pure — no wall clock, no `math/rand`. Tests pin the
  `h×w` contract, no-panic on any `(w, h, tick)`, byte-stability, the seam, that consecutive
  frames move, the decoded-asset integrity, the color field, and a golden.

## Run it

```sh
cd examples/bust
go run ./cmd/preview                    # live, in colour (Ctrl-C to quit — cursor is restored)
go run ./cmd/preview frames 5           # dump 5 frames (structure + colour check)
go test ./...                           # shape, no-panic, determinism, loop-seam, motion, golden

# headless colour gate (no TTY needed): rasterize a filmstrip to a PNG and look at it
go run ./cmd/preview frames 8 90 150 46 | ../../scripts/ansi2png.py --cw 4 --ch 4 > /tmp/bust.png
```

## Re-baking the asset

`bust_lum.png` is committed, so nothing below is needed to *run* the animation — only to
regenerate the luminance matte from a different source.

```sh
cd examples/bust
python3 clean.py ~/Downloads/bust.png bust_lum.png   # matte → luminance+alpha asset
go test ./... -run TestGolden -update                 # asset changed ⇒ refresh the golden
```

`clean.py` needs `python3` + Pillow on `PATH` (author-time only). The **palette** and **motion**
constants (the colorways, `period`, `rippleSpread`, `fill`, `vPlace`, the posterization) live in
`bust.go` and are documented in `references/palette-cycle-kit.md` — all swept **by eye** against
the `ansi2png` filmstrip, per the plugin's "tune by looking, not arithmetic" loop. Posterization
is unforgiving of a stock watermark, so `clean.py` de-speckles it and the emitted asset is
verified watermark-free before committing; the watermarked source is **never copied into the
repo**, only the clean `bust_lum.png` ships.

## How the demo media was made

The plugin's own headless recorder, `scripts/record-headless.sh` — no `vhs` required. The loop
is long (a full hue rotation is `period` = 720 ticks), so the demo strides across the whole loop
rather than showing every tick:

```sh
# a lean README GIF (one full hue rotation) and the truecolor MP4
../../scripts/record-headless.sh -o ../../docs/bust --no-mp4 --fps 24 --width 440 -- \
  go run ./cmd/preview frames 72 10 150 46
../../scripts/record-headless.sh -o ../../docs/bust --no-gif --fps 30 --width 600 -- \
  go run ./cmd/preview frames 120 6 150 46
```

Because the animation is a seamless loop, the GIF loops with no ping-pong. Flat color fields
compress far better as a 256-colour GIF than photographic content does, but the truecolor
[`docs/bust.mp4`](../../docs/bust.mp4) still renders the slow hue-drift most faithfully.
