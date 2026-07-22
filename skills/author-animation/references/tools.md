# The ecosystem — providers & build-time tools

You are not alone and you are not starting from `math.Sin`. There is a deep ecosystem
of converters, renderers, and libraries. Use it two ways:

1. **Providers** — libraries that *generate* the animation (fresco).
2. **Build-time tools** — converters that turn a source image/video into frames, or
   inform a palette, which you then **bake into the deterministic artifact**. The tool
   runs at author/build time; the shipped animation still runs offline (see *Baking*).

Live runtime pipelines (a `chafa | …` that runs every startup, sixel/kitty pieces) are
the deferred **future hybrid door** — not this plugin's self-contained artifact.

## Providers (Go-native, importable)

- **fresco** — `github.com/ZviBaratz/fresco` — the Go generative-field provider. A pure
  `Render(w, h, frame, opts) string` ANSI engine (deterministic, snapshot-testable) with
  built-in fields: **rain, tunnel, ripple, galaxy**. For a full-pane animated background
  or one of those effects, use fresco rather than reimplementing it — and if you're
  adding a *new* field from inside the fresco repo, that's a fresco variant (skill §A).
  `examples/saucer` is a worked composite that imports fresco from *outside* the repo —
  a cartoon flying saucer drifting across a fresco *aurora* night sky, lighting the sky
  beneath it and trailing a tractor beam the stars show through — showing the
  parse-the-rendered-output-and-composite shape this section describes.
