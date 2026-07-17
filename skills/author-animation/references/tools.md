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
libcaca/img2txt.

## Baking — keep the artifact deterministic

When a piece is *sourced* from an image/video, run the converter **at build time** and
commit the result as data the Go animation replays — frames as `[]string`, or a derived
palette/mask as a `[]rune`/`[]color`. The `Frame`/`Animation` then indexes that baked
data by `tick`. Result: tool-quality visuals, but the shipped animation is still pure,
offline, snapshot-testable. Never invoke a converter from inside the render loop.

## The headless colour gate

No vhs and no live terminal (a sandbox, CI, an agent)? Render frames and rasterize them
to a PNG you can actually look at:

```sh
go run ./cmd/preview frames 5 | ${CLAUDE_PLUGIN_ROOT}/scripts/ansi2png.py > /tmp/f.png   # then open/Read it
```

`${CLAUDE_PLUGIN_ROOT}/scripts/ansi2png.py` (stdlib-only) turns the truecolor `frames` dump
into an image — the headless stand-in for the GIF gate. You still judge the colour by eye,
never from the formula. It resolves half-block, quadrant and full-block cells into their
sub-cell fg/bg regions, but **collapses sextant/octant/braille cells to their foreground** —
those finer tiers only read faithfully on a real terminal or the GIF gate.
