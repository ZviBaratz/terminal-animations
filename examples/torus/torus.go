// Package torus is a spinning 3D wireframe torus for the terminal: a pure,
// deterministic, braille-rendered object that tumbles about two axes and closes
// back on itself with no seam.
//
// It is the field-shaped standalone from references/… (skill §B) — a pure function
// of (w, h, tick) — composed well past the conventional "one effect in flat ASCII"
// default, and it is this repo's worked example of the TOP rung of the resolution
// ladder:
//
//   - Fidelity tier: braille (U+2800–28FF). Every character cell carries a 2×4 grid
//     of individually addressable dots, so the visible grid is w×2 by h×4 — the
//     finest *monochrome* detail rung from references/techniques.md. That reference
//     argues the rungs above half-block buy sharper hard *edges* rather than smoother
//     colour, which is exactly why a wireframe belongs here and a smooth colour field
//     does not.
//   - The two brightness channels, split cleanly (references/craft.md): a braille cell
//     is monochrome — eight dots share one foreground colour — so the DOT MASK carries
//     pure geometry and COLOUR carries all the brightness. Dimming a wire by dropping
//     dots would break thin lines into noise; it is never done here.
//   - Hidden-line removal. The opaque torus surface is rasterized into a per-dot depth
//     buffer that is never drawn; wires are kept only where they are in front of it.
//     Without this a wireframe torus is famously ambiguous (the Necker-cube effect)
//     and its spin direction visually flips. This is the per-cell z-buffer of Sloane's
//     donut.c (references/effects.md) promoted to per-dot resolution by the tier.
//   - Depth read as hue, not just brightness: a designed iridescent palette runs deep
//     cyan-blue (far) → indigo → violet → magenta → hot pink-white (near), blended with
//     a Lambert N·L term on the analytic torus normal so the wireframe reads as a lit
//     solid rather than a flat tangle of curves.
//   - Composition: a dim backdrop wash painted as the cell BACKGROUND — the one way to
//     layer a smooth colour field under a monochrome braille glyph in the same cell —
//     times an edge vignette, so the splash reads as a window onto something larger
//     and not a lit rectangle. The wash is a smooth dim gradient, so it gets
//     motion-stable screen-locked Bayer dithering; the wires deliberately do not
//     (dithering line art only makes it dashed).
//   - A truly seamless forever-loop: every time-varying term flows through a single
//     phase θ = 2π·(tick mod P)/P at an INTEGER harmonic, where P is Period(w,h), so
//     Frame(w,h,0) and Frame(w,h,P) receive identical inputs and are byte-identical
//     (TestLoopSeam). P is a function of the pane, not a constant — see Period for why
//     a fixed angular rate flickers on a large pane and how the loop stretches to fix
//     it. At the 100×28 reference pane P is 720, so the recording recipe in README.md
//     still spans exactly one loop.
//     Note tiltY: a torus tumbled by integer harmonics about two coordinate axes
//     secretly repeats at period/2, because at that point the accumulated matrix is a
//     product of π-rotations about coordinate axes and every one of those is a symmetry
//     of the torus. The fixed oblique pre-tilt breaks that degeneracy — it is load
//     bearing, not decoration, and TestPeriodIsMinimal pins it.
//
// The taste constants below were chosen against the beauty gate (the ansi2png.py
// filmstrip), by eye — not computed. See README.md.
package torus

import (
	"math"
	"strconv"
	"strings"
)

