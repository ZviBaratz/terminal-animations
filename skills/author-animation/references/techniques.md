# The technical palette

The levers that decide how much detail and colour a terminal can actually show.
Most "conventional-looking" terminal art is conventional because it never left the
default rung: **one ASCII glyph per cell, functional colour, no dithering.** Reach
here *before* composing — the fidelity tier is a design choice, not a default.

## The spatial-resolution ladder

A monospace cell is ~1×2 (twice as tall as wide). Glyphs that subdivide the cell,
paired with a foreground (ink) and background colour, turn one cell into a small
pixel grid. Pick the rung deliberately.

| Rung | Codepoints | Grid / cell | Colour | Support |
|---|---|---|---|---|
| **Half block** ▀▄▌▐ | U+2580, U+2584, U+258C, U+2590 | **1×2** (2px) | **fg+bg = 2 independent truecolor pixels** | Universal. The portable workhorse for real raster. |
| **Quadrant** | U+2596–259F | **2×2** (4 regions) | 2 colours/cell (bi-level pattern) | Very broad. |
| **Sextant** | U+1FB00–1FB3B (Unicode 13, 2020) | **2×3** (6 regions) | 2 colours/cell | Good by now: kitty, foot, WezTerm, VTE, Konsole. |
| **Octant** | U+1CD00–1CDE5 (Unicode 16, Sept 2024) | **2×4** (8 regions) | 2 colours/cell | Newer/patchier; good terminals draw them programmatically. Verify per terminal. |
| **Braille** ⠿ | U+2800–28FF | **2×4** (8 dots) | **monochrome** — one fg colour, dots on/off | Universal. |

**The tradeoff that decides the rung:**
- **Braille** = the finest *monochrome* detail (8 addressable dots) — line art, edges,
  plots, high-detail silhouettes (Bad Apple). One colour per cell.
- **Half/quadrant/sextant/octant** = *colour* raster, coarser, only 2 colours in a
  cell. **Half-block is the sweet spot**: 2 fully independent 24-bit pixels per cell,
  works everywhere. Climb to sextant/octant for more colour-pixels where the terminal
  supports it.
- **1×1 ASCII glyph ramp** (`" .:-=+*#%@"`) = texture/character, lowest spatial
  resolution. Right for a deliberately *typographic* look — not a default to settle for.

**Why it can look pixelated, and the real sharpness lever.** A half-block field is exactly
`w × 2h` pixels — **one pixel per column** horizontally (the cell is one column wide), two
rows per cell vertically. So horizontal detail is capped at the terminal's column count:
**filling the terminal is the main sharpness lever** — the same field is blocky in 40
columns and smooth in 200. Quadrant/sextant/octant only subdivide the cell *vertically* and
still carry two colours per cell, so they sharpen hard edges, not smooth gradients: for a
smooth colour field half-block is already the right rung — render it wider, don't climb.

## The graphics-protocol tier (true raster — future/high-fidelity)

Not glyph tricks: the terminal blits real pixels from your image data. Use for
photographic fidelity, smooth gradients, sub-glyph detail — on a known-capable
terminal. Detect and fall back to the ladder otherwise (chafa/notcurses do this).

- **Sixel** — lowest-common-denominator raster; palette-indexed. Broadest support
  (xterm, foot, WezTerm, Konsole, Windows Terminal, iTerm2, Ghostty, VTE…).
- **Kitty graphics protocol** — RGBA + alpha, precise placement, animation. Richest;
  kitty, WezTerm, Ghostty.
- **iTerm2 inline images** — OSC 1337 base64 PNG; simple, least portable.

Caveats: awkward under tmux, larger payloads, no glyph fallback. In *this* plugin the
self-contained deterministic artifact stays glyph-based; graphics protocols are the
**future runtime-hybrid door**, and a build-time source (render to PNG, view it).

## Colour depth & degradation

16 (SGR 30–37/90–97) → 256 (`38;5;N`) → truecolor (`38;2;R;G;B`). Detect truecolor via
`COLORTERM=truecolor|24bit` or terminfo `RGB`/`Tc`; robust code queries the terminal
(DA/OSC) rather than trusting `$TERM`. **Author in truecolor, then quantize down**:
24-bit → nearest 256-cube → nearest 16. Default to 256 as the safe middle when unsure.
Luminance for a ramp: `Y ≈ 0.2126R + 0.7152G + 0.0722B`.

**Work perceptually for palettes & matching.** sRGB is not perceptually uniform, so
"nearest colour" and "evenly-spaced gradient" done in RGB come out lumpy — uneven steps,
muddy mid-tones. Design palettes and do nearest-palette quantization in a perceptual
space: **OKLab / OKLCH** (Ottosson, 2020), where Euclidean distance tracks how different
two colours *look*. Convert sRGB → OKLab, pick/space/match there, convert back for the SGR
bytes. (The luminance `Y` above is fine for a brightness ramp; it is not a colour
*distance*.)

## Dithering (smooth gradients on a limited ramp/palette)

- **Floyd–Steinberg** (error diffusion): best-looking *stills*, least banding. **But
  the pattern is position-dependent, so it shimmers/crawls under motion** — avoid for
  animation.
- **Ordered / Bayer** (fixed threshold matrix, 2×2→4×4→8×8): a given value always
  dithers the same way → **stable under motion, no temporal flicker.** The right choice
  for moving terminal art and video (it's why ffmpeg animation pipelines use
  `paletteuse=dither=bayer`).
- **Blue noise** (and **spatiotemporal blue noise**): a threshold texture with no
  low-frequency clumping — a still reads less mechanical than Bayer's visible grid, and a
  *spatiotemporal* blue-noise texture (EA/NVIDIA) stays stable frame-to-frame: the current
  state of the art for *animated* dithering. Use one if you have it; Bayer stays the cheap,
  dependency-free motion-stable default.
- **Rule:** stills → Floyd–Steinberg (or blue noise); motion → ordered/Bayer (or
  spatiotemporal blue noise).

## The width-1 safety rule (always)

Every glyph technique above is width-1-safe *except* when you stray outside the listed
sets. A double-width glyph (much CJK, many emoji) breaks the `w` count. Index glyph
sets as `[]rune`, never `string`. For a smooth luminance field, the safest "pixel" is a
**space painted with a background colour** — no glyph, no width trap (see `craft.md`).
