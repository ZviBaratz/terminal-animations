# The tuning harness

Two loops, straight from `references/craft.md`: a fast **inner loop** to check
structure, and a **beauty gate** that records a GIF so you watch the motion in
colour.

## Files

| File | What it is |
|---|---|
| `preview/` | Copy the directory to `cmd/preview/`, rename `main.go.tmpl` → `main.go`, and wire `render()`. `main.go.tmpl` is the live loop + `frames` dumper; the verbatim build-tagged `size_*.go` give the live loop the real terminal size so it fills the pane. |
| `preview.sh` | Thin wrapper that runs the preview program live (`Ctrl-C` to quit). |
| `record.sh` | The beauty gate: records a short GIF of the preview via vhs. |
| `record-headless.sh` | Headless beauty gate: turns a `frames` dump into a seamless-loop GIF + a truecolor MP4 with only `ffmpeg` + `python3` — no vhs. |
| `ansi2png.py` | Headless colour gate: rasterizes the `frames` dump into a PNG you can open/Read when there's no TTY. Stdlib Python, no deps. |
| `harness/` | Copy the directory to `cmd/wasm/`, rename `main.go.tmpl` → `main.go`, and wire `render()` — the same one line as `cmd/preview`. Compiles the animation to WASM for the browser harness. |
| `harness.sh` | Builds an animation to WASM and serves the pages with the dev panel open: scrub any tick, drag the pane, compare tick vs tick+period. Needs only `go` + `python3`. |
| `posters.sh` | Renders the still frame each viewer shows before its module lands — and the *only* thing shown under `prefers-reduced-motion` or on a phone. Landscape + portrait, via `ansi2png.py`. |
| `manifest.py` | Writes `web/animations.json` by merging each `examples/<name>/meta.json`. Used by both `harness.sh` and the pages workflow, so the two cannot drift. |
| `../web/` | The pages themselves — static files, no node, no bundler. `index.html` is the gallery (zero WASM); `view.html` is the viewer, and the authoring harness is that same page with `&dev`. `harness.js` holds the painter, `glyphs.js` the sub-cell tables, `gallery.js` the index. |

## Inner loop (fast, no extra tools)

```sh
# live, in colour — fills the terminal and reflows on resize (Ctrl-C to quit):
scripts/preview.sh                        # runs `go run ./cmd/preview`

# structure + headless colour check (no TTY needed):
go run ./cmd/preview frames 5             # dump ticks 0..4 at 80×24
go run ./cmd/preview frames 90 12         # 90 frames strided by 12 (ticks 0,12,…) — shows motion in a slow loop
go run ./cmd/preview frames 90 12 200 56  # …at an explicit 200×56 pane (a big field for the PNG gate)
go run ./cmd/preview frames 1 | cat -v    # see the raw SGR colour bytes
```

The live loop fills the whole terminal and follows resizes; `frames N [stride] [w h]`
dumps a deterministic filmstrip — `stride` spreads the ticks so a slow forever-loop still
shows motion, and the optional `w h` renders a big field for the headless gate.

Check: exactly `h` lines of `w` cells, width-1 glyphs, real negative space,
consecutive frames differ, and — the step that's easy to skip — the colour
actually varies the way you intended.

## Browser harness (needs only go + python3)

The terminal loop can't scrub, and checking the resolution ladder means resizing
a real window by hand. The browser harness compiles the animation to WASM and
drives `Frame(w, h, tick)` directly, so tick and pane size become controls:

```sh
scripts/harness.sh examples/nebula        # → http://localhost:8731/view.html?anim=nebula&dev
scripts/harness.sh examples/nebula 9000   # …on a different port
```

| Control | What it answers |
|---|---|
| tick slider, `step`, `←`/`→` | What does frame N actually look like? (no rebuild) |
| `w`/`h`, `fit to window` | Does it reflow across the resolution ladder? |
| `compare` + `Δ` | Is the loop seamless? Set Δ = period; the panes must be identical. |
| `render / paint` readout | Which side costs what? `render` is the Go call, `paint` is the canvas — they fail differently, so a single combined number cannot tell you which one moved. |
| `⚠ N glyphs via font` | Are any cells falling off the sub-cell model? Non-zero means those cells neither match the PNG export nor stay on the fast path. It should always be zero. |