// Tunable taste constants — decided by looking, not arithmetic.
const (
	// basePeriod is the loop length AT refSpan (24 s at 30 fps — unhurried). The loop
	// a given pane actually runs is Period(w, h), which stretches this so that motion
	// per frame stays constant in DOT space rather than in angle. See Period.
	basePeriod = 720
	refSpan    = 112 // min(2*100, 4*28) — the 100×28 pane every constant was tuned at

	spinU = 1   // θx harmonic — one full tumble about x per loop
	spinV = 2   // θy harmonic — two about y; the 1:2 ratio is what makes it tumble
	tiltY = 0.6 // fixed oblique pre-tilt (rad) — breaks the period/2 symmetry (see above)

	bigR   = 2.0 // torus major radius (the ring)
	smallR = 0.8 // torus minor radius (the tube) — larger = fatter, less hole

	persp   = 6.0  // eye distance; smaller = stronger perspective, more depth drama
	fitFrac = 0.86 // fraction of the short axis the torus spans — the breathing margin

	ringsU      = 12  // meridian rings (around the tube) — structure vs. moiré
	ringsV      = 22  // longitude rings (around the ring) — structure vs. moiré
	wireDensity = 2.2 // wire samples per dot of arc — >1 keeps lines unbroken
	surfDensity = 1.6 // occluder samples per dot of arc — >1 keeps the surface pinhole-free
	minSteps    = 8   // floor on any sample count, so tiny panes still form a shape
	maxSteps    = 900 // ceiling, so a huge pane cannot blow the frame budget

	depthBias = 0.40 // world-space slack when testing a wire against the surface it
	// lies on. A wire sits exactly ON the occluder, so its depth error
	// grows with surface obliquity — the classic shadow-map acne shape.
	// 0.18 rejected on-surface samples in 1-3 sample runs, punching holes
	// that migrate as the object turns (dashes plus shimmer). Measured
	// over the whole loop at 100x28, dropouts fall 217 -> 17 going 0.18
	// -> 0.40, and flatten after that. The far side cannot leak in at
	// this value: it sits ~2*smallR behind, and no drawn sample anywhere
	// in the loop exceeds a depth delta of 0.5 until bias passes 0.7.

	// The depth window the palette is stretched across, in units of maxR. Because the
	// back-face cull removes everything facing away, the visible surface only spans the
	// near part of the object — mapping the palette across the FULL [-maxR, +maxR] would
	// squeeze every wire into the top half of the ramp and the torus would read as one
	// flat magenta mesh with no cyan in it at all.
	depthNear = -1.00 // rz that gets the hot near colour
	depthFar  = 0.35  // rz that gets the deep far colour

	// The raw shade piles up around 0.6–0.9 (most of a front-facing surface is both
	// near-ish and lit-ish), which would spend the whole palette in its magenta band.
	// >1 pulls the mids down into the indigo/violet so the full ramp gets used and the
	// hot pink stays a rare near-limb highlight rather than the body colour.
	shadeGamma = 2.20

	lightMix = 0.40 // how much of the shade is Lambert vs. raw depth (0 = pure depth)
	lightX   = -0.4 // light direction, pointing from the surface toward the light…
	lightY   = -0.7 // …up and to the left of the eye, so the tumble has a lit side
	lightZ   = -1.0 // …and mostly toward the viewer

	washAmt   = 0.60 // overall backdrop brightness — the layer that stops a black box
	washBase  = 0.25 // floor of the backdrop glow at the corners (before the vignette)
	washFall  = 1.30 // radial falloff of the backdrop glow; larger = tighter halo
	washTint  = 0.20 // where the backdrop samples the palette (the deep cyan-blue end)
	washSwirl = 0.30 // depth of the backdrop's slow rotating asymmetry
	vigPow    = 0.60 // vignette softness (smaller = softer, larger bright core)
)

