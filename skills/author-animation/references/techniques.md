# The technical palette

The levers that decide how much detail and colour a terminal can actually show.
Most "conventional-looking" terminal art is conventional because it never left the
default rung: **one ASCII glyph per cell, functional colour, no dithering.** Reach
here *before* composing — the fidelity tier is a design choice, not a default.

## The spatial-resolution ladder

A monospace cell is ~1×2 (twice as tall as wide). Glyphs that subdivide the cell,
paired with a foreground (ink) and background colour, turn one cell into a small
pixel grid. Pick the rung deliberately.

| Rung | Codepoints | Grid / cell | Colour | Support | Headless gate |
|---|---|---|---|---|---|
| **Half block** ▀▄▌▐ | U+2580, U+2584, U+258C, U+2590 | **1×2** (2px) | **fg+bg = 2 independent truecolor pixels** | Universal. The portable workhorse for real raster. | ✅ resolved |
| **Quadrant** | U+2596–259F | **2×2** (4 regions) | 2 colours/cell (bi-level pattern) | Very broad. | ✅ resolved |
| **Sextant** | U+1FB00–1FB3B (Unicode 13, 2020) | **2×3** (6 regions) | 2 colours/cell | Good by now: kitty, foot, WezTerm, VTE, Konsole. | ❌ collapses to fg |
| **Octant** | U+1CD00–1CDE5 (Unicode 16, Sept 2024) | **2×4** (8 regions) | 2 colours/cell | Newer/patchier; good terminals draw them programmatically. Verify per terminal. | ❌❌ **cell dropped** |
| **Braille** ⠿ | U+2800–28FF | **2×4** (8 dots) | **monochrome** — one fg colour, dots on/off | Universal. | ✅ resolved |

**Climbing can blind your own beauty gate — check the last column before you commit to a
rung.** `ansi2png.py` resolves half-block, quadrant and braille into their sub-cell
regions, but **collapses a sextant cell to a flat foreground block**, and **drops an
octant cell entirely**. That is not a cosmetic caveat: `record-headless.sh` builds the
GIF/MP4 *through* `ansi2png.py`, so on a box without `vhs`/`ttyd` a ❌ rung means you have
**no headless gate and no demo recording** — you would be tuning blind.

Octant is the trap worth spelling out, because it fails *silently and structurally*
rather than visibly. Octants need Unicode 16 (Sept 2024); a Python whose `unicodedata` is
older (3.10 ships UCD 13) reports `isprintable() == False` for U+1CD00–1CDE5, and the
rasterizer's parse loop contributes **no cell at all** for a non-printable character. So
every cell after the first octant on a row **shifts one column left** — a 5-cell row
rasterizes to 4. You do not get a wrong colour, you get a sheared image, which is easy to
misread as a bug in your animation. Check with
`python3 -c "import unicodedata; print(unicodedata.unidata_version)"` before trusting an
octant filmstrip.

If you need a ❌ rung, budget for one of: teaching `ansi2png.py` that rung (the
`rows × cols` sub-cell mask machinery is already there — braille was added this way, and
sextant is a `(2, 3)` table), recording via `record.sh` (needs `vhs` + `ttyd`), or judging
on a real terminal only.

**Braille bit order is irregular — use the table, not a shift.** The codepoint is
`U+2800 + mask`, but the dot numbering is column-major for the historic 6-dot cell
(1,2,3 down the left column, 4,5,6 down the right) and only *then* appends dots 7/8 as a
bottom row. So the obvious `1 << (row*2 + col)` is **wrong on three of the four rows** —
it gives `0x08` for (row 1, col 1) where the answer is `0x10`:

```
 dot1 dot4      0x01 0x08      (row 0, col 0) (row 0, col 1)
 dot2 dot5      0x02 0x10      (row 1, col 0) (row 1, col 1)
 dot3 dot6      0x04 0x20      (row 2, col 0) (row 2, col 1)
 dot7 dot8      0x40 0x80      (row 3, col 0) (row 3, col 1)   <- appended later
```

Spot checks: `⠁` = dot 1 only, `⡀` = dot 7, `⢀` = dot 8, `⡇` = the whole left column,
`⣿` = all eight. Pin the table in a unit test — a wrong mapping still renders *something*
plausible, so it fails silently and reads as a subtly wrong texture rather than a crash.

**Braille depends on the reader's font in a way no other rung does — say so in the
README.** U+2800–28FF is universally *supported* and far from universally *included*:
MesloLGS NF, JetBrains Mono and DejaVu Sans **Mono** all lack it, and fontconfig then
falls back silently, usually to proportional DejaVu Sans. The result is dots at the wrong
pitch inside a monospace cell, and it looks like a bug in your animation. Two traps:

- `fc-match "TheirFont:charset=2800"` naming a *different* family means they are seeing a
  fallback. Do not filter with `:spacing=100` — Iosevka has braille and is tagged
  `spacing=90`, so that query hides it.