The animation needs a `cmd/wasm` entrypoint — copy `harness/` as above. This is
a *looking* tool, not a gate: `record-headless.sh` still owns the artifact, and
`ansi2png.py` still owns the headless colour check. Note also that scrubbing
backwards is only meaningful for a pure `Frame(w, h, tick)` animation; a
stateful one can only be replayed forward.

`harness.sh` lands you on `view.html?anim=<name>&dev` — the visitor's page with the
dev panel open. `~` toggles the panel, `esc` goes back to the gallery. There is
only one painter: what you tune here is what a visitor sees, to the pixel.

Posters are not rendered by default, since each costs a full preview run and the
viewer treats a missing one as "start on black". `POSTERS=1 scripts/harness.sh …`
when you want to check the still.

### Checking the page against the PNG export

The page claims to match `ansi2png.py` — same sub-cell model, so a block or
braille frame rasterizes identically. That claim is checkable, and worth checking
whenever the painter changes:

```sh
cd examples/torus
go run ./cmd/preview frames 1 1 100 28 | ../../scripts/ansi2png.py --cw 6 --ch 12 > /tmp/t.png
```

At `cell=6` the canvas is 600×336 and so is the PNG — the same pixel grid, no
rescale — so the two should be pixel-identical, and a mismatch is diffable rather
than a matter of taste. Odd cell sizes are the discriminating case: at even widths
a correct and an incorrect sub-cell split can look the same.

### Publishing it

`.github/workflows/pages.yml` builds every `examples/*/cmd/wasm` and publishes
`web/` to GitHub Pages on push to `main` (PRs build only, so a broken WASM build
is caught pre-merge without overwriting the live site). The build step is needed
because `web/` is source-only — the `.wasm` modules, `wasm_exec.js`, the manifest
and the posters are all gitignored, and `wasm_exec.js` in particular must match the
toolchain that built the module.

An animation joins the site by having a `cmd/wasm` entrypoint; the workflow
discovers it by glob. Add a `meta.json` beside it (title, blurb, `resolutions`,
`accent`, `loop`, `posterTick`) and it also gets a proper listing on the index —
without one it still appears, on resolution 1 (half block), with no blurb.

`resolutions` is a list of ladder rungs (`[1]` half block … `[5]` braille), not a
single value: the resolution ladder is one label dimension, so an animation that
combines rungs — say a half-block field with braille detail — lists under each one
it uses. It is the first such dimension; others (loop character, palette) would be
added as further list-valued keys, and the gallery groups by whichever it renders.

One-time repo setup: **Settings → Pages → Source = "GitHub Actions"**.

## Beauty gate (needs vhs + ttyd + ffmpeg)

```sh
scripts/record.sh --build "go build -o /tmp/anim ./cmd/preview" -- /tmp/anim
# → out/preview.gif ; open it and watch the motion.
```

Build first (hidden) rather than `go run` so the recording never captures
compilation. Keep the window and framerate modest — a full-pane field changes
every cell every frame and compresses poorly. Run `scripts/record.sh --help` for
sizing and options.

> Install vhs: <https://github.com/charmbracelet/vhs#installation> (it pulls in
> `ttyd` and `ffmpeg`). If they're not on PATH, `record.sh` says so and stops.

**No vhs and no live terminal (a sandbox, CI, an agent)?** Rasterize the frames to a
PNG and look at it:

```sh
go run ./cmd/preview frames 5 | ./scripts/ansi2png.py > /tmp/anim.png
# → open or Read /tmp/anim.png ; frames are stacked into a filmstrip.
```