// brailleBit maps a dot at (row, col) inside a cell to its bit in the U+2800 block.
// The dot numbering is column-major for the historic 6-dot cell (1,2,3 down the left
// column; 4,5,6 down the right) and only then tacks 7/8 on as a bottom row — so the
// bit order is NOT raster order and the obvious 1<<(row*2+col) is wrong on rows 1-3.
//
//	dot1 dot4   0x01 0x08
//	dot2 dot5   0x02 0x10
//	dot3 dot6   0x04 0x20
//	dot7 dot8   0x40 0x80   <- appended to the standard later, hence the irregularity
var brailleBit = [4][2]uint8{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// torStops is the designed palette: shade → colour, far/dark to near/hot. A deep
// blue-black void, deep cyan-blue on the receding side, through indigo and violet to a
// magenta near limb and a pink-white highlight. Hue moves with depth, not just
// luminance, so the tumble reads even where the dot density is flat. Spaced to read
// evenly (OKLab-informed) and interpolated in sRGB.
var torStops = []struct {
	t float64
	c rgb
}{
	{0.00, rgb{0.02, 0.04, 0.10}},
	{0.22, rgb{0.06, 0.16, 0.34}},
	{0.45, rgb{0.22, 0.20, 0.62}},
	{0.66, rgb{0.55, 0.26, 0.75}},
	{0.85, rgb{0.95, 0.31, 0.72}},
	{1.00, rgb{1.00, 0.87, 0.96}},
}

// bayer4 is the ordered-dither threshold matrix (values 0..15). Indexed by screen
// position, a given value always dithers the same way → stable under motion.
var bayer4 = [4][4]float64{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

type rgb struct{ r, g, b float64 }

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// palette maps a 0..1 shade onto torStops by piecewise-linear interpolation.
func palette(t float64) rgb {
	t = clamp01(t)
	for i := 1; i < len(torStops); i++ {
		hi := torStops[i]
		if t <= hi.t {
			lo := torStops[i-1]
			k := (t - lo.t) / (hi.t - lo.t)
			return rgb{lerp(lo.c.r, hi.c.r, k), lerp(lo.c.g, hi.c.g, k), lerp(lo.c.b, hi.c.b, k)}
		}
	}
	return torStops[len(torStops)-1].c
}

// vig is the edge vignette: 1 at the centre, 0 at every border, so the pane never
// meets a hard rectangular edge (references/craft.md). u and yn are 0..1 across the
// full width and the full height — this is the one place y is normalized by height
// rather than width.
func vig(u, yn float64) float64 {
	e := math.Sin(math.Pi*u) * math.Sin(math.Pi*yn)
	if e <= 0 {
		return 0
	}
	return math.Pow(e, vigPow)
}

// chan8 clamps a 0..1 channel to a 0..255 byte.
func chan8(x float64) uint8 {
	v := int(math.Round(clamp01(x) * 255))
	return uint8(v)
}

// Period returns the loop length in ticks for a w×h pane: Frame(w,h,0) and
// Frame(w,h,Period(w,h)) are byte-identical.
//
// It is NOT a constant, and that is the point. The torus turns by a fixed ANGLE per
// tick, but what the eye sees is dots moving across a grid — and a bigger pane scales
// the object up, so the same angle carries every dot proportionally further. Past
// about one dot of travel per frame the whole pattern rewrites itself each frame and
// a braille render visibly flickers; there is no way to soften it, because a braille
// cell is monochrome and an individual dot cannot be dimmed. Measured share of lit
// dots that change between consecutive frames, at the shipped 24 s loop: 49% at
// 100×28 (the tuned reference, reads as motion) but 95% at 210×60 (reads as flicker).
//
// So the loop stretches with the pane to hold that number near the reference. A pane
// at the reference size runs the base 24 s; a pane twice as large takes about twice as
// long, and turns at the same apparent speed. Smaller panes are never sped up — the
// base loop is already unhurried — so this only ever slows things down.
//
// Pure in (w, h), so Frame stays pure. Resizing does move the phase, since tick is
// divided by a different loop length; the animation is free-running and a resize
// already re-lays out the whole pane, so that is not a seam anyone can see.
func Period(w, h int) int {
	if w <= 0 || h <= 0 {
		return basePeriod
	}
	span := 2 * w
	if 4*h < span {
		span = 4 * h
	}
	if span <= refSpan {
		return basePeriod
	}
	return int(math.Round(float64(basePeriod) * float64(span) / refSpan))
}

// Frame renders the torus at absolute frame `tick` into a w×h pane: exactly h lines
// of exactly w cells, or "" for a degenerate pane. Pure — no wall clock, no
// math/rand, no package-level mutable state — so it is snapshot-testable and safe to
// call concurrently.
func Frame(w, h, tick int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	// The braille dot grid: every cell is 2 dots wide and 4 tall. A terminal cell is
	// about 1×2, so a braille dot is very nearly square — which is why both axes below
	// are scaled by the SAME factor and the torus stays circular rather than elliptical.
	pw, ph := 2*w, 4*h

	// One phase for every time-varying term. The double modulo keeps a negative tick
	// safe (TestNoPanic feeds -5), and every use of th below is an integer harmonic,
	// which is what makes tick 0 and tick Period(w,h) byte-identical.
	period := Period(w, h)
	th := 2 * math.Pi * float64(((tick%period)+period)%period) / float64(period)

	// Rotation: Ry(θy) then Rx(θx). tiltY rides inside θy as a constant offset — see
	// the package comment for why it is load-bearing rather than decorative.
	ax := spinU * th
	ay := spinV*th + tiltY
	sa, ca := math.Sin(ax), math.Cos(ax)
	sb, cb := math.Sin(ay), math.Cos(ay)
	rot := func(x, y, z float64) (float64, float64, float64) {
		x1 := x*cb + z*sb
		z1 := z*cb - x*sb
		return x1, y*ca - z1*sa, y*sa + z1*ca
	}

	// Fit the torus to the short axis with a margin. Every point lies within maxR of
	// the origin, and maximizing the projected radius ρ·persp/(persp+z) over
	// ρ²+z² = maxR² gives maxR·persp/√(persp²−maxR²) — the exact worst case over every
	// attitude of the tumble, so the shape can never clip the border (pinned by
	// TestFitsPane) without the slack a looser bound would waste.
	maxR := bigR + smallR
	maxProj := maxR * persp / math.Sqrt(persp*persp-maxR*maxR)
	scale := fitFrac * math.Min(float64(pw), float64(ph)) / (2 * maxProj)
	cx, cy := float64(pw)/2, float64(ph)/2

	// project maps a rotated point to a dot coordinate plus its depth.
	project := func(rx, ry, rz float64) (int, int, bool) {
		m := persp / (persp + rz)
		px := int(math.Floor(cx + scale*rx*m))
		py := int(math.Floor(cy + scale*ry*m))
		return px, py, px >= 0 && px < pw && py >= 0 && py < ph
	}

	// --- pass 1: the occluder ------------------------------------------------
	// Rasterize the opaque surface into a per-dot depth buffer. It is never drawn; it
	// exists only so pass 2 can discard the wires behind it. Sample counts follow the
	// on-screen arc length so the surface has no pinholes at any pane size, and the
	// per-frame cos/sin tables keep the inner loop free of trig entirely.
	zbuf := make([]float64, pw*ph)
	for i := range zbuf {
		zbuf[i] = math.Inf(1)
	}
	nu := clampInt(int(2*math.Pi*maxR*scale*surfDensity), minSteps, maxSteps)
	nv := clampInt(int(2*math.Pi*smallR*scale*surfDensity), minSteps, maxSteps)
	cosU, sinU := ringTable(nu)
	cosV, sinV := ringTable(nv)
	for j := 0; j < nv; j++ {
		rr := bigR + smallR*cosV[j]
		zz := smallR * sinV[j]
		for i := 0; i < nu; i++ {
			rx, ry, rz := rot(rr*cosU[i], rr*sinU[i], zz)
			if px, py, ok := project(rx, ry, rz); ok {
				if idx := py*pw + px; rz < zbuf[idx] {
					zbuf[idx] = rz
				}
			}
		}
	}

	// --- pass 2: the wires ---------------------------------------------------
	// One dot mask and one shade per CELL: the mask is the geometry (up to 8 dots), the
	// shade is the single colour they must share, taken from the NEAREST wire in the
	// cell — the front wire wins, which is the right read for one foreground colour.
	mask := make([]uint8, w*h)
	shade := make([]float64, w*h)
	cdep := make([]float64, w*h)
	for i := range cdep {
		cdep[i] = math.Inf(1)
	}
	ll := math.Sqrt(lightX*lightX + lightY*lightY + lightZ*lightZ)
	lx, ly, lz := lightX/ll, lightY/ll, lightZ/ll

	plot := func(x, y, z, nx, ny, nz float64) {
		rx, ry, rz := rot(x, y, z)
		px, py, ok := project(rx, ry, rz)
		if !ok {
			return
		}
		rnx, rny, rnz := rot(nx, ny, nz)
		// Hidden-line removal, in two overlapping stages. A back-face cull against the
		// eye: the tube occludes itself *analytically*, with no z-fighting, because the
		// normal is exact. Then the depth buffer, which is the only one of the two that
		// can catch one part of the ring passing in front of another.
		//
		// They overlap heavily — summed over the whole 720-tick loop at 100×28,
		// disabling either one alone raises the lit-cell count only ~4% (the survivor
		// picks up the slack), while disabling both raises it ~19%. Per frame the
		// spread is far wider (dropping the depth test alone ranges 0% to +32%), which
		// is why the aggregate is quoted. Both are kept because each is exact on a case
		// the other only approximates, and together they let depthBias stay small enough
		// that the far side does not fringe through at the silhouette. (What actually
		// cured the early z-fighting dashes was raising depthBias, not this cull.)
		if rnx*(-rx)+rny*(-ry)+rnz*(-persp-rz) <= 0 {
			return
		}
		if rz > zbuf[py*pw+px]+depthBias {
			return
		}
		ci := (py/4)*w + (px / 2)
		mask[ci] |= brailleBit[py&3][px&1]
		if rz >= cdep[ci] {
			return
		}
		cdep[ci] = rz
		diff := rnx*lx + rny*ly + rnz*lz
		if diff < 0 {
			diff = 0
		}
		depth := clamp01((depthFar*maxR - rz) / ((depthFar - depthNear) * maxR))
		shade[ci] = math.Pow(clamp01(lightMix*diff+(1-lightMix)*depth), shadeGamma)
	}

	// Meridian rings (constant u, sweeping v — around the tube).
	nvw := clampInt(int(2*math.Pi*smallR*scale*wireDensity), minSteps, maxSteps)
	wCosV, wSinV := ringTable(nvw)
	for k := 0; k < ringsU; k++ {
		u := 2 * math.Pi * float64(k) / ringsU
		cu, su := math.Cos(u), math.Sin(u)
		for j := 0; j < nvw; j++ {
			rr := bigR + smallR*wCosV[j]
			plot(rr*cu, rr*su, smallR*wSinV[j], wCosV[j]*cu, wCosV[j]*su, wSinV[j])
		}
	}
	// Longitude rings (constant v, sweeping u — around the ring).
	nuw := clampInt(int(2*math.Pi*maxR*scale*wireDensity), minSteps, maxSteps)
	wCosU, wSinU := ringTable(nuw)
	for k := 0; k < ringsV; k++ {
		v := 2 * math.Pi * float64(k) / ringsV
		cv, sv := math.Cos(v), math.Sin(v)
		rr := bigR + smallR*cv
		zz := smallR * sv
		for j := 0; j < nuw; j++ {
			plot(rr*wCosU[j], rr*wSinU[j], zz, cv*wCosU[j], cv*wSinU[j], sv)
		}
	}

	// --- pass 3: compose -----------------------------------------------------
	var b strings.Builder
	// A lit cell is at most 39 bytes (\x1b[38;2;255;255;255;48;2;255;255;255m + a
	// 3-byte braille rune); an empty one is a bg-only 20. Each row adds a 4-byte reset
	// and a newline.
	b.Grow(w*h*40 + h*5)
	ref := math.Min(float64(pw), float64(ph)) / 2
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			// The backdrop wash, evaluated once per cell (it is the cell's background,
			// so it needs no sub-cell resolution). Dot-space coordinates keep the glow
			// radially symmetric rather than stretched.
			nx := ((float64(c)+0.5)*2 - cx) / ref
			ny := ((float64(r)+0.5)*4 - cy) / ref
			glow := 1 / (1 + washFall*(nx*nx+ny*ny))
			swirl := 0.5 + 0.5*math.Cos(2*math.Atan2(ny, nx)-th)
			lum := washAmt *
				(washBase + (1-washBase)*glow) *
				(1 - washSwirl + washSwirl*swirl) *
				vig((float64(c)+0.5)/float64(w), (float64(r)+0.5)/float64(h))
			wc := palette(washTint)
			// Screen-locked Bayer: a given value always dithers the same way, so the
			// dim wash never shimmers or crawls as the torus turns.
			d := ((bayer4[r&3][c&3]+0.5)/16 - 0.5) / 255.0
			br, bg, bb := chan8(wc.r*lum+d), chan8(wc.g*lum+d), chan8(wc.b*lum+d)

			ci := r*w + c
			if mask[ci] == 0 {
				appendEmpty(&b, br, bg, bb)
				continue
			}
			fc := palette(shade[ci])
			appendDots(&b, chan8(fc.r), chan8(fc.g), chan8(fc.b), br, bg, bb, mask[ci])
		}
		b.WriteString("\x1b[0m")
		if r < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// ringTable returns cos/sin of n angles evenly spaced around a circle. Hoisting these
// out of the rasterizer's inner loop is what keeps it trig-free: the occluder pass
// costs n+m transcendentals per frame instead of n×m.
func ringTable(n int) ([]float64, []float64) {
	cs, sn := make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		cs[i], sn[i] = math.Cos(a), math.Sin(a)
	}
	return cs, sn
}

// appendDots writes one braille cell — foreground = the lit dots, background = the
// backdrop wash — as an SGR truecolor sequence. Hand-rolled with strconv so the
// per-cell hot path carries no fmt reflection or allocation (Frame calls this once per
// lit cell, and appendEmpty for the rest — together, w×h calls a frame).
func appendDots(b *strings.Builder, fr, fg, fb, br, bg, bb uint8, dots uint8) {
	b.WriteString("\x1b[38;2;")
	writeChan(b, fr)
	b.WriteByte(';')
	writeChan(b, fg)
	b.WriteByte(';')
	writeChan(b, fb)
	b.WriteString(";48;2;")
	writeChan(b, br)
	b.WriteByte(';')
	writeChan(b, bg)
	b.WriteByte(';')
	writeChan(b, bb)
	b.WriteByte('m')
	b.WriteRune(rune(0x2800 + int(dots)))
}

// appendEmpty writes a cell with no lit dots: a plain space over the wash. It sets no
// foreground — nothing is drawn — which keeps a sparse wireframe's frames roughly half
// the size they would otherwise be. A space is also preferred over U+2800 (blank
// braille) on size alone: one byte instead of three, on the majority of cells.
// (ansi2png.py resolves U+2800 to solid background as of this change, so either glyph
// now rasterizes correctly — the choice here is bytes, not correctness.)
func appendEmpty(b *strings.Builder, br, bg, bb uint8) {
	b.WriteString("\x1b[48;2;")
	writeChan(b, br)
	b.WriteByte(';')
	writeChan(b, bg)
	b.WriteByte(';')
	writeChan(b, bb)
	b.WriteString("m ")
}

func writeChan(b *strings.Builder, v uint8) {
	var s [3]byte
	b.Write(strconv.AppendUint(s[:0], uint64(v), 10))
}
