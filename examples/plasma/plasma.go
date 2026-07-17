// Package plasma is the reference animation for the terminal-animations plugin:
// a pure, deterministic, half-block truecolor plasma field.
//
// It exists to show the plugin's craft in one small artifact, composed past the
// conventional "one effect in flat ASCII" default:
//
//   - Fidelity tier: half blocks (▀) — every character cell carries TWO independent
//     24-bit pixels (fg = top, bg = bottom), doubling vertical resolution. This is
//     the "portable workhorse" rung from references/techniques.md.
//   - Designed palette: a cosine gradient (Iñigo Quílez form), not a functional
//     grey→white ramp. The colour rides luminance, so the field stays smooth.
//   - Composition: a classic plasma (summed sines) plus a radial ripple whose focus
//     slowly orbits, with an edge vignette so the field sits in real negative space
//     rather than meeting a hard rectangular border (references/craft.md).
//   - Determinism: a pure function of (w, h, tick) — no wall clock, no math/rand — so
//     it is snapshot-testable (see plasma_test.go's golden).
//
// The taste constants below were chosen against the beauty gate (the ansi2png.py
// filmstrip), by eye — not computed. See README.md.
package plasma

import (
	"fmt"
	"math"
	"strings"
)

// Tunable taste constants — decided by looking, not arithmetic.
const (
	speed  = 0.10 // animation time advanced per tick
	drift  = 0.05 // palette hue drift per time unit
	warp   = 12.0 // radial-ripple spatial frequency
	bands  = 0.14 // field value -> palette position (colour-band tightness)
	vigPow = 0.55 // vignette softness (smaller = softer, larger bright core)
)

// palette maps a scalar to an RGB colour in [0,1]³ via a cosine gradient:
// col(t) = 0.5 + 0.5·cos(2π(t + d)). The per-channel d offsets are the palette's
// identity — this set gives a cool indigo→magenta→gold nebula.
func palette(t float64) (float64, float64, float64) {
	const dr, dg, db = 0.00, 0.16, 0.34
	r := 0.5 + 0.5*math.Cos(2*math.Pi*(t+dr))
	g := 0.5 + 0.5*math.Cos(2*math.Pi*(t+dg))
	b := 0.5 + 0.5*math.Cos(2*math.Pi*(t+db))
	return r, g, b
}

// pixel is the field: the colour at world coordinate (u, v) and time t, already
// scaled by the vignette factor vig. (cx, cy) is the orbiting radial focus.
func pixel(u, v, t, cx, cy, vig float64) (uint8, uint8, uint8) {
	val := math.Sin(u*6.0+t) +
		math.Sin(v*6.0+t*0.8) +
		math.Sin((u+v)*5.0+t*1.3) +
		math.Sin(math.Hypot(u-cx, v-cy)*warp-t*1.7)
	r, g, b := palette(val*bands + t*drift)
	return chan8(r * vig), chan8(g * vig), chan8(b * vig)
}

// vig is the edge vignette: 1 at the centre, fading to 0 at every border, so the
// animation reads as a window onto something larger, not a lit box.
func vig(u, yn float64) float64 {
	e := math.Sin(math.Pi*u) * math.Sin(math.Pi*yn)
	if e <= 0 {
		return 0
	}
	return math.Pow(e, vigPow)
}

func chan8(x float64) uint8 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 255
	default:
		return uint8(x*255 + 0.5)
	}
}

// Frame renders frame `tick` into exactly h lines of exactly w visible cells (or ""
// for a degenerate pane). Each cell is a half block ▀: its foreground colour paints
// the upper sub-pixel, its background the lower — so the visible grid is w × 2h
// truecolor pixels. Pure in (w, h, tick).
func Frame(w, h, tick int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	W := float64(w)
	H2 := float64(2 * h) // pixel rows: two per cell
	vMax := H2 / W       // world height, keeping pixels square (circles stay round)
	t := float64(tick) * speed
	cx := 0.5 + 0.18*math.Cos(t*0.5)
	cy := vMax*0.5 + 0.10*math.Sin(t*0.37)

	var b strings.Builder
	b.Grow(w*h*22 + h*4)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			u := (float64(c) + 0.5) / W
			topPy := float64(2*r) + 0.5
			botPy := float64(2*r+1) + 0.5
			tr, tg, tb := pixel(u, topPy/W, t, cx, cy, vig(u, topPy/H2))
			br, bg, bb := pixel(u, botPy/W, t, cx, cy, vig(u, botPy/H2))
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", tr, tg, tb, br, bg, bb)
		}
		b.WriteString("\x1b[0m")
		if r < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
