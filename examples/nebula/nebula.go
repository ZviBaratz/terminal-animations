// Package nebula is a deep-space splash-screen animation for the terminal:
// a pure, deterministic, half-block truecolor field that evokes drifting slowly
// through a nebula, and loops forever with no seam.
//
// It is the field-shaped standalone from references/… (skill §B) — a pure
// function of (w, h, tick) — composed well past the conventional "one effect in
// flat ASCII" default:
//
//   - Fidelity tier: half blocks (▀). Every character cell carries TWO independent
//     24-bit pixels (fg = top, bg = bottom), doubling vertical resolution — the
//     "portable workhorse" rung from references/techniques.md. Brightness rides
//     colour luminance (a uniform glyph), so a dim region stays a smooth dim wash
//     rather than a scatter of confetti dots.
//   - The cloud: fractal value noise (fbm) put through an Iñigo-Quílez domain warp,
//     which turns flat plasma banding into swirled, billowing cloud structure. All
//     randomness enters only through an integer coordinate hash (hash2) — never
//     math/rand — so the field is snapshot-testable.
//   - Designed palette: a cool indigo → violet → magenta ramp (deep blue-black voids,
//     indigo dust, a soft magenta glow in the dense cores), not a functional
//     grey→white gradient.
//   - Depth: a sparse layer of drifting parallax stars behind the cloud. They slide
//     WITH the drift (slower, being far away), so they read as travel through space
//     and never as stuck pixels over a moving field (references/craft.md).
//   - Motion-stable Bayer (ordered) dithering, screen-locked, so quantization never
//     shimmers or crawls as the field drifts — the right choice for animation.
//   - Composition: an edge vignette fades every border to black, so the splash reads
//     as a window onto something larger, not a lit rectangle.
//   - A truly seamless forever-loop: every time-varying term flows through a single
//     phase θ = 2π·(tick mod period)/period, so Frame(w,h,0) and Frame(w,h,period)
//     receive identical inputs and are byte-identical (see nebula_test.go's TestLoopSeam).
//
// The taste constants below were chosen against the beauty gate (the ansi2png.py
// filmstrip), by eye — not computed. See README.md.
package nebula

import (
	"fmt"
	"math"
	"strings"
)

// Tunable taste constants — decided by looking, not arithmetic.
const (
	period = 1080 // loop length in ticks; θ wraps here (≈36 s at 30 fps — slow drift)

	noiseScale = 3.2 // spatial frequency of the cloud across the pane
	octaves    = 5   // fbm octaves — more = finer wisps
	lacunarity = 2.0 // per-octave frequency multiplier
	gain       = 0.5 // per-octave amplitude multiplier
	warpAmt    = 2.6 // domain-warp strength (the swirl)

	driftR = 0.55 // radius of the camera's drift circle (the "moving through")
	churnR = 0.45 // radius of the warp's churn circle (the billow)
	churnK = 1    // churn harmonic — 1 = the cloud evolves once per loop (calmest)

	voidCut      = 0.42 // density below this is empty space
	densityGain  = 2.2  // contrast lift after the cut
	densityGamma = 1.5  // >1 keeps mids dim and cores bright (carves voids)
	vigPow       = 0.6  // vignette softness (smaller = softer, larger bright core)

	starScale    = 0.5   // star lattice frequency vs. pixels
	starAmp      = 5.0   // star drift amplitude in pixels (slow parallax creep)
	starRadius   = 1.2   // star point radius in pixels
	starDensity  = 0.013 // fraction of lattice cells that hold a star
	starTwinkleK = 2     // twinkle harmonic — integer keeps the loop seamless

	noiseSeed = 1337 // decorrelates the cloud hash
	starSeed  = 9973 // decorrelates the star hash
)

// starCol is the (cool white) colour a star contributes before occlusion/vignette.
var starCol = rgb{0.74, 0.80, 0.96}

