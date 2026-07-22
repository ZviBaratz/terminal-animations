# Changelog

All notable changes to the `terminal-animations` plugin are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Because the plugin's `version` is pinned in `.claude-plugin/plugin.json`, installed
> users only receive updates when that version is bumped. Bump it — and add an entry
> here — with every user-facing change.

## [Unreleased]

### Added

- **`examples/life`** — Conway's Game of Life as a **symmetric kaleidoscope**: a glowing
  **rose window** of cathedral glass that blooms, cools, and reblooms — and the first
  worked example of the stateful
  **`Animation` interface** (`Update` / `View` / `Done`). The convention has always described
  that shape — and the test contract even names a Life blinker and glider as its canonical
  goldens — but every gallery piece was a pure `Frame(w, h, tick)`, so the interface had zero
  worked examples. Life earns the stateful shape honestly: a cell's next value depends on its
  neighbours, not on the tick, so there is no closed form to fold into a `Frame`. The
  load-bearing idea is that a **random-soup Life is intrinsically noise** — a glyph in every
  cell, no focal point, no negative space, which `craft.md` warns reads as texture, not weather
  (two earlier cuts — braille cells, then ember-coloured soup — were rejected at the beauty gate
  for exactly this). No colour treatment fixes noise; the fix is to change *what is simulated*.
  Conway's rules are **isotropic**, so a seed built with a symmetry group stays symmetric for
  every later generation, for free: seeding the soup with the full **8-fold (D4) symmetry of the
  square** turns the identical churn into a mandala with a focal centre. The symmetry is never
  enforced in the step (that would corrupt the physics) — it rides the rules' own equivariance,
  from an exactly-symmetric seed (a fundamental wedge mirrored into all eight images). The sim is
  confined to a **centred disc** that is grown to **fill the frame**: the sim runs on a *square
  board larger than the pane* and the pane views a centred window into it, so a wide pane sees a
  full-width band through the middle of the mandala, and a **radial vignette** fades the corners
  (outside the disc) to black so the circle is sculpted by the fade rather than left as a small
  medallion in a void. The square board is also what keeps the diagonal reflection exact — that
  reflection is a lattice symmetry only about the **integer** centre, which a square gives for any
  pane (a non-square board's diagonal maps cells off-grid), and confining growth to the interior
  disc keeps it there (the side is rounded up to **odd** so the board is exactly symmetric about
  that centre pixel — the cells never notice, but the halo's blur reads the whole board and an
  even side leaves an edge tap present on one side and missing on the other). The ember
  **treatment** carries over: a per-pixel **heat** field that a birth **ignites** toward
  white-hot over a few frames and a death leaves to **decay** (a bright leading edge with a
  decaying trail — the canonical "this is moving" signal; igniting rather than flashing a birth
  on in one frame is what makes the motion read as a smooth breath, not a jump), and a sustain
  that **cools with age** so settled regions dim to a low ember rather than sitting as stuck
  bright points. In a symmetric field this turns flicker into a **pulse**: the whole mandala
  breathes. On top of that field sits the colour pipeline, which is what makes it read as
  *light*: a Life cell is one pixel, so painting heat directly gives hard-edged blocks whose
  colour jumps from neighbour to neighbour — a mosaic — so the heat is also blurred into a
  **halo** and composited back over itself with a screen blend (a sliding-window box blur, O(1)
  per pixel, identical on both axes so the symmetry survives). Light bleeds between cells, the
  gaps light up, and every falloff becomes a continuous gradient. The ramp is **cathedral
  glass** — cobalt, violet, magenta, ruby, gold, white — blended in **OKLab** rather than sRGB
  (a straight lerp between saturated stops dips through a muddy midpoint, which wide gradients
  show plainly), **baked to a lookup table** at init (the conversion is a dozen transcendentals
  a sample, far too much per pixel per frame — with the table the whole upgrade costs *less*
  than the ramp it replaced) and **dithered** with the screen-locked Bayer matrix `torus` and
  `nebula` already use. The radial vignette fades the ramp **coordinate** rather than the
  finished colour, so the falloff walks the palette down through gold, ruby, magenta and violet
  to cobalt: the disc reads as concentric bands of glass, with a gentle luminance falloff on top
  still carrying the corners to black.
  Half-block tier (`▀`, board `w × 2h`, two truecolor pixels per cell). Standard **B3/S23**,
  synchronous, **no wraparound**, and fully deterministic: the symmetric soup is a fixed
  coordinate hash (never `math/rand` or a wall clock), `Update(tick)` is idempotent (it advances
  frame by frame and only moves forward, so a repeated tick never double-steps), and the
  **rebloom** is keyed on the generation count. The rebloom is what keeps it a live gallery piece:
  a symmetric soup cascades then thins, so when the disc's live **population** drops below a fill
  floor — the honest signal, since cells culled at the rim keep the raw change-count high — a
  fresh symmetric soup is seeded and **fades in over the existing embers** rather than snapping,
  so the refresh reads as a breath (`Done()` is always false — it is ambient, not a one-shot).
  `life_test.go` pins the `h×w` contract, no-panic, that every visible glyph is the half-block
  `▀`, **that the live set stays exactly symmetric across many ticks**, a blinker returning to
  itself after 2 generations, a glider returning shifted by `(1,1)` after 4, `Update` idempotence,
  determinism across independently-constructed boards, and a golden. *(Two rejected cuts taught the
  load-bearing lesson: braille made every cell a confetti speck, and ember-coloured soup was still
  formless noise — the fix was not a better palette but symmetry, which gives the field the focal
  point and ornament that beauty requires.)* It leaves one residual gap for a future pick: a
  **`Done()` that returns `true`** — a one-shot resolving into a fixed end-state (a wipe, a
  typewriter) — which Life, being an ambient loop, never exercises.
- **`examples/bust`** — the worked example of **"silkscreen the subject, cycle the palette"**
  (`references/palette-cycle-kit.md`, `references/tools.md` §Baking): a classical marble bust under
  a slow, hypnotic color wash — one bust fills the pane and a smooth, continuous diagonal tint
  gradient rakes across it while the palette drifts around the hue wheel forever. It is the answer
  to *why* a "realistic" bust kept falling flat: a terminal is bad at subtle and spectacular at
  bold, so at half-block resolution the marble's gentle gradients collapse into banded mud, while
  flat saturated color is what truecolor half-blocks do best. So the bust is no longer rendered
  accurately — it is screenprinted. At author time `clean.py` (pure Pillow) mattes the **whole**
  bust off its watermarked white field (flooding only near-pure white so lit marble that touches
  the frame isn't eaten, keeping every component above a size floor so a highlight can't split off
  the body), then bakes a compact **luminance + alpha** asset (`bust_lum.png`, ~25 KB): the marble's
  narrow tonal range is contrast-stretched to fill the ramp and the stock watermark de-speckled so
  it can't surface under posterization. `Frame(w, h, tick)` fits the bust **once** across the whole
  pane (one continuous head), posterizes its luminance into four flat bands, and maps each band —
  and the silhouette's background — to one of nine **analogous** colorways whose base hue steps
  evenly around the wheel. The colorway is a **continuous function of screen position**: a looping
  phase advances the whole set, and a smooth diagonal term (`hueSweep`) offsets it per pixel, so a
  gentle tint gradient rakes the image with **no zones and no hard edges**; the crossfade is
  **hue-aware** (HSV along the shorter arc) so it drifts through clean vivid hues instead of
  desaturating to gray. A long `period` and a modest `hueSweep` are the "hypnotic, not
  seizure-inducing" knobs — temporal speed and spatial spread — that keep the wash a slow, mellow
  breath. (The palette went through two rejected cuts: a wall of nine *clashing* tiled busts, then
  one bust under a 3×3 *color-zone overlay* whose hard edges read as an artifact once the hues were
  analogous — hence the fully continuous field.) All the motion is color and the geometry never
  moves; at every pixel the colorway index advances by exactly `len(colorways)` per period, so the
  loop is byte-identical at the seam. Pure, offline, deterministic. The watermarked source is never
  committed. `bust_test.go` pins the `h×w` contract, no-panic, determinism, the seam, live motion, a
  golden, an asset-integrity guard (against matte amputation), and a color-field guard (many
  distinct colors + at least one saturated ink).
- **`examples/saucer`** — a fourth reference animation, and the first to **compose with the
  ecosystem** rather than reimplement everything in stdlib maths. A hand-authored subject — a
  cartoon flying saucer — drifts across a background night sky generated by the
  **[fresco](https://github.com/ZviBaratz/fresco)** provider (`Render(w, h, tick, Aurora)`),
  dimmed and deepened into a starry night, coupled so the two read as one scene. It answers a fair
  criticism — that the plugin *documented* combining tools (`references/tools.md`) far more than it
  *demonstrated* it.

  Its one idea past a generic "field + particles" is a **subject that belongs to the scene**: the
  saucer isn't pasted on top — it **lights the sky beneath it** (a glow halo) and on some passes
  lowers a **tractor beam** the aurora and stars show through, so the subject drives the field. A
  sparse field of **twinkling stars** gives the night depth, and each pass is deterministically
  **varied** (height, speed, direction, whether it beams) with long quiet gaps, so a flyby is an
  event, not a metronome. It also shows the general shape of building on **any** provider whose
  public surface is rendered output — **parse the ANSI back to a cell grid, composite, re-emit**
  (`parseSky`) — and stays a pure, deterministic `Frame` *despite a moving subject*: the saucer's
  position is a closed form of `tick` (`flightAt`), fresco's `Profile` is pinned to `TrueColor`,
  and every other layer is hash-driven; free-running like `plasma`. Tests carry *teeth* on the
  composition: `TestSkyLayerPresent` (the provider really lit the sky), `TestSaucerComesAndGoes`
  (a flyby, not a fixture, and passes vary), `TestSaucerPaints` (the hull's block glyph appears
  only when the saucer is on screen), and `TestStars` (the star lattice is sparse and twinkles).
  It is the first example module with a dependency, so it ships a `go.sum`.

  *(This example began as a fresco **galaxy** with a coupled particle layer and was reworked when
  it read as mud: a galaxy is a smooth, low-contrast subject, the worst case for a coarse glyph
  grid. The load-bearing lesson — now in `craft.md` — is **choose the subject to the medium**: a
  cell grid wants bold contrast and motion, so a directional aurora and a crisp cartoon sprite
  read where a soft gradient never could.)*
- **`examples/torus`** — a third reference animation, and the first to demonstrate the
  **top rung of the resolution ladder**: a pure, deterministic **braille** 3D wireframe
  torus that tumbles about two axes, removes its own hidden lines with a per-dot depth
  buffer plus an analytic back-face cull, and closes on itself with no seam. Braille
  cells are monochrome, so it is the clean worked example of splitting the two
  brightness channels — the **dot mask carries geometry, colour carries depth** — with
  an iridescent cyan→magenta depth ramp, a Lambert `N·L` term on the analytic torus
  normal, and a dim backdrop wash painted as the cell *background* (the only way to
  layer a colour field under a monochrome glyph in the same cell).

  It also documents a trap worth knowing: **a torus tumbled on integer harmonics about
  two coordinate axes secretly repeats at `period/2`**, because the accumulated rotation
  there is a product of π-rotations about coordinate axes and every one of those is a
  symmetry of the torus. A fixed oblique pre-tilt breaks the degeneracy, and
  `TestPeriodIsMinimal` pins it — comparing the SGR-stripped dot grid (the wash varies
  with θ and would mask the bug) and requiring a large *fraction* of cells to differ
  (`sin(π)` is `1.22e-16`, so the degenerate case still differs in ~4% of dots).
- **`scripts/record-headless.sh`** — a vhs-free beauty gate. It runs a `frames` dump,
  splits it on the `--- frame N ---` headers, rasterizes each frame through `ansi2png.py`,
  and encodes a 256-colour seamless-loop GIF (motion-stable Bayer dither) plus a truecolor
  H.264 MP4, using only `ffmpeg` + `python3`. Pairs with the scaffold's strided `frames`
  mode: make `frames × stride` span one loop `period` and the GIF loops with no ping-pong.
- **`scripts/harness/` + `scripts/harness.sh`** — a browser harness for the *looking* loop.
  It compiles an animation to WASM and drives `Frame(w, h, tick)` from a static page, so tick
  and pane size become controls: scrub to any frame with no rebuild, drag the pane to check the
  resolution ladder without resizing a real terminal, and put tick beside tick+`period` to watch
  the loop seam while you tune rather than as an assertion that goes red afterwards. Needs only
  `go` + `python3` — no node, no bundler, no terminal emulator. The frame subset is one
  `ESC[38;2;…;48;2;…m` + glyph per cell, so the page decodes to a packed cell buffer and paints
  `ImageData` runs directly (marshaling the ~360KB frame string across the JS boundary, or
  drawing per-cell via the 2D context, both cost far more). `main.go.tmpl` follows the
  `cmd/preview` scaffold convention — copy the directory, wire `render()`, same two shapes — and
  `examples/nebula` + `examples/plasma` each ship a `cmd/wasm`. This is a looking tool, not a
  gate: `record-headless.sh` still owns the artifact and `ansi2png.py` the headless colour check.
  Build outputs (`/web/*.wasm`, `/web/wasm_exec.js`) are gitignored — `wasm_exec.js` must match
  the toolchain that built the module, so it is never safe to commit.
- **`.github/workflows/pages.yml`** — the repo's first workflow: builds every
  `examples/*/cmd/wasm` and publishes `web/` to GitHub Pages on push to `main`. Pull requests
  build without deploying, so a broken WASM build is caught pre-merge without overwriting the
  live site. The build step exists because `web/` is source-only by design (see above). It also
  writes `web/animations.json`, a manifest of what was built; the page turns that into an
  animation picker, so a hosted visitor can find animations other than the default instead of
  having to guess `?anim=` names. `scripts/harness.sh` writes the same manifest locally, and the
  picker stays hidden when only one animation is built. Requires a one-time repo setting:
  **Settings → Pages → Source = "GitHub Actions"**.
- **`.github/workflows/ci.yml`** — the repo had test suites but nothing running them. `pages.yml`
  builds every animation on PRs, so a *compile* break was caught; a passing build with failing
  tests was not, and the suites only ran when someone remembered to run them by hand. This adds
  two jobs: `go` runs `gofmt -l`, `go vet` and `go test -race` in each `examples/*` module, and
  `scripts` runs `ansi2png_test.py`. Modules are discovered rather than listed, so a new example
  is covered the moment it lands, and the discovered count is asserted non-zero — a glob that
  matched nothing would otherwise make the job pass vacuously.
- **A gallery (`web/index.html`) and a viewer (`web/view.html`), split by what each has to be
  good at.** The page a visitor landed on was the authoring harness: two control bars over a
  small canvas, ~60% empty gutter, no type hierarchy. The gallery now ships **zero WASM**, is
  mobile-first and indexable, and presents the resolution ladder *including its gaps* — rungs
  2, 3 and 4 have nothing on them and say so. The ladder is a *view* of one label dimension:
  an animation carries a list of `resolutions` and lists under every rung it uses, so a piece
  that combines rungs is not forced to pick one, and a resolution off the ladder gets an
  *unclassified* row rather than silently vanishing. Each rung's sample is **drawn** through
  `web/glyphs.js` at that rung's real sub-cell geometry, because no font has sextant, octant or
  braille coverage (checked, not assumed) and a typeset ladder would have been three rows of
  tofu. The viewer is full-bleed and borderless — `craft.md` asks that a piece read as "a window
  onto something larger, not a lit box" — and the authoring harness rides along with it behind
  `?dev`, so what you tune is what a visitor sees, to the pixel. Also closes two long-standing
  gaps: `prefers-reduced-motion` and small screens now hold the ~2MB module back entirely rather
  than merely pausing, leaving the poster on screen.
- **`web/glyphs.js` + `web/glyphs.test.js`** — the sub-cell geometry model, extracted as pure
  integer tables with no DOM, shared by the painter, the gallery samples and the tests, and
  pinned against `scripts/ansi2png.py`.
- **`web/ladder.js` + `web/ladder.test.js`** — the resolution ladder as a pure, DOM-free label
  dimension: the rung table, the many-to-many grouping (one animation under every rung it uses),
  and the viewer's caption names, shared by the gallery and the viewer and tested so an off-ladder
  resolution cannot silently drop an animation.
- **`examples/torus/cmd/wasm`**, so the top rung of the ladder is no longer the one animation the
  browser cannot open.
- **`examples/bust/cmd/wasm`, and a `meta.json` for `bust` and `saucer`** — so every example now
  both opens in the browser and carries the metadata the gallery reads, making the "`meta.json` per
  animation" below literally true. `bust` shipped only a terminal `cmd/preview`, so the Pages
  build — keyed on the `.wasm` modules that compile — never saw it and it fell off the manifest
  silently; `saucer` opened but rode the bare defaults (its name as title, rung 1, no blurb or
  accent). Each meta now carries a title, blurb, `resolutions`, accent and loop shape, plus a
  hand-picked `posterTick` for the still shown before the module loads: `bust` at its violet
  vaporwave attitude (tick 450, accent `#8469DB` — the wash's own violet ink), `saucer` at a lit
  flyby that reads in both the landscape and portrait poster panes (tick 260, accent `#7DCFFF` from
  its aurora palette, which is also the saucer's dome glass).
- **`scripts/manifest.py` and `scripts/posters.sh`**, plus a `meta.json` per animation — title,
  blurb, `resolutions`, accent and loop shape, merged into `animations.json` at build time, and
  still-frame posters rendered through `ansi2png.py` so the gallery needs neither WASM nor a
  running animation to show what something looks like.
- **Self-hosted type: Departure Mono + JetBrains Mono**, both OFL 1.1 with no Reserved Font Name
  (verified in the bundled licences, not the project pages), subsetted to 37KB total.
- **`scripts/ansi2png.py --stats`** — a numeric read of a frame: luminance histogram, the
  fraction that is dark, and how the lit pixels spread across the hue wheel, printed to
  *stderr* so stdout still carries the PNG. Judging by eye stays the rule; this is for the
  question the eye is bad at — *"it looks flat and I can't say why."* The diagnosis it
  exists to surface is **most pixels in the bottom luminance bin with the lit ones piled
  into one or two hue buckets**, which means the designed ramp is never being *reached*: a
  problem in the field feeding the palette, not in the palette, and one that no amount of
  re-picking colour stops can fix. `examples/life` measured at 76% of its pixels below
  luminance 16 and 19% of lit pixels orange against 4% violet — a seven-stop ramp rendering
  as orange-on-black — which is what aimed its rework at the field instead of at the stops.

### Changed

- **The craft rubric learns spatial diffusion, ramp mechanics, and when to measure**, all
  from building `examples/life`:
  - `craft.md` gains **"A per-cell field is a mosaic until you diffuse it"** — the rule that
    a field sampled per cell (a sim grid, particles, a Life board) jumps in colour at every
    cell boundary however good the palette is, and the fix is spatial: blur it into a halo
    and composite back with a **screen blend** (a clamped sum blows the core to a white
    disc), blurring the *scalar field* rather than the finished RGB, with a sliding-window
    box blur so the cost is O(1) per pixel. Includes the trap that a blur of a *symmetric*
    field needs an **odd-sided grid**, or edge taps differ between the halves and the light
    tilts while the cells stay perfectly symmetric.
  - `craft.md` Composition gains **fade the ramp coordinate, not the colour** — scaling the
    palette's input rather than multiplying its output turns a vignette from a dimming into
    a colour *arc*, which is where a ramp's cold stops finally get shown.
  - `techniques.md` extends the OKLab section past picking stops to **interpolating** in
    OKLab, smoothstepping each segment for C1 continuity, and **baking the ramp to a LUT** —
    not an optimization but what makes it affordable: per pixel the conversion measured at
    ~3× an entire frame budget, while the baked table came out *cheaper* than the sRGB lerp
    it replaced. Generalized to: any scalar → colour function is LUT-able, and per-pixel
    transcendentals are the first thing to hunt when a field animation misses its budget.
  - `craft.md` §"Tune by looking" gains **measure when looking isn't finding it**, and
    §"The tuning loop" gains **show candidates when the author is the judge** — a subjective
    revision ("nicer colours") is a question, so render two to four labelled directions and
    let them choose instead of spending a build on one guess.
  - `effects.md` notes that a *random-soup* Life is intrinsically noise while its
    **isotropic rules preserve a symmetric seed for free**, and adds bloom to the combining
    moves. `agents/tuner.md` gains a measure-first step and cites it as the evidence for
    calling a fault out of a tuner's reach. `SKILL.md` picks up the two new red flags.
- **`scripts/record-headless.sh`** documents the GIF size model: file size tracks *source
  pixels × frames*, a field of continuous gradients costs several times more per pixel than
  a sparse one (adding a glow can double the GIF), and `--width` is a trap — it only matches
  the raster when it equals `pane_width × --cw`, and any other value rescales, invents
  intermediate colours, and can leave the file *bigger* than the width being shrunk from.

- **The skill now makes the vision *load-bearing* — it is graded, not just elicited.** The
  Brief's "one special idea" used to be stated once and then abandoned, so a build could pass
  every test and still miss what was asked (a matted still panned in an ellipse, no light, no
  atmosphere). Now `SKILL.md` §1 captures a **Vision Card** (subject · motion *verb* · light ·
  atmosphere · palette · the one idea) as a durable artifact, and the beauty gate (§Tune,
  `craft.md`, `agents/tuner.md`) grades the result against it slot by slot — and may **fail a
  merely-competent piece.** New craft: `craft.md` §"Making a subject move in 3D" (pseudo-3D
  turn, parallax, relighting sweep, atmospheric depth — the vocabulary that beats a pan) and
  the red flag that two quarter-phase sinusoids are an ellipse, not motion.
- **The skill gained *two* real subject techniques, as paste-ready machinery.** "Layer a field
  behind the subject" and "screenprint a subject" are no longer slogans:
  `references/atmosphere-kit.md` composites a **baked subject (with alpha) over a
  runtime-synthesized scene** (moving light, drifting `fbm` mist, lit backdrop, rim), and the
  new `references/palette-cycle-kit.md` **silkscreens a subject** — bake a luminance + alpha
  matte, posterize it into flat bands at run time, and recolor the bands through curated
  colorways with a hue-aware, seamless-looping crossfade (the pattern `examples/bust` now
  embodies). `tools.md` §Baking was rewritten to headline **two** baking patterns beside the
  RED bake-a-finished-picture pan — *composite over a live scene* and *bake a luminance+alpha
  matte, do the color at run time* — each with the **subject-integrity check** against matte
  amputation. `craft.md` / `effects.md` gain the load-bearing lesson that in a terminal **bold
  flat color beats subtle gradient realism** — when a realistic subject underwhelms, silkscreen
  it — and `techniques.md` notes routing a *photographic* subject (posterize/duotone, or widen
  + light-as-sharpness; the sixel/kitty tier as a real option). The `tuner` is told its reach
  ends at sweeping constants — a missing layer or wrong treatment is an author fix.
- **`examples/torus` now uses the palette it was designed around: `shadeGamma` 2.2 → 1.3.**
  (Shade median 0.24 → 0.43; the *rendered* gain is smaller than that ratio because the
  palette is non-linear — mean lit luminance in the demo recording rises 43 → 51.) The old
  value was justified by a note claiming the raw shade "piles up at 0.6–0.9" and would
  spend the whole palette in its magenta band. Measured over 16 frames at matched phase
  at 100×28, that does not reproduce — the raw shade is already well spread (p10 0.17,
  median 0.52, p90 0.84). Applying 2.2 crushed the median to **0.24**, left only **10.8%**
  of lit cells reaching violet and **none** reaching the pink-white top of the ramp, so
  the designed iridescent palette rendered as near-uniform dark indigo. Because braille
  dots are separated marks, dim low-contrast dots stop grouping into lines — this read as
  "the wires look dotty" and sent an investigation after a rasterization bug that did not
  exist. At 1.3 the median is 0.43 and 23.2% reach violet.

- **Braille's font dependency is now documented on the tier, not just in one example.**
  `techniques.md` and `examples/torus/README.md` gain the two traps that make a live
  braille gate untrustworthy: many popular monospace fonts (MesloLGS NF, JetBrains Mono,
  DejaVu Sans **Mono**) **contain no braille at all** and fall back silently to
  proportional DejaVu Sans; and even a braille-carrying font has a **line box taller than
  four dot rows**, which puts a screen-locked blank band every 4 dot rows that moving art
  drifts through as apparent jitter. Counter-intuitively, the tighter a font's dots the
  worse the banding — Cascadia Mono's seam is 5.4× its intra-cell gap versus DejaVu's
  1.4× — because shrinking the gap inside a cell does nothing to the cell height. Both
  docs carry measured geometry tables and a per-launch Alacritty test command. Note
  `fc-list ':charset=2800:spacing=100'` is **not** a valid check: Iosevka has braille and
  is tagged `spacing=90`.

- **A false solidity claim is retracted from `examples/torus/README.md`.** It asserted the
  wires were contiguous because "consecutive samples land within one dot 99.7% of the
  time" — that is Chebyshev distance ≤ 1, which counts a *diagonal* step as connected even
  though no font draws corner-touching dots as joined, so the metric could not see the
  defect it was cited to rule out. It also had no test behind it. Measured along each
  wire's run of distinct dots: **88.3% orthogonal, 11.7% diagonal, 0.00% gaps** at 100×28.
  Genuine breaks are absent; 4-connected rasterization was measured and rejected.

- **`ansi2png.py` rasterizes glyph ink coverage, so a density ramp reads as a ramp.**
  Every printable glyph was painted as a flat block of its foreground colour, so `·` and
  `@` rasterized identically. For an engine that splits brightness between glyph density
  and colour luminance — fresco's `lumRange`, the two-channel split `craft.md` teaches —
  that put half the signal in a channel the gate could not see. It was worse than blind:
  it **ranked the sweep backwards.** Mean pixel brightness over a tunnel field at
  `lumRange` 0 / 0.5 / 0.75 / 1 measured 153 / 104 / 88 / 76 — monotonically *decreasing*,
  because at `lumRange` 0 density carries all the brightness, nearly every cell holds some
  glyph, and painting each as a full block renders a vivid full-bleed field where the
  terminal truth is a faint dust of `·` and `:`. An author sweeping the PNG and picking
  the image with the most presence was steered to the setting that dots the field out.
  The same sweep now reads 28 / 32 / 36 / 55, in the right order. Shade and typographic
  glyphs blend over the background at an approximate coverage (`INK`), judged by eye at
  terminal proportions — the ordering along a ramp has to be right, not the third decimal.
  A printable glyph with no `INK` entry still falls back to a solid foreground block, so
  labels, box drawing, unknown scripts and the sextant tier stay visible rather than
  vanishing into an invented coverage. **On by default**, not behind a flag: a correctness
  gate whose correctness depends on remembering a flag is a footgun. That does change
  output for text — a typewriter's letters now render blended rather than solid.

- **The resolution ladder now states which rungs the headless gate can actually see.**
  `techniques.md`'s ladder table gains a **Headless gate** column, because the skill
  otherwise pushes authors *up* the ladder into a blind spot: `record-headless.sh` builds
  the GIF/MP4 through `ansi2png.py`, so choosing a rung it cannot resolve means no
  headless gate and no demo recording on a box without `vhs`/`ttyd`. Half-block, quadrant
  and braille resolve; **sextant collapses to its foreground; octant is worse — the cell
  is dropped entirely.** Octants need Unicode 16 (Sept 2024), so a Python with an older
  `unicodedata` (3.10 ships UCD 13) reports them non-printable and the parse loop emits no
  cell, shearing every row that contains one (a 5-cell row rasterizes to 4). That fails
  silently and structurally rather than visibly, so it is called out explicitly.
- **`techniques.md` documents the braille bit order**, which is irregular: the numbering
  is column-major for the historic 6-dot cell and only then appends dots 7/8 as a bottom
  row, so the obvious `1 << (row*2 + col)` is wrong on three of the four rows. A wrong
  mapping still renders something plausible, so it fails silently — the table plus its
  spot checks (`⠁ ⡀ ⢀ ⡇ ⣿`) are there to be pinned in a unit test. Also notes that braille
  dots are near-square and both axes must ride one scale factor.
- **`craft.md`: a closed seam does not prove `period` is the *true* period.** Rotating a
  symmetric subject (torus, sphere, cube, polyhedron) by integer harmonics lands back on a
  symmetry of the object at `period/2`, so the loop silently closes early and the recording
  wastes half its frames on a repeat. Adds the minimal-period test and the two subtleties
  that give it teeth: compare the SGR-stripped glyph grid (another θ-varying layer would
  otherwise satisfy the assertion on its own), and require a large *fraction* of cells to
  differ (`sin(π)` is `1.22e-16`, so exact `!=` passes even when fully degenerate).
- **`SKILL.md`: mutate the thing under test and watch the test fail.** An animation is
  mostly float thresholds and near-symmetries, so a plausible assertion passes for the
  wrong reason far more often than in ordinary code; a test that survives its feature
  being removed reads as coverage while providing none. If a property resists a test with
  teeth, say so where the test would be and check it at the beauty gate.
- **`scripts/README.md`: detail tiers must record 1:1.** `--width` rescaling is harmless
  for a smooth field but destroys line art on a sub-cell tier, where the artwork *is* the
  dot pattern — match `pane × cell-size` to `--width` and choose a deliberately small pane,
  inverting the "fill the terminal" sharpness lever that applies to fields.
- **`ansi2png.py` now renders braille (U+2800–28FF) as its 2×4 dot grid** instead of
  collapsing the cell to a solid foreground block. Lit dots take the foreground colour,
  unlit dots the background, and `U+2800` (the braille blank) is real negative space —
  it is `isprintable()`, so it previously fell through to a solid *foreground* cell. So
  braille line art (wireframes, plots, edges, high-detail silhouettes) now reads as line
  art in the headless PNG gate and in the `record-headless.sh` GIF/MP4, not as a field of
  solid rectangles. Dots fill their sub-rectangle rather than being drawn inset, which
  keeps them legible after the GIF's downscale and dither. Internally the quadrant path
  is generalized to a `rows × cols` sub-cell mask that braille reuses; region edges sit
  at `k·size//n`, so the regions tile any cell size exactly — the half-block, quadrant
  and full-block output is **byte-identical**, verified against the previous version at
  eight cell sizes including odd ones, so `docs/plasma.gif`, `docs/nebula.gif` and
  `docs/nebula.mp4` are unchanged and need no regeneration. **Sextant (U+1FB00–1FB3B)
  collapses to its foreground and octant (U+1CD00–1CDE5) is dropped entirely** — judge
  those two tiers on a real terminal or the GIF gate. The stale caveats in `ansi2png.py`'s
  docstring, `scripts/README.md` and `references/tools.md` are corrected accordingly.
- **`cmd/preview` scaffold is now a directory** — `scripts/preview.go.tmpl` becomes
  `scripts/preview/` (`main.go.tmpl` + build-tagged `size_unix.go` / `size_other.go`),
  copied as a unit. The live loop now **fills the whole terminal and reflows on resize**
  (was a fixed 80×24 corner) via a zero-dependency `TIOCGWINSZ` ioctl, and uses the alt
  screen so it exits cleanly. `frames` gains a stride and optional pane size —
  `frames N [stride] [w h]` — so a slow forever-loop shows real motion in the headless
  gate, and a big field can be rendered for `ansi2png.py`. **Note the third positional
  arg is now `stride`, not width**: the old `frames N W H` is now `frames N stride W H`,
  and a pane size is only applied when both `W` and `H` are given. Both reference previews
  were regenerated from the new scaffold.
- `examples/plasma` adopts the allocation-free `strconv` per-cell render path (the
  `appendCell`/`writeChan` helpers, matching `examples/nebula`) — byte-identical output,
  so the golden is unchanged; the scaffold now models the fast path in both examples.
- **Loop-seam is now a first-class convention.** `skills/author-animation` (SKILL.md +
  `craft.md`) distinguishes a **free-running** field (linear time, never exactly repeats)
  from a **seamless loop** (`θ = 2π·(tick mod period)/period`, byte-identical at the seam),
  and the standalone test checklist adds `TestLoopSeam` for true loops. `examples/plasma`
  is relabelled free-running (its demo GIF is ping-ponged); no code or golden change.
- **Reference corrections.** `craft.md` no longer over-claims goldens are byte-portable
  across machines — they are same-machine, and the portable guarantees are shape / no-panic
  / seam. `techniques.md` now sets half-block resolution expectations: a field is `w × 2h`
  pixels, one pixel per column, so **filling the terminal** is the sharpness lever (and
  quadrant/sextant sharpen edges, not smooth gradients).
- `scripts/preview.sh`: point fresco-variant authors at the dedicated preview
  program `new-variant` has them write (it selects the variant and sweeps
  `LumRange`) instead of `cmd/fresco-demo`, which only cycles the shipped roster on
  a timer — a final look, not a per-variant tuning knob.
- **A new `animPeriod(w, h)` global replaces the loop length the page used to hardcode.**
  `index.html` carried a nebula-shaped `1080` in two places, which was wrong for anything else.
  It takes the pane because `torus.Period(w, h)` scales with it, and `0` now means free-running —
  which turns plasma's compare pane from a confusing non-result into a stated property: the
  viewer disables Δ and says why, instead of letting you hunt for a match that cannot exist.

### Fixed

- **The first frame of a `frames` dump was a lie.** A stateful `Animation` owns its
  dimensions and re-seeds when `View` is asked for a pane it was not constructed at, and
  `scripts/preview/main.go.tmpl` wires `render()` over an animation built at a fixed size —
  so the first frame of any dump at a *different* size was whatever that re-seed had just
  produced, not the tick asked for. It failed in the worst way available: silently, and
  identically for every input, so a **one-frame dump rendered the seed at every tick** and a
  sweep of a taste constant came out byte-identical at every value — which reads as "this
  knob does nothing" rather than as a bug. A discarded warm-up render now moves the re-seed
  ahead of the dump (a pure `Frame` carries no state, so for one it is only a wasted call).
  Fixed in the template and in `examples/life`, the only stateful example.

- **The "nothing to publish" guard in `pages.yml` could never fire.** Without `shopt -s nullglob`,
  an unmatched bash glob expands to its own literal text, so `for dir in examples/*/cmd/wasm`
  runs once with a bogus path and dies on `cd examples/*` under `set -e` — exiting before the
  `${#names[@]} -eq 0` check is ever evaluated. The job still failed, just on a confusing `cd`
  error rather than the intended "nothing to publish" one. Both workflows now set `nullglob`.
- **The browser painter did not match `ansi2png.py`, in two ways that predate braille.**
  `ansi2png.py`'s `QUAD` models 15 glyphs; the page modelled 9 — `▙▚▛▜▞▟` cannot be expressed as
  one `[x, y, w, h]` rect, so they fell through to `ctx.fillText`, the ~70ms/frame path the
  ImageData rewrite exists to escape. Separately, the truncate/`Math.ceil` sub-cell rule
  disagreed with the PNG by one pixel at odd cell sizes, against what `ansi2png_test.py` pins.
  Both were invisible because nebula and plasma emit only `▀` — a *vertical* split, where the
  cell width never mattered. Fixed by extracting the model into `web/glyphs.js` and replacing
  the rect lookup with a mask + row-band decomposition shared by both tiers; geometry now
  follows `_imap` exactly. On a synthetic frame of all 15 quadrants plus 15 braille patterns at
  `cw=7`/`ch=14` — odd, the discriminating case, since at even widths a correct and an incorrect
  split look identical — the new painter differs from the export in **0 of 2940** pixels, the
  old one in **1018**, across 24 of 30 cells.
- **The compare pane silently ignored a typed Δ.** The generic `input` listener ran
  `readControls()`, which re-derives Δ, *before* the flag marking Δ hand-edited was set, so a
  typed value reverted inside the same event. This made the seam check pass vacuously: Δ=720
  reported "seamless" and so did Δ=719, which should be impossible. Δ is now on its own listener.
- **`scripts/posters.sh` rendered tick 1 whenever a poster tick was 0.** `cmd/preview` accepts a
  stride only when it is `> 0`, so `frames 2 0` silently left the stride at 1 and the
  keep-the-second-frame pipeline returned tick 1. Latent for the shipped animations, which all
  declare a non-zero `posterTick`, but it would have fired for any animation with no `meta.json`
  — a case `manifest.py` explicitly supports. Tick 0 now asks for a single frame instead.

## [0.1.0] - 2026-07-18

Initial release: an expert terminal-animation authoring skill, its tuning harness, and a
bundled reference animation.

### Added

- **`author-animation` skill** — a 5-stage process (Brief → Select → Compose → Build →
  Tune) that interrogates the vision, composes technique and style past the conventional
  default, builds to a testable convention, and finishes at a beauty gate.
- **Reference library** — `craft.md` (the motion/beauty rubric), `techniques.md` (the
  sub-cell resolution ladder, colour depth, dithering), `effects.md` (the demoscene
  catalog as springboards to combine), and `tools.md` (providers and build-time tools,
  with fresco as one provider among them).
- **Standalone convention (§B)** — a small, framework-free `Frame(w, h, tick)` /
  `Animation` shape that stays deterministic and snapshot-testable, plus a `cmd/preview`
  scaffold (`scripts/preview.go.tmpl`).
- **fresco hand-off (§A)** — an instruction-level hand-off to fresco's `new-variant`
  skill for field variants authored inside the fresco repo.
- **Tuning harness** (`scripts/`) — `preview.sh` (live), `record.sh` (the vhs GIF beauty
  gate), `preview.go.tmpl` (preview + frame-dumper scaffold; restores the cursor on
  Ctrl-C), and `ansi2png.py` (a stdlib headless colour gate that rasterizes frames to a
  PNG, resolving half-block / quadrant / full-block sub-cell glyphs).
- **`tuner` subagent** — drives the render → look → tune loop.
- **Reference animation** (`examples/plasma/`) — a pure, deterministic half-block
  truecolor plasma with a designed palette, an orbiting focus, and an edge vignette;
  includes bounds/determinism/golden tests and `docs/plasma.gif`.
- Plugin and marketplace manifests; bundled files are referenced via
  `${CLAUDE_PLUGIN_ROOT}` so the harness resolves once installed.

[Unreleased]: https://github.com/ZviBaratz/terminal-animations/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/ZviBaratz/terminal-animations/releases/tag/v0.1.0