- Even with a braille-carrying font, the **line box is taller than four dot rows**, so a
  blank band lands every 4 dot rows, screen-locked. Moving art drifting through those
  bands has dots wink out and back, which reads as jitter. Counter-intuitively the
  *tighter* the font's dots, the *worse* this is — Cascadia Mono's seam is 5.4× its
  intra-cell gap against DejaVu's 1.4×, because shrinking the gap inside a cell does
  nothing to the cell height. Fix is negative line spacing (`font.offset.y` in Alacritty)
  to make the cell exactly four dot pitches. See `examples/torus/README.md` for measured
  tables and a per-launch test command.

Consequence for **gating**: "judge it live" is not sufficient advice on this rung, because
a live terminal with the wrong font is a worse gate than the PNG. Check the font first.
And for **line art specifically**, contrast does much of the work that geometry cannot:
braille dots are separated marks, so dim wires stop grouping into lines at all. A too-dim
palette reads as "dotty" and sends you hunting for a rasterization bug that isn't there —
this happened in `examples/torus`, where the real culprit was a `shadeGamma` of 2.2.

Note also that **braille dots are very nearly square**: a terminal cell is roughly 1×2, and
2×4 dots divide that into 0.5×0.5 units. So drive both axes off the *same* scale factor or
circles come out elliptical — `examples/{plasma,nebula}` do it by normalizing y against the
pixel **width**, `examples/torus` by projecting both axes through one `scale` derived from
the short side. Either works; mixing per-axis normalizations does not.

**The tradeoff that decides the rung:**
- **Braille** = the finest *monochrome* detail (8 addressable dots) — line art, edges,
  plots, high-detail silhouettes (Bad Apple). One colour per cell. Because the mask is
  monochrome, it is the tier that most cleanly forces `craft.md`'s two-channel split:
  let the **dots carry geometry** and **colour carry brightness**, and never dim a line
  by dropping dots — thin strokes shatter into noise (`examples/torus`).
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
columns and smooth in 200. Climbing *does* add sub-cell columns — quadrant/sextant/octant
are two regions wide, not one — but each cell still carries only **two** colours, so the
extra regions buy sharper hard *edges* inside a cell, not a smoother colour ramp: for a
smooth colour field half-block is already the right rung — widen before you climb.

**A photographic subject (a bust, a face) will read soft on half-block** — a smooth
continuous-tone object has no hard edges for the extra sub-cell columns to sharpen, and its
horizontal detail is capped at the column count. Mitigate it rather than fighting the rung:
bake at a **higher native pixel resolution** (widen), and lean on **light and contrast to
define form** — a raking key and a rim carve the silhouette and features the glyph grid
can't resolve on its own (`atmosphere-kit.md`), which is why "dramatic lighting" does double
duty as a *sharpness* tool here. Or sidestep the softness entirely: instead of preserving the
continuous tone, **posterize the subject into a few flat bands and recolor them** — a terminal
loves bold flat color, and a silkscreened subject reads *sharp* precisely because posterizing
manufactures the hard edges the grid can resolve (`palette-cycle-kit.md`). If the deploy target is a known graphics-capable terminal
and photographic fidelity is the whole point, the **graphics-protocol tier below is a real
option, not only a future one** — bake to PNG and blit via sixel/kitty, falling back to the
half-block ladder elsewhere.

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

**Interpolate there too — then bake the ramp.** Picking stops perceptually and then
`lerp`ing between them in sRGB is the common half-measure, and it is fine while the field
is sparse. The moment the field carries *wide smooth gradients* (a glow, a bloom, a broad
vignette) the two flaws become visible: a straight sRGB blend between two saturated stops
dips through a **muddy, darker midpoint**, and a piecewise-linear ramp has a **kink at
every stop** that a wide gradient shows as a Mach band. Both are cheap to fix:

- **Blend in OKLab** — convert the two stops, `lerp` the three components, convert back.
  Clamp on the way out: a blend of two in-gamut colours can leave sRGB.
- **Smoothstep the segment parameter** (`t·t·(3-2t)`) so the ramp is C1 across each stop
  and the kink disappears.
- **Bake the result into a lookup table** (a few hundred entries) at init and index it
  per pixel. This is not an optimization, it is what makes the above *affordable*: an
  OKLab round trip is a dozen-odd `pow`/`cbrt` calls, and evaluating one per pixel per
  frame measured at roughly **three times an entire frame budget**. With the table the
  whole upgrade came out **cheaper than the piecewise sRGB lerp it replaced**, because a
  table index also beats searching the stop list per pixel.

Generalize the last point: **any scalar → colour function is LUT-able**, and per-pixel
transcendentals are the first thing to hunt when a field animation misses its frame
budget. Profile the *pixel* loop, not the simulation — the simulation usually isn't the
problem.

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