`ansi2png.py` (stdlib Python, no deps) turns the truecolor `frames` dump into an
image — the headless stand-in for the GIF gate. You still judge the colour by eye,
never from the formula. It resolves half-block, quadrant and full-block cells into their
2×2 sub-cell regions and braille into its 2×4 dot grid; a sextant cell collapses to its
foreground, and an octant cell is dropped entirely — on a pre-Unicode-16 `unicodedata` the
parse loop emits no cell, shearing every row that contains one.
Cell size: `--cw` / `--ch` flags (else the `ANSI2PNG_CW` /
`ANSI2PNG_CH` env vars, else 7×14 — use `--ch 4` or more so all four braille dot rows
get a pixel):

```sh
go run ./cmd/preview frames 5 | ./scripts/ansi2png.py --cw 8 --ch 16 > /tmp/anim.png
```

Prefer the flags — an env var set before the `|` (`ANSI2PNG_CW=8 go run … | ansi2png.py`)
applies to the *producer*, not to `ansi2png`, so it is silently ignored.

**`--stats` — measure the frame when looking isn't finding it.** Judging by eye stays the
rule, but "it looks flat and I can't say why" is a question the eye is bad at. `--stats`
prints a luminance histogram, how much of the frame is dark, and how the lit pixels spread
across the hue wheel — to *stderr*, so stdout still carries the PNG:

```sh
go run ./cmd/preview frames 5 | ./scripts/ansi2png.py --stats --cw 6 --ch 12 > /tmp/anim.png
```

Read it for the shape, not the digits. Most pixels in the bottom luminance bin with the lit
ones piled into one or two hue buckets means **the designed ramp is not being reached** —
the palette is fine, the field feeding it never visits the palette's middle. That is a fix
in the field (longer trails, spatial diffusion, a different mapping), not in the colours.
`examples/life` was measured at 76% of pixels below luminance 16 with 19% of lit pixels
orange against 4% violet — a seven-stop ramp rendering as orange-on-black — which is what
sent that rework at the field instead of at the stops.

### The moving artifact, still no vhs — `record-headless.sh`

`ansi2png.py` gives you a still filmstrip; `record-headless.sh` gives you the *moving*
artifact on the same vhs-free box (only `ffmpeg` + `python3`). It runs your `frames`
command, splits the dump on the `--- frame N ---` headers, rasterizes each frame through
`ansi2png.py`, and encodes a 256-colour **seamless-loop GIF** (motion-stable Bayer dither)
plus a truecolor **MP4**:

```sh
# 120 frames × stride 9 = 1080 ticks = exactly one nebula loop → a seamless GIF + MP4:
scripts/record-headless.sh -o out/nebula -- go run ./cmd/preview frames 120 9 220 56
# → out/nebula.gif  (loops forever)   and   out/nebula.mp4  (smaller, sharper)
```

Make `frames × stride` equal one loop `period` so the GIF loops seamlessly — that's what
the scaffold's strided `frames N [stride] [w h]` mode is for. A full-motion field
compresses poorly as a GIF, so the MP4 is typically ~20–30× smaller at higher fidelity and
is the better "optimal" artifact. `scripts/record-headless.sh --help` lists the fps / width
/ cell-size knobs (and `--no-gif` / `--no-mp4`).

**Detail tiers must record 1:1 — match `pane × cell-size` to `--width`.** `--width`
(default 640) rescales the rasterized frames, which is fine for a *field*: a nebula is a
smooth colour ramp and survives resampling. It is destructive for **line art on a sub-cell
tier**, where the artwork *is* the dot pattern — downscaling averages individual dots into
the background and a crisp wireframe arrives as a grey haze. Size the recording so no
rescale happens at all:

```sh
# braille torus: 100 cols × 6px = 600px wide, so --width 600 means NO rescale.
# 120 frames × stride 6 = 720 = Period(100, 28) — exactly one loop at *this* pane.
# The torus scales its loop length with the pane, so re-derive this if you resize.
scripts/record-headless.sh -o out/torus --width 600 --cw 6 --ch 12 -- \
  go run ./cmd/preview frames 120 6 100 28
```

Note the pane is *deliberately small*. Recording a braille piece at a field's 220×56 and
letting ffmpeg scale to 640 shrinks each dot below one pixel. The lever that makes a field
sharper — fill the terminal — inverts for detail tiers: **choose the pane so the dots land
on whole pixels**, then match `--width` to it.