// nebStops is the designed palette: density → colour, low to high. Deep blue-black
// voids, indigo dust, violet, a magenta glow, and a pale magenta-white hot core.
// Spaced to read evenly (OKLab-informed) and interpolated in sRGB.
var nebStops = []struct {
	t float64
	c rgb
}{
	{0.00, rgb{0.02, 0.01, 0.06}},
	{0.30, rgb{0.14, 0.09, 0.34}},
	{0.55, rgb{0.34, 0.16, 0.60}},
	{0.78, rgb{0.66, 0.25, 0.72}},
	{0.92, rgb{0.90, 0.45, 0.85}},
	{1.00, rgb{0.99, 0.80, 0.96}},
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

// hash2 is the deterministic randomness source: a uint32 bit-mix of an integer
// lattice coordinate + seed, returned in [0,1). Large odd primes decorrelate the
// axes; unsigned multiplication wraps, which is what makes it a hash.
func hash2(ix, iy, seed uint32) float64 {
	n := ix*374761393 + iy*668265263 + seed*2246822519
	n = (n ^ (n >> 13)) * 1274126177
	n ^= n >> 16
	return float64(n) / 4294967296.0
}

// valueNoise is smooth value noise at (x, y): bilinear interpolation of the four
// surrounding lattice hashes with a quintic fade (6t⁵−15t⁴+10t³), whose C² continuity
// removes the grid creases the eye would otherwise catch in slow motion.
func valueNoise(x, y float64, seed uint32) float64 {
	fx, fy := math.Floor(x), math.Floor(y)
	tx, ty := x-fx, y-fy
	ux := tx * tx * tx * (tx*(tx*6-15) + 10)
	uy := ty * ty * ty * (ty*(ty*6-15) + 10)
	ix, iy := uint32(int32(fx)), uint32(int32(fy))
	c00 := hash2(ix, iy, seed)
	c10 := hash2(ix+1, iy, seed)
	c01 := hash2(ix, iy+1, seed)
	c11 := hash2(ix+1, iy+1, seed)
	return lerp(lerp(c00, c10, ux), lerp(c01, c11, ux), uy)
}

// fbm sums octaves of value noise into fractal detail, normalised to [0,1].
func fbm(x, y float64) float64 {
	sum, amp, freq, norm := 0.0, 1.0, 1.0, 0.0
	for i := 0; i < octaves; i++ {
		sum += amp * valueNoise(x*freq, y*freq, noiseSeed+uint32(i))
		norm += amp
		freq *= lacunarity
		amp *= gain
	}
	return sum / norm
}

// warpedDensity is the cloud field at (x, y): fbm domain-warped by fbm (Iñigo Quílez).
// (cx, cy) is the slowly circulating churn offset injected into the warp, which makes
// the cloud billow and evolve in place.
func warpedDensity(x, y, cx, cy float64) float64 {
	qx := fbm(x, y)
	qy := fbm(x+5.2, y+1.3)
	rx := fbm(x+warpAmt*qx+1.7+cx, y+warpAmt*qy+9.2+cy)
	ry := fbm(x+warpAmt*qx+8.3+cx, y+warpAmt*qy+2.8+cy)
	return fbm(x+warpAmt*rx, y+warpAmt*ry)
}

// shape carves voids and cores out of the raw density: cut the empty floor, lift
// contrast, then gamma the result so mids stay dim and only the cores go bright.
func shape(d float64) float64 {
	d = clamp01((d - voidCut) * densityGain)
	return math.Pow(d, densityGamma)
}

// palette maps a shaped density in [0,1] to a colour along the designed ramp.
func palette(t float64) rgb {
	if t <= 0 {
		return nebStops[0].c
	}
	last := len(nebStops) - 1
	if t >= 1 {
		return nebStops[last].c
	}
	for i := 1; i <= last; i++ {
		if t <= nebStops[i].t {
			a, b := nebStops[i-1], nebStops[i]
			f := (t - a.t) / (b.t - a.t)
			return rgb{lerp(a.c.r, b.c.r, f), lerp(a.c.g, b.c.g, f), lerp(a.c.b, b.c.b, f)}
		}
	}
	return nebStops[last].c
}

// starLum is the star layer's brightness at pixel-space (wx, wy): the soft, twinkling
// contribution of the nearest seeded star in the surrounding 3×3 lattice cells (the
// neighbourhood so a star straddling a cell edge still renders whole). Pure in θ.
func starLum(wx, wy, theta float64) float64 {
	bx, by := math.Floor(wx), math.Floor(wy)
	best := 0.0
	for dj := -1; dj <= 1; dj++ {
		for di := -1; di <= 1; di++ {
			cellX, cellY := bx+float64(di), by+float64(dj)
			ix, iy := uint32(int32(cellX)), uint32(int32(cellY))
			if hash2(ix, iy, starSeed) >= starDensity {
				continue
			}
			sx := cellX + hash2(ix, iy, starSeed+1)
			sy := cellY + hash2(ix, iy, starSeed+2)
			d := math.Hypot(wx-sx, wy-sy)
			if d >= starRadius {
				continue
			}
			base := 0.5 + 0.5*hash2(ix, iy, starSeed+3)
			phase := hash2(ix, iy, starSeed+4) * 2 * math.Pi
			twinkle := 0.65 + 0.35*math.Sin(starTwinkleK*theta+phase)
			fall := 1 - d/starRadius
			if l := base * twinkle * fall * fall; l > best {
				best = l
			}
		}
	}
	return best
}

// vig is the edge vignette: 1 at the centre, fading to 0 at every border, so the
// field reads as a window onto something larger rather than a lit box.
func vig(u, yn float64) float64 {
	e := math.Sin(math.Pi*u) * math.Sin(math.Pi*yn)
	if e <= 0 {
		return 0
	}
	return math.Pow(e, vigPow)
}

// view carries the per-frame state (derived from θ) shared by every sub-pixel.
type view struct {
	W, H2      float64 // pane width, and pixel-row count (2·h)
	camx, camy float64 // cloud drift offset (noise space)
	cx, cy     float64 // warp churn offset (noise space)
	sdx, sdy   float64 // star drift offset (pixel space)
	th         float64 // loop phase θ
}

// at renders one sub-pixel: column `col`, pixel row `py` (0-based, top of pane = 0).
func (v view) at(col, py int) (uint8, uint8, uint8) {
	u := (float64(col) + 0.5) / v.W
	ysq := (float64(py) + 0.5) / v.W // square-aspect vertical, so the cloud isn't stretched
	yn := (float64(py) + 0.5) / v.H2 // 0..1 over full height, for the vignette
	nx := u*noiseScale + v.camx
	ny := ysq*noiseScale + v.camy

	dens := shape(warpedDensity(nx, ny, v.cx, v.cy))
	col3 := palette(dens)
	shade := vig(u, yn)

	// Sparse parallax stars, composited behind the cloud (occluded by bright cores).
	wx := float64(col)*starScale + v.sdx
	wy := float64(py)*starScale + v.sdy
	star := starLum(wx, wy, v.th) * (1 - dens)
	r := (col3.r + starCol.r*star) * shade
	g := (col3.g + starCol.g*star) * shade
	b := (col3.b + starCol.b*star) * shade

	// Screen-locked Bayer dither of the low bit, applied equally to each channel.
	dd := ((bayer4[py&3][col&3]+0.5)/16 - 0.5) / 255.0
	return chan8(r + dd), chan8(g + dd), chan8(b + dd)
}

// Frame renders frame `tick` into exactly h lines of exactly w visible cells (or ""
// for a degenerate pane). Each cell is a half block ▀: its foreground colour paints
// the upper sub-pixel, its background the lower — so the visible grid is w × 2h
// truecolor pixels. Pure in (w, h, tick), and byte-identical every `period` ticks.
func Frame(w, h, tick int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	// Every time term derives from θ = 2π·(tick mod period)/period, so tick=0 and
	// tick=period produce identical inputs → a provably seamless loop.
	th := 2 * math.Pi * float64(((tick%period)+period)%period) / float64(period)
	v := view{
		W:    float64(w),
		H2:   float64(2 * h),
		camx: driftR * math.Cos(th),
		camy: driftR * math.Sin(th),
		cx:   churnR * math.Cos(churnK*th),
		cy:   churnR * math.Sin(churnK*th),
		sdx:  starAmp * math.Cos(th),
		sdy:  starAmp * math.Sin(th),
		th:   th,
	}

	var b strings.Builder
	b.Grow(w*h*22 + h*4)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			tr, tg, tb := v.at(c, 2*r)
			br, bg, bb := v.at(c, 2*r+1)
			fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", tr, tg, tb, br, bg, bb)
		}
		b.WriteString("\x1b[0m")
		if r < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func clamp01(x float64) float64 {
	switch {
	case x < 0:
		return 0
	case x > 1:
		return 1
	default:
		return x
	}
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
