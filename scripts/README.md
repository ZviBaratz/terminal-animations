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
| `harness.sh` | Builds an animation to WASM and serves the browser harness: scrub any tick, drag the pane, compare tick vs tick+period. Needs only `go` + `python3`. |
| `../web/` | The harness page itself (`index.html`, `harness.js`) at the repo root — static files, no node, no bundler. Shared by every animation; `?anim=<name>` picks the module. |

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
scripts/harness.sh examples/nebula        # → http://localhost:8731/?anim=nebula
scripts/harness.sh examples/nebula 9000   # …on a different port
```

| Control | What it answers |
|---|---|
| tick slider, `step`, `←`/`→` | What does frame N actually look like? (no rebuild) |
| `w`/`h`, `fit to window` | Does it reflow across the resolution ladder? |
| `compare` + `Δ` | Is the loop seamless? Set Δ = period; the panes must be identical. |
| `ms/frame` readout | Is `Frame` itself fast enough, separate from paint cost? |

The animation needs a `cmd/wasm` entrypoint — copy `harness/` as above. This is
a *looking* tool, not a gate: `record-headless.sh` still owns the artifact, and
`ansi2png.py` still owns the headless colour check. Note also that scrubbing
backwards is only meaningful for a pure `Frame(w, h, tick)` animation; a
stateful one can only be replayed forward.

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
never from the formula. Cell size: `--cw` / `--ch` flags (else the `ANSI2PNG_CW` /
`ANSI2PNG_CH` env vars, else 7×14):

```sh
go run ./cmd/preview frames 5 | ./scripts/ansi2png.py --cw 8 --ch 16 > /tmp/anim.png
```

Prefer the flags — an env var set before the `|` (`ANSI2PNG_CW=8 go run … | ansi2png.py`)
applies to the *producer*, not to `ansi2png`, so it is silently ignored.

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
