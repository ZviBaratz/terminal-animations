# nebula — a deep-space splash field

![drifting through a nebula — half-block truecolor, seamless loop](../../docs/nebula.gif)

A standalone splash-screen animation: a pure, deterministic, **half-block truecolor**
field that evokes **drifting slowly through a nebula** — calm, slow, and looping
forever with no seam. It follows the skill's §B convention (a field-shaped standalone,
authored outside the fresco repo) and composes several techniques past the conventional
"one effect in flat ASCII" default.

## What it demonstrates

- **Fidelity tier — half blocks.** Every cell is a `▀`: the foreground colour paints
  the upper pixel, the background the lower, so the visible grid is `w × 2h` independent
  24-bit pixels — the "portable workhorse" rung from `references/techniques.md`.
- **A real cloud, not banded plasma.** Fractal value noise (`fbm`) put through an
  Iñigo-Quílez **domain warp** (`warpedDensity()`), which turns flat gradients into
  swirled, billowing structure. All randomness enters only through an integer coordinate
  hash (`hash2`), so the field is snapshot-testable.
- **A designed palette.** A cool **indigo → violet → magenta** ramp (`nebStops`) — deep
  blue-black voids, indigo dust, a soft magenta glow in the dense cores — not a
  functional grey→white gradient. Brightness rides colour luminance, so dim regions stay
  a smooth wash instead of breaking into confetti.
- **Depth — drifting parallax stars.** A sparse star layer (`starLum()`) behind the
  cloud that slides *with* the drift, slower (being far away), so it reads as travel
  through space and never as stuck pixels over a moving field (`references/craft.md`).
- **Motion-stable dithering.** Screen-locked **Bayer** (ordered) dither, so quantization
  never shimmers or crawls as the field drifts — the right choice for animation, unlike
  error diffusion.
- **Composition.** An **edge vignette** fades every border to black, so the splash reads
  as a window onto something larger rather than a lit rectangle.
- **A truly seamless forever-loop.** Every time term flows through a single phase
  `θ = 2π·(tick mod period)/period`, so `Frame(w,h,0)` and `Frame(w,h,period)` receive
  identical inputs and are **byte-identical** — pinned exactly by `TestLoopSeam`.
- **Determinism.** `Frame(w, h, tick)` is pure — no wall clock, no `math/rand`.
  `nebula_test.go` pins the `h×w` contract, no-panic on any `(w, h, tick)`,
  byte-stability, the seamless loop, that consecutive frames actually move, and a golden
  frame.

## Run it

```sh
cd examples/nebula
go run ./cmd/preview            # live, in colour (Ctrl-C to quit — cursor is restored)
go run ./cmd/preview frames 5   # dump 5 frames (structure + colour check)
go test ./...                   # shape, no-panic, determinism, loop-seam, motion, golden

# headless colour gate (no TTY needed): rasterize frames to a PNG and look at it
go run ./cmd/preview frames 6 | ../../scripts/ansi2png.py > /tmp/nebula.png
```

## How the demo GIF was made

`docs/nebula.gif` was produced with the plugin's **own** headless pipeline — no `vhs`
required: dump frames spanning exactly one loop at a high-resolution `220×56` size →
rasterize each with `ansi2png.py` → assemble with `ffmpeg` using **Bayer** dithering
(ordered, stable under motion). Because the animation is already a seamless loop
(`period` ticks close back on frame 0), the GIF loops perfectly with no ping-pong.

The GIF is capped at 256 colours, which bands a smooth truecolor nebula; the **live
preview** (`go run ./cmd/preview`, full 24-bit) is the true fidelity, and a truecolor
`ffmpeg` **MP4/webm** of the same frame set shows it losslessly if you want a shareable
capture.

## Tuning notes

The taste constants at the top of `nebula.go` — `period` (drift speed), `driftR`/`churnR`/
`churnK` (drift & billow), `noiseScale`/`octaves`/`warpAmt` (cloud structure), the
`voidCut`/`densityGain`/`densityGamma` shaping, `vigPow`, and the star + palette values —
were swept and picked **by eye** against the `ansi2png` filmstrip. `churnK` was dropped
to `1` and `period` lengthened so the cloud evolves *once*, slowly, per loop — the
"calm and slow" the brief asked for — exactly the "tune by looking, not arithmetic" loop
the plugin preaches.
