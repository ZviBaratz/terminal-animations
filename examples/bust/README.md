# bust — a marble bust, lit and turning in mist

![a classical marble bust turning under a sweeping warm light, dissolving into drifting mist — half-block truecolor, seamless loop](../../docs/bust.gif)

> **Full fidelity:** the GIF above is 256-colour; the truecolor 24-bit capture is
> [`docs/bust.mp4`](../../docs/bust.mp4) — closer to what the live terminal shows.

<video src="../../docs/bust.mp4" width="560" autoplay loop muted playsinline>
  Inline video isn't supported here —
  <a href="../../docs/bust.mp4">watch or download <code>docs/bust.mp4</code></a>.
</video>

A looping terminal animation where **only the subject is baked** from a still PNG, and the
**scene around it is synthesized live**. It is the author-animation skill's "bake the
subject, synthesize the scene" pattern (`references/atmosphere-kit.md`, `tools.md` §Baking):
the marble is matted off its background at build time with an alpha channel; the drifting
mist, the sweeping light, and the lit backdrop are pure functions of `tick` in Go. Nothing
is installed to run it, and nothing about the atmosphere is frozen into the frames.

## Vision Card

- **Subject** — a classical marble bust, matted whole (head + shoulders) from a stock still.
- **Motion verb** — *turning*. A seamless pseudo-3D yaw, not a pan.
- **Light** — a warm key that **orbits** the head, raking the marble; a cool silhouette rim.
- **Atmosphere** — a dark lit-space backdrop; fbm mist drifting behind and pooling at the
  dissolving base, thin wisps in front.
- **Palette** — cool blue-black space against warm-lit white marble; the contrast is the drama.
- **The one special idea** — the statue is *lit and turning in weather*, a live scene, not a
  photograph with a camera move.

## What it demonstrates

- **Bake the subject, synthesize the scene.** `bake.sh` bakes *only* the bust — matted, with
  alpha — as a seamless turn. `bust.go` composites it, every tick, over a runtime backdrop +
  mist + a moving light. A light and a fog that move can't be frozen, so they stay live; that
  is the whole reason the baked frames keep an alpha channel. This is the fix for this
  example's first cut, which was the anti-pattern: a matted still panned in an ellipse over a
  baked spotlight — a photograph with a Ken Burns move.
- **A pseudo-3D turn, honestly.** A single still can't reveal the statue's back, so the
  "turn" is a gentle perspective keystone (yaw = `A·sinθ`) baked into the frames — it reads
  as *rotating*, not sliding, and loops. Baked **premultiplied** with a non-ringing downscale
  so the composite is a clean `premult + backdrop·(1−α)` with no cutout halo.
- **Dramatic lighting as a moving field.** The warm key orbits on the loop phase; the marble
  brightens and warms where the light rakes and falls into cool shadow elsewhere, with the
  silhouette edge receding into the backdrop except where the light catches it. On the
  soft-by-nature half-block rung, this raking light is what *defines* the form.
- **A matte that keeps the whole subject.** The bust is white marble on a white field — the
  hard case. `clean.py` floods only near-pure-white (so lit marble that touches the frame
  isn't eaten) and keeps every component above a size floor (so a highlight can't split off
  the shoulders). The torso's flat bottom crop is dissolved to transparent so the mist can
  pool where the bust fades. `TestBakedSheet` guards that the matte didn't collapse.
- **Fidelity tier — half blocks.** Every cell is a `▀`: foreground paints the upper pixel,
  background the lower, so the visible grid is `w × 2h` independent 24-bit pixels.
- **A seamless forever-loop.** Every atmosphere term is a sinusoid of `θ = 2π·(tick mod
  period)/period` and the subject sheet is indexed `tick mod period`, so `Frame(w,h,0)` and
  `Frame(w,h,period)` are **byte-identical** — pinned by `TestLoopSeam`.
- **A resolution-independent fit.** The atmosphere fills any pane; the native `140×70`-cell
  subject is centered in it — cropped to a dramatic close-up when the pane is smaller,
  sitting in more scene when larger — so `Frame` keeps the "exactly `h` lines of `w` cells"
  contract at every size (`TestShape`).
- **Determinism.** `Frame(w, h, tick)` is pure — no wall clock, no `math/rand`. Tests pin the
  `h×w` contract, no-panic on any `(w, h, tick)`, byte-stability, the seam, that consecutive
  frames move, the decoded-sheet integrity (including that alpha is present), and a golden.

## Run it

```sh
cd examples/bust
go run ./cmd/preview                    # live, in colour (Ctrl-C to quit — cursor is restored)
go run ./cmd/preview frames 5           # dump 5 frames (structure + colour check)
go test ./...                           # shape, no-panic, determinism, loop-seam, motion, golden

# headless colour gate (no TTY needed): rasterize a filmstrip to a PNG and look at it
go run ./cmd/preview frames 4 18 140 70 | ../../scripts/ansi2png.py --cw 4 --ch 4 > /tmp/bust.png
```

## Re-baking the artifact

`frames.png` is committed, so nothing below is needed to *run* the animation — only to
regenerate the baked subject from a different source or with a different turn.

```sh
cd examples/bust
./bake.sh ~/Downloads/bust.png          # clean.py (matte → RGBA) → Pillow (turn) → frames.png
go test ./... -run TestGolden -update    # frames.png changed ⇒ refresh the pinned golden
```

`bake.sh` needs `python3` + Pillow on `PATH` (author-time only). The **turn** constants live
in `bake.sh` (yaw amplitude, keystone insets); the **atmosphere** constants (light, mist,
backdrop, rim) live in `bust.go` and are documented in `atmosphere-kit.md` — all swept **by
eye** against the `ansi2png` filmstrip, per the plugin's "tune by looking, not arithmetic"
loop. The watermarked source is **never copied into the repo**; only the clean `frames.png`
ships.

## How the demo media was made

The plugin's own headless recorder, `scripts/record-headless.sh` — no `vhs` required:

```sh
# a lean README GIF (36 frames × stride 2 = one full loop) and the truecolor MP4
../../scripts/record-headless.sh -o ../../docs/bust --no-mp4 --fps 18 --width 400 -- \
  go run ./cmd/preview frames 36 2 140 70
../../scripts/record-headless.sh -o ../../docs/bust --no-gif --fps 24 --width 560 -- \
  go run ./cmd/preview frames 72 1 140 70
```

Because the animation is a seamless loop, the GIF loops with no ping-pong. Full-motion
photographic content compresses poorly as a 256-colour GIF, so the truecolor
[`docs/bust.mp4`](../../docs/bust.mp4) is the smaller, sharper artifact.
