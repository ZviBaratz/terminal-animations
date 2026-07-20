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
2×2 sub-cell fg/bg regions and **braille (U+2800–28FF) into its 2×4 dot grid** — a lit dot
takes the foreground, an unlit dot the background — so braille line art reads as line art
rather than a field of solid rectangles. It still **collapses sextant and octant cells to
their foreground**; those two tiers only read faithfully on a real terminal or the GIF gate.
