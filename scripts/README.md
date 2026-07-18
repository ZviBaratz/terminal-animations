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
| `ansi2png.py` | Headless colour gate: rasterizes the `frames` dump into a PNG you can open/Read when there's no TTY. Stdlib Python, no deps. |

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
never from the formula. Cell size knobs: `ANSI2PNG_CW` / `ANSI2PNG_CH`.
