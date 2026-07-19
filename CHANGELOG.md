# Changelog

All notable changes to the `terminal-animations` plugin are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Because the plugin's `version` is pinned in `.claude-plugin/plugin.json`, installed
> users only receive updates when that version is bumped. Bump it — and add an entry
> here — with every user-facing change.

## [Unreleased]

### Added

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

### Changed

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
