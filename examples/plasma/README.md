# plasma — the reference animation

![half-block truecolor plasma](../../docs/plasma.gif)

The reference standalone animation for this plugin: a pure, deterministic,
**half-block truecolor plasma**. It exists to show the craft *composed past* the
conventional "one effect in flat ASCII" default — and to prove the convention and the
harness end to end.

## What it demonstrates

- **Fidelity tier — half blocks.** Every cell is a `▀`: the foreground colour paints
  the upper pixel, the background the lower, so the visible grid is `w × 2h`
  independent 24-bit pixels — the "portable workhorse" rung from
  `references/techniques.md`, not a flat `.·+*#@` ramp.
- **A designed palette.** A cosine gradient (`palette()` in `plasma.go`) — a cohesive
  indigo → rust → gold nebula with a cool accent — chosen against the beauty gate, not
  a functional grey→white ramp. Brightness rides colour luminance, so it stays smooth
  instead of breaking into confetti.
- **Composition.** A summed-sine plasma plus a radial ripple whose focus slowly
  **orbits**, all under an **edge vignette** so the field sits in real negative space
  rather than meeting a hard rectangular border (`references/craft.md`).
- **Determinism.** `Frame(w, h, tick)` is pure — no wall clock, no `math/rand` — so it
  is snapshot-testable. `plasma_test.go` pins the `h×w` contract, no-panic on any
  `(w, h, tick)`, byte-stability, and a golden frame.

It follows the skill's §B convention exactly: a pure `func Frame(w, h, tick int) string`,
a `cmd/preview/` copied from `scripts/preview/` (the live loop fills the terminal; the
`frames` dumper feeds the headless gate), and the test above.

## Run it

```sh
cd examples/plasma
go run ./cmd/preview            # live, in colour (Ctrl-C to quit — cursor is restored)
go run ./cmd/preview frames 5   # dump 5 frames (structure + colour check)
go test ./...                   # bounds, no-panic, determinism, golden

# headless colour gate (no TTY needed): rasterize frames to a PNG and look at it
go run ./cmd/preview frames 5 | ../../scripts/ansi2png.py > /tmp/plasma.png
```

## How the demo GIF was made

`docs/plasma.gif` was produced with the plugin's **own** headless pipeline — no `vhs`
required: dump frames → rasterize each with `ansi2png.py` → assemble with `ffmpeg`
using **Bayer** dithering (ordered, so it's stable under motion — no temporal shimmer,
unlike error diffusion; see `references/techniques.md`), looped ping-pong so it's seamless.

## Tuning notes

The taste constants at the top of `plasma.go` (`speed`, `drift`, `warp`, `bands`,
`vigPow`, and the palette `d` offsets) were swept and picked **by eye** against the
`ansi2png` filmstrip. The palette `d = (0.00, 0.16, 0.34)` was chosen over a tighter
warm-only set (too monochrome) and a wider full-spectrum spread (the rainbow cliché) —
exactly the "tune by looking, not arithmetic" loop the plugin preaches.