- **ascii-image-converter** — `github.com/TheZoraiz/ascii-image-converter` (+ `aic_package`)
  — image → ASCII/**braille**, truecolor, dithering, in-process.
- **Charm stack** — **bubbletea** (TUI runtime — the *driver*, not the animation),
  **lipgloss** (styling/colour/layout), **harmonica** (spring-physics easing for natural
  motion), **ntcharts** (braille charts). **go-sixel** (`mattn/go-sixel`) emits raster to
  sixel terminals.

## Build-time converters (shell out via `os/exec`)

| Tool | Use | Truecolor | Example |
|---|---|---|---|
| **chafa** | best image→terminal; symbol modes incl. `sextant`; can emit sixel/kitty | yes | `chafa -f symbols --symbols sextant -c full -s 80x40 in.png` |
| **notcurses** / `ncplayer` | video→ANSI; blitters `half/quad/sex(3x2)/oct(4x2)/braille/pixel`, auto-degrades | yes | `ncplayer -b sex clip.mkv` |
| **timg** | image/video viewer; sixel/kitty/iterm + half/quad fallback | yes | `timg -g 80x40 in.png` |
| **viu** | simple viewer; kitty/iterm/sixel else half-blocks | yes | `viu -w 80 in.png` |
| **ascii-image-converter** | CLI too; braille + dither | yes | `ascii-image-converter in.png -C -b --dither -W 80` |
| **ffmpeg** | extract frames; build GIF palettes | (images) | `ffmpeg -i in.mp4 -vf fps=15 f_%04d.png` |

For **video→ANSI** use notcurses/timg with ffmpeg upstream; for GIF output use ffmpeg's
`palettegen` + `paletteuse=dither=bayer` (Bayer, not error-diffusion — no temporal
flicker; see `techniques.md`). Palette-limited (skip for truecolor work): catimg, jp2a,
libcaca/img2txt (the foundational coloured-ASCII library — VLC/MPlayer render through it).

## Beyond the Go core — other-language toolkits, authoring, recording

The Go convention is for the *shipped* artifact; at design time, reach wider.

- **Python engines & canvases** — **terminaltexteffects** (TTE): a zero-dep library of 70+
  ready terminal *text* animations (easing, motion paths, scenes) to study or drive;
  **Rich / Textual** (Textualize): the dominant Python TUI/animation stack, sibling to the
  Charm stack; **drawille**: the canonical braille-canvas abstraction for line art / plots.
- **Cell-grid stacks** — the Charm tools sit on a tcell-style cell buffer; **tcell** /
  **termbox** (Go) are that layer directly, for raw cells without a framework.
- **ANSI-art authoring & formats** — the scene *behind* 16colo.rs: **durdraw** (a modern
  frame-based ASCII/ANSI/Unicode animation *studio* — per-frame timing, 256-colour,
  CP437↔Unicode), the **PabloDraw / Moebius / TheDraw** editors, the **SAUCE** metadata
  standard, and **ansilove** (renders `.ANS`/`.XB` → PNG). Author or convert here, then
  bake to the deterministic artifact.
- **Recording** — besides `vhs` (this plugin's `record.sh`), **asciinema** + **agg**
  captures a live terminal session (asciicast) and renders it to GIF (gifski-based) — the
  right record→GIF path when you're demoing a real running program, not a frame function.

## Baking — keep the artifact deterministic

When a piece is *sourced* from an image/video, run the converter **at build time** and
commit the result as data the Go animation replays — frames as `[]string`, RGBA pixels, or a
derived palette/mask as a `[]rune`/`[]color`. The `Frame`/`Animation` then indexes that baked
data by `tick`. Result: tool-quality visuals, but the shipped animation is still pure,
offline, snapshot-testable. Never invoke a converter from inside the render loop.

**Bake the subject, synthesize the scene — don't bake a finished picture.** The tempting
shortcut with a still is to bake the *whole frame*: matte the subject onto a backdrop,
ffmpeg a Ken Burns pan + brightness "breathe", stack the frames. That ships a photograph
with a camera move — one flat plane, no light, no depth (it is the failure `atmosphere-kit.md`
and `examples/bust`'s history exist to warn against). Bake **only the subject, with an alpha
channel**; synthesize the backdrop, the mist, and the *moving* light at run time in `Frame`
(`atmosphere-kit.md`). A light and a fog that move cannot be frozen — the moment you bake
them flat you are back to the panned photo. What you bake, then, is:

- **Subject motion — a pseudo-3D turn, not a pan.** A single still can't show the back, so
  fake rotation: warp the cut-out with a perspective keystone whose yaw = `A·sinθ` (loops,
  reads as turning). `ffmpeg -vf perspective` or Pillow `Image.transform(PERSPECTIVE)` — bake
  it *premultiplied* (RGB × alpha) with a **non-ringing** downscale (`bilinear`, not Lanczos,
  or the premultiplied edge overshoots into a bright cutout halo), so `Frame`'s composite is
  the one-line `premult + backdrop·(1−α)`.
- **A source video / turntable** for genuine rotation — extract frames with ffmpeg
  (`fps=…`), matte each, bake the sequence. The only honest path to a *literal* spin.
- **Fully-baked atmosphere is possible but usually wrong** — ffmpeg *can* synthesize a
  moving light (`geq` with a light position in `t`), drifting mist (`perlin` → `gblur` →
  `blend`), and a warp, all at build time. Do it only when you must ship a plain frame
  sequence; otherwise synthesize atmosphere live so it stays tunable and can move
  independently of the subject.

**The subject-integrity check (do this before anything else).** A matte silently amputates:
"keep the largest connected component" drops a subject that a bright highlight split into two
blobs; a brightness flood-fill from the borders eats brightly-lit parts of the subject that
touch an edge. **Look at the silhouette** (render the alpha as ASCII, or open the cut-out)
and confirm the *whole* subject survived — not just the easy part. Concretely: keep *every*
component above a size floor (not only the largest), set the background threshold high enough
that lit subject pixels aren't classified as background, and assert a sane coverage fraction
in the bake so a collapse fails loudly. `examples/bust`'s `clean.py` and its `TestAsset`
alpha guard are the worked version.

**Or silkscreen the subject — recolor at run time.** When the subject's *subtle shading* is
exactly what's failing at low resolution (a photo going banded and muddy, a marble bust that
reads as a broken photograph), don't light it — reduce it. Bake only a **luminance + alpha**
matte (contrast-stretched so the tones fill the ramp, de-speckled so no watermark surfaces
under quantization), then at run time **posterize** the luminance into a few flat bands and
map each to a bold flat ink, **cycling the palette** for motion while the geometry stays
frozen. A terminal is bad at subtle and spectacular at bold; flat saturated color is what it
renders best, so this often beats trying to light realism it can't show. `palette-cycle-kit.md`
is the paste-ready code and `examples/bust` the worked piece.

## The headless colour gate

No vhs and no live terminal (a sandbox, CI, an agent)? Render frames and rasterize them
to a PNG you can actually look at:

```sh
go run ./cmd/preview frames 5 | ${CLAUDE_PLUGIN_ROOT}/scripts/ansi2png.py > /tmp/f.png   # then open/Read it
```

`${CLAUDE_PLUGIN_ROOT}/scripts/ansi2png.py` (stdlib-only) turns the truecolor `frames` dump
into an image — the headless stand-in for the GIF gate. You still judge the colour by eye,
never from the formula. It resolves half-block, quadrant and full-block cells into their
2×2 sub-cell fg/bg regions and **braille (U+2800–28FF) into its 2×4 dot grid** — a lit dot
takes the foreground, an unlit dot the background — so braille line art reads as line art
rather than a field of solid rectangles. Sextant and octant both miss, but not in the same
way: a sextant cell **collapses to its foreground**, while an octant cell is **dropped
entirely** — on a Python with a pre-Unicode-16 `unicodedata` the parse loop emits no cell,
shearing every row that contains one. Those two tiers only read faithfully on a real
terminal or the GIF gate; see the headless-gate column in
[`techniques.md`](techniques.md#the-spatial-resolution-ladder).

**`--stats` when looking isn't finding it.** Judging by eye stays the rule, but "it looks
flat and I can't say why" is the one question the eye is bad at. `--stats` adds a numeric
read of the frame on *stderr*, so stdout still carries the PNG:

```sh
go run ./cmd/preview frames 5 | ${CLAUDE_PLUGIN_ROOT}/scripts/ansi2png.py --stats > /tmp/f.png
```

Read it for the shape, not the digits. Most pixels in the bottom luminance bin with the lit
ones piled into one or two hue buckets means **the designed ramp is never being reached** —
a fault in the field feeding the palette, not in the palette, and one no amount of
re-picking colour stops will fix. That is a report to the author, not something to sweep —
see [`craft.md`](craft.md) §"Tune by looking".
