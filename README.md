# terminal-animations

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Claude Code plugin for **authoring beautiful terminal animations** — ASCII/ANSI
motion of any kind, for splash screens, loaders, demos, or just delight.

It does one thing well: when you ask for a terminal animation, it **routes the
idea to the right home** and then helps you ship it *tested and visually tuned*,
encoding the hard-won craft of terminal motion so a new animation starts good
rather than random.

## The routing rule

There is no one-size interface for terminal motion — a generative field and a
game-of-life sim want different shapes. The plugin picks:

| If the animation is… | It belongs in… |
|---|---|
| full-pane, a pure function of `(position, frame)`, palette-gradient coloured, loops forever, no subject | **a [fresco](https://github.com/ZviBaratz/fresco) field variant** |
| stateful (a sim), or has a subject/sprite, or *resolves* (a one-shot that ends), or is non-field motion | **a standalone animation** |

- **fresco variant** — a new field for fresco's roster (rain, tunnel, ripple,
  galaxy, …). When routing lands here **and you are inside the fresco repo**, the
  plugin defers to fresco's own `new-variant` skill and follows it — an
  instruction-level hand-off, never a duplicate of fresco's contract. Outside the
  fresco repo, that path isn't available, so the plugin does the standalone path.
- **standalone animation** — authored here, to a small deliberate convention (see
  the skill), with a preview loop and a golden/contract test.

## The standalone convention

Chosen from real examples, not guessed — and the deliberate seed of a possible
future library:

```go
// pure, free-running (a plasma, a starfield):
func Frame(w, h, tick int) string

// stateful or resolving (game of life, a typewriter, a wipe):
type Animation interface {
    Update(tick int)      // advance state
    View(w, h int) string // render current state to exactly h lines of w cells
    Done() bool           // true once a one-shot has resolved; always false for a loop
}
```

Plus a `cmd/preview/main.go` loop to watch it, and a test pinning the `h×w`
contract, no-panic on any `(w, h, tick)`, determinism where pure, and a golden
frame.

## What's inside

- `skills/author-animation/` — the routing skill and the standalone authoring
  path (`references/craft.md` holds the universal motion/beauty heuristics).
- `scripts/` — the tuning harness: a live preview runner, a vhs GIF recorder, and
  a frame dumper for structural checks. Needs `vhs`, `ttyd`, and `ffmpeg` for the
  GIF path.
- `agents/tuner.md` — an optional subagent that drives the render → look → tune
  loop for you.

## Install

```
/plugin marketplace add ZviBaratz/terminal-animations
/plugin install terminal-animations@terminal-animations
```

Then just ask for an animation ("a plasma splash", "a typewriter intro", "a new
fresco variant") — the `author-animation` skill triggers on its own.

## Scope

In scope: producing a tested, tuned animation plus a preview. Out of scope:
wiring it into a specific app's splash/cycling harness (a separate step), runtime
LLM generation (never put a model in a 60fps render loop), and building a
standalone animation *library/registry* now — the convention above keeps that
door open without walking through it yet.

## License

MIT © Zvi Baratz
