# The tuning harness

Two loops, straight from `references/craft.md`: a fast **inner loop** to check
structure, and a **beauty gate** that records a GIF so you watch the motion in
colour.

## Files

| File | What it is |
|---|---|
| `preview.go.tmpl` | Copy to `cmd/preview/main.go` and wire `render()` to your animation. Runs a live loop, or `frames N` to dump frames to stdout. |
| `preview.sh` | Thin wrapper that runs the preview program live (`Ctrl-C` to quit). |
| `record.sh` | The beauty gate: records a short GIF of the preview via vhs. |

## Inner loop (fast, no extra tools)

```sh
# live, in colour:
scripts/preview.sh                 # runs `go run ./cmd/preview`

# structure + headless colour check (no TTY needed):
go run ./cmd/preview frames 5      # dump 5 frames
go run ./cmd/preview frames 1 | cat -v   # see the raw SGR colour bytes
```

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

**No vhs and no live terminal (a sandbox, CI)?** The `frames` output still carries
the full colour in its SGR bytes — to *see* it, parse those escapes and rasterize
each cell to an image (a short awk/Python filter emitting PPM or PNG), then open
that. It is the headless stand-in for the GIF gate; you still judge the colour by
eye, never from the formula.
