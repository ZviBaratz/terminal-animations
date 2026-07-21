// Package life is Conway's Game of Life rendered as a glowing rose window: a stateful,
// deterministic animation where a live cell ignites white-hot, settles to a steady glow,
// and — when it dies — leaves a halo that cools through gold, ruby and magenta to a cold
// cobalt before going out.
//
// It is the standalone convention's Animation shape (skill §B) — Update / View / Done
// carrying a Life grid as state — and this repo's first worked example of that
// interface; every other example (plasma, nebula, torus, saucer, bust) is a pure
// Frame(w, h, tick). Life earns the stateful shape honestly: a cell's next value is a
// function of its neighbours, not of the tick, so there is no closed form to fold into
// a Frame.
//
//   - The idea: a symmetric kaleidoscope. Conway's rules are isotropic, so a seed with
//     mirror/rotational symmetry stays symmetric for every later generation. The identical
//     churn that reads as busy noise in a random soup reads as a breathing mandala once it
//     is symmetric — anchored to a focal centre (references/craft.md: a glyph in every cell is
//     texture, not weather; give it a focal point and it becomes ornament). The seed is built
//     with the full 8-fold symmetry of the square (D4) and confined to a centred disc.
//   - It fills the frame. The disc is grown until it covers the pane — the sim runs on a
//     SQUARE board larger than the pane and View shows a centred window into it (a wide pane
//     sees a full-width band through the middle of the mandala) — and a radial vignette fades
//     the corners, so the circle is sculpted by the fade rather than left as a small medallion
//     in a void. The square board is also what keeps the diagonal symmetry exact (see reset).
//   - Fidelity tier: half-block (U+2580 ▀), the portable workhorse of
//     references/techniques.md. Each terminal cell is two independent truecolor pixels
//     (fg = top, bg = bottom), so the board is w × 2h. Bold cells — not the fine braille
//     confetti a first cut tried — are what let the eye read structure rather than specks.
//   - Motion read as motion (references/craft.md): this does not render alive/dead directly.
//     It carries a HEAT field that a birth ignites toward white-hot over a few frames and a
//     death leaves to decay, so every change drags a cooling trail — the canonical "this is
//     moving" signal — and the symmetric field pulses rather than flickers. Igniting a birth
//     (rather than flashing it on in one frame) is what makes the step-every-few-frames motion
//     read as a smooth breath instead of a jump.
//   - It glows rather than tiles. A Life cell is one pixel, so painting the heat field
//     directly gives hard-edged blocks whose colour jumps from neighbour to neighbour — a
//     mosaic. The heat is therefore also blurred into a HALO and composited back over
//     itself (bloom/at), so light bleeds between cells, the gaps light up and every hot
//     cell's falloff is a continuous gradient. That is what carries the smooth colour; the
//     ramp below only decides which hues it passes through.
//   - Colour carries everything, on a designed ramp of cathedral glass: a cobalt ground,
//     violet and magenta through the mid-field, ruby and gold along the arms, a white core.
//     Blended in OKLab so the gradients stay even instead of dipping through muddy
//     midpoints, baked to a lookup table, and dithered at the 8-bit output so the wide
//     smooth washes do not band. Half-block gives the smooth per-pixel gradient a glow
//     wants (a fading region on the glyph-density channel would just break into dots —
//     references/craft.md).
//   - A radial vignette fades the disc toward black at its rim — but it fades the RAMP
//     COORDINATE, not the finished colour, so the falloff walks down the palette (gold →
//     ruby → magenta → violet → cobalt) instead of dimming toward grey. The mandala reads
//     as concentric bands of glass with black corners rather than as a rectangle.
//   - Determinism: the symmetric soup is a fixed integer-coordinate hash (the hash2 pattern
//     the other examples use), never math/rand or a wall clock, and the stagnation rebloom
//     is keyed on the generation count — so a given (size, tick) always replays to the same
//     field.
//
// The taste constants below were chosen against the beauty gate by eye, not computed.
// See README.md.
package life

import (
	"math"
	"strconv"
	"strings"
)

// Tunable taste constants — decided by looking at the beauty gate, not arithmetic.
const (
	// ticksPerGen is how many render frames pass between Life generations. Slower than a
	// bare sim wants, on purpose: the heat trails need frames to ignite and cool between
	// steps, and a slower step lets the eye follow a structure instead of watching it
	// strobe. At the 30fps the recordings run, 7 frames/gen is ~4 generations a second —
	// calm — while giving each birth several frames to bloom, so the motion reads as a
	// smooth pulse rather than a jump every step.
	ticksPerGen = 7

	// seedDensity is the fraction of cells alive in the initial (and reseeded) soup.
	// Below Life's ~0.3 chaos peak, deliberately — a sparser field leaves the negative
	// space the glow needs to read against.
	seedDensity = 0.22

	// symDiagonal picks the symmetry group the seed is built with — which Life then keeps
	// for ever, since its rules are isotropic. true → D4, the full 8-fold symmetry of the
	// square (both axis mirrors and both diagonals): a kaleidoscopic mandala. false → D2,
	// the two axis mirrors only (4-fold). The identical churn that reads as noise in a
	// random soup reads as ornament once it is symmetric. Chosen by eye at the gate.
	symDiagonal = true

	// domFrac sets the live disc's radius as a fraction of the (square) board's half-extent.
	// The board is a square larger than the pane and the pane views a centred window into it
	// (see reset/View), so pushing domFrac close to 1 grows the disc until it fills the frame:
	// the pane shows a band through a frame-filling mandala, and only the corners — outside the
	// disc — fade to black under the radial vignette, so the circle is sculpted by the fade
	// rather than left as a small medallion in a void. It stays strictly interior (rad ≤ half−1)
	// so the diagonal reflection — a lattice symmetry only about the integer centre — stays exact.
	domFrac = 0.98

	// The heat model, per pixel, updated every frame. Each pixel eases toward a target: a
	// just-born cell (age 0) targets a white-hot 1 and RISES to it at riseEase, so a birth
	// ignites over a couple of frames — a bloom, not an instant flash — which is what turns
	// the step-every-few-frames motion from a jump into a smooth pulse (an instant spike on
	// birth while deaths only decay is the asymmetry that reads as "jumpy"). An older living
	// cell targets its age-cooled sustain and eases DOWN off the birth peak at sustainEase; a
	// dead cell's heat decays by deathDecay each frame, leaving the cooling trail. The fast
	// rise keeps the bright leading edge; the slow fall keeps the decaying trail.
	//
	// The sustain itself cools with the cell's AGE: a young/active cell glows at
	// sustainHot, but a cell that has survived unchanged for ageFull generations relaxes
	// to the much dimmer sustainCold, so a settled still-life recedes into a low
	// background ember instead of sitting as a fixed bright dot (references/craft.md:
	// fixed bright points read as stuck pixels). Recency of activity therefore reads as
	// brightness — churning regions stay hot, dead structure goes dim.
	sustainHot  = 0.66 // glow of a young/active living cell
	sustainCold = 0.24 // glow a long-settled still-life cools to
	ageFull     = 14.0 // generations of survival to cool from hot to cold
	ageCap      = 255  // survivor age saturates here
	riseEase    = 0.55 // how fast a fresh birth blooms UP toward the white-hot spark
	sustainEase = 0.20 // how fast heat eases DOWN toward the (age-dependent) sustain
	// deathDecay is the per-frame cooling of a dead cell's trail. Slow on purpose: heat is
	// the ramp coordinate, so a fast decay skips the palette's whole middle — at 0.8 a dying
	// cell fell past ruby and magenta within about two frames and the field read as two
	// colours, hot and black. At 0.9 the trail spends visible frames in each band, which is
	// what makes the cooling arc a colour arc.
	deathDecay = 0.90

	// The halo. A Life cell is one pixel, so rendering heat alone paints hard-edged blocks
	// whose colour jumps discontinuously from neighbour to neighbour — a mosaic, not a glow.
	// bloom blurs the heat field and composites the blur back over it (see bloom/at), so
	// light bleeds between cells: the gaps light up, the edges soften, and a hot cell's
	// falloff travels DOWN the ramp through gold, ruby and violet instead of stopping at the
	// pixel border. This is what carries the smooth colour; the ramp only decides its hues.
	// Radius and passes set the halo's reach (two passes of a box blur ≈ Gaussian);
	// bloomGain how far it lifts the field before structure starts dissolving into haze.
	bloomGain   = 1.0
	bloomRadius = 2
	bloomPasses = 2

	// The vignette acts on the ramp COORDINATE, not on the finished colour: fading toward
	// the rim slides a pixel down the palette (gold → ruby → magenta → violet → cobalt)
	// rather than dimming it toward grey, which is what gives the mandala its concentric
	// stained-glass zoning for free. vigTone is how much of the fade rides the ramp (1 =
	// all of it); lumPow is the residual luminance falloff on top, kept gentle so the
	// colour arc reads across most of the disc yet the corners still reach true black.
	vigTone = 1.0
	lumPow  = 0.35

	// Stagnation reseed. Pure soup settles to still-lifes and blinkers within dozens of
	// generations — a dead gallery piece. When a generation changes too few cells for
	// stagnStreak generations running, inject a fresh soup patch (plus a glider) so the
	// field never goes static. Keyed on the generation count, not wall-clock or rand, so
	// replay stays reproducible.
	stagnStreak = 8
	stagnFrac   = 0.008

	// popFloorFrac reblooms the disc once its live fill drops below this fraction of the disc
	// area. Population is the honest fill signal: cells culled at the disc rim keep the raw
	// change count high, so change-based stagnation alone lets the mandala thin almost to
	// black before it fires. The floor keeps it visibly full.
	popFloorFrac = 0.10

	vigPow = 1.0 // radial-vignette softness (smaller = broader core; larger = tighter circle)

	// Fixed hashes so the soup, the planted gliders and each reseed draw from
	// independent streams rather than aliasing one another.
	seedSalt   = 0x9e3779b1
	reseedSalt = 0x85ebca77
)

// glassStops is the designed ramp: heat → colour, cold to hot. The physics underneath is
// still embers — a cell ignites, sustains, cools — but the palette is cathedral glass
// rather than firelight, which is what the frame-filling rose window wanted: a cobalt
// ground, violet and magenta through the mid-field, ruby and gold along the arms, and a
// white core. Hue moves with heat, not just luminance, so a cooling trail reads as a
// colour arc rather than a fade to grey, and the radial vignette (which rides this same
// coordinate) turns the disc into concentric bands of glass.
//
// Spaced to read evenly by eye and interpolated in OKLab — see buildRamp: a straight sRGB
// lerp between two saturated stops dips through a muddy, darker midpoint, which the wide
// smooth gradients the halo now paints would show plainly.
var glassStops = []struct {
	t float64
	c rgb
}{
	{0.00, rgb{0.01, 0.01, 0.04}}, // cold — near-black with a blue cast
	{0.14, rgb{0.05, 0.10, 0.36}}, // cobalt — the coldest visible glass
	{0.30, rgb{0.28, 0.10, 0.55}}, // violet
	{0.48, rgb{0.72, 0.12, 0.48}}, // magenta
	{0.64, rgb{0.90, 0.18, 0.22}}, // ruby
	{0.82, rgb{1.00, 0.62, 0.18}}, // gold — the living glow
	{1.00, rgb{1.00, 0.97, 0.86}}, // white — a fresh spark
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

// Life is a half-block Conway's Game of Life rendered as cooling embers. It carries the
// board and a per-pixel heat field as state, stepping a generation every ticksPerGen
// frames while the heat decays every frame. Construct with New; it owns its dimensions
// and re-seeds if View is later asked for a different pane.
type Life struct {
	w, h    int       // terminal cell dimensions (the viewed pane)
	bw, bh  int       // square sim board, odd side ≥ max(w, 2h); View shows a centred w × 2h window
	alive   []bool    // current life state per pixel
	next    []bool    // scratch buffer for one generation, reused across steps
	age     []uint16  // generations a cell has survived unchanged (cools its glow)
	ageNext []uint16  // scratch buffer for the age, reused across steps
	heat    []float64 // glow per pixel, 0..1 — a birth ignites it, death lets it decay
	glow    []float64 // heat blurred into a halo; rebuilt every frame (see bloom)
	blurTmp []float64 // scratch for the separable blur, reused across frames
	gen     int       // generations stepped so far
	frame   int       // frames advanced to (absolute tick)
	stagn   int       // consecutive low-change generations (stagnation counter)
	cx, cy  int       // integer centre of the symmetric disc (bw/2, bh/2)
	rad     int       // radius of the live disc; growth is confined here to keep symmetry exact
}

// New builds a Life for a w×h terminal pane (internal pixel grid w × 2h) and seeds it
// deterministically with a symmetric soup confined to a centred disc, so it opens as a
// symmetric field that Life keeps symmetric for ever. A degenerate pane yields a Life
// whose View is "".
func New(w, h int) *Life {
	l := &Life{}
	l.reset(w, h)
	return l
}

// reset re-seeds the whole board at a new size. Called by New and by View when the
// requested pane differs from the constructed one (a resize re-lays out everything
// anyway, so restarting the sim there is invisible).
func (l *Life) reset(w, h int) {
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	l.w, l.h = w, h
	// The pane's pixel viewport is w × 2h, but the sim runs on a SQUARE board large enough to
	// hold the full circular mandala — an odd side S ≥ max(w, 2h) — and View shows a centred window
	// into it. A square domain is what lets the disc grow to fill the frame while keeping the
	// diagonal (D4) reflection an exact lattice symmetry: it needs an integer centre, which a
	// square gives for any pane, where a non-square board's diagonal would map cells off-grid.
	// The side is rounded up to ODD so the board is exactly symmetric about its own centre
	// pixel: on an even side the index range [0, S-1] straddles bw/2 lopsidedly and the
	// mirror of column 0 falls off the far edge. The cells never notice — they are confined
	// to a strictly interior disc — but the halo's blur reads the whole board, and a tap
	// present on one side and missing on the other tilts the glow at the rim. Rounding up
	// costs one row and column and leaves the disc radius unchanged (min/2 truncates alike
	// for 2k and 2k+1).
	s := w
	if 2*h > s {
		s = 2 * h
	}
	if s%2 == 0 {
		s++
	}
	l.bw, l.bh = s, s
	n := l.bw * l.bh
	l.alive = make([]bool, n)
	l.next = make([]bool, n)
	l.age = make([]uint16, n)
	l.ageNext = make([]uint16, n)
	l.heat = make([]float64, n)
	l.glow = make([]float64, n)
	l.blurTmp = make([]float64, n)
	l.gen = 0
	l.frame = 0
	l.stagn = 0
	l.cx, l.cy = l.bw/2, l.bh/2
	l.rad = discRadius(l.bw, l.bh)
	l.seedSymmetric(seedSalt)
	// Seed the heat from the opening soup so frame 0 glows rather than starting black, and
	// build its halo too — otherwise the very first frame renders as bare blocks and the
	// glow only appears one frame later.
	for i := range l.alive {
		if l.alive[i] {
			l.heat[i] = sustainHot
		}
	}
	l.bloom()
}

// hash2 is the deterministic randomness source: a uint32 bit-mix of an integer
// lattice coordinate plus a salt, returned in [0,1). Same shape as examples/nebula.
func hash2(ix, iy, salt uint32) float64 {
	n := ix*374761393 + iy*668265263 + salt*2246822519
	n = (n ^ (n >> 13)) * 1274126177
	n ^= n >> 16
	return float64(n) / 4294967296.0
}

// discRadius is the live disc's radius for a bw×bh pixel grid: domFrac of the smaller
// half-extent, but always at least one pixel short of it, so the disc stays strictly inside
// the board. That interiority is what keeps the diagonal reflection exact: it is a lattice
// symmetry only about the integer centre, and a disc touching a non-square edge would break it.
func discRadius(bw, bh int) int {
	half := bw / 2
	if bh/2 < half {
		half = bh / 2
	}
	r := int(domFrac * float64(half))
	if r > half-1 {
		r = half - 1
	}
	if r < 0 {
		r = 0
	}
	return r
}

// inDisc reports whether pixel (x, y) lies within the live disc centred at (cx, cy).
func (l *Life) inDisc(x, y int) bool {
	dx, dy := x-l.cx, y-l.cy
	return dx*dx+dy*dy <= l.rad*l.rad
}

// seedSymmetric lays down a soup that is exactly symmetric under the active dihedral group,
// which Life — being isotropic — then preserves for every later generation. It samples only
// the fundamental wedge (an eighth of the disc for D4, a quarter for D2) and mirrors each hit
// into all its images; that is what makes the seed exactly symmetric, where mirroring a whole
// random field would double-sample the axes and read lopsided. Keyed on the coordinate hash,
// so it replays identically.
func (l *Life) seedSymmetric(salt uint32) {
	r2 := l.rad * l.rad
	for dy := 0; dy <= l.rad; dy++ {
		dx0 := 0
		if symDiagonal {
			dx0 = dy // D4 wedge is dy ≤ dx; D2 takes the whole quadrant dx ≥ 0
		}
		for dx := dx0; dx <= l.rad; dx++ {
			if dx*dx+dy*dy > r2 {
				break // past the rim on this row; larger dx is only farther out
			}
			if hash2(uint32(dx), uint32(dy), salt) < seedDensity {
				l.plotSym(dx, dy)
			}
		}
	}
}

// plotSym marks the cell at offset (dx, dy) from the disc centre alive, together with all its
// images under the active symmetry group: the four axis mirrors always, plus the four diagonal
// images under D4. Because every image shares dx²+dy², all land inside the disc.
func (l *Life) plotSym(dx, dy int) {
	l.plot(l.cx+dx, l.cy+dy)
	l.plot(l.cx-dx, l.cy+dy)
	l.plot(l.cx+dx, l.cy-dy)
	l.plot(l.cx-dx, l.cy-dy)
	if symDiagonal {
		l.plot(l.cx+dy, l.cy+dx)
		l.plot(l.cx-dy, l.cy+dx)
		l.plot(l.cx+dy, l.cy-dx)
		l.plot(l.cx-dy, l.cy-dx)
	}
}

// plot marks pixel (x, y) alive when it is on the grid.
func (l *Life) plot(x, y int) {
	if x >= 0 && x < l.bw && y >= 0 && y < l.bh {
		l.alive[y*l.bw+x] = true
	}
}

// glider is the classic 5-cell spaceship, offsets from a corner; four rotations give the four
// diagonal headings. The symmetric soup no longer plants gliders (a lone glider would break the
// field's symmetry), but glider and stamp remain for the canonical spaceship test.
var glider = [5][2]int{{1, 0}, {2, 1}, {0, 2}, {1, 2}, {2, 2}}

// stamp writes a pattern (dot offsets) into the board at (ox, oy), rotated by rot×90°
// about the pattern's 3×3 bounding box, as freshly-alive cells. Out-of-bounds dots are
// dropped (no wraparound).
func (l *Life) stamp(pattern [][2]int, ox, oy, rot int) {
	for _, p := range pattern {
		dx, dy := p[0], p[1]
		for r := 0; r < (rot & 3); r++ {
			dx, dy = 2-dy, dx // rotate 90° within a 3×3 box
		}
		x, y := ox+dx, oy+dy
		if x >= 0 && x < l.bw && y >= 0 && y < l.bh {
			l.alive[y*l.bw+x] = true
		}
	}
}

// Update advances the board to absolute frame `tick`, one frame at a time: it steps a
// Life generation every ticksPerGen frames and cools the heat field on every frame in
// between. It only ever moves forward, so calling Update(N) twice (or replaying an
// earlier tick) never re-advances — it is idempotent for a given tick. Advancing frame
// by frame is what makes the trails smooth; in the live loop tick increments by one, so
// each call does exactly one frame.
func (l *Life) Update(tick int) {
	if l.bw == 0 || l.bh == 0 {
		if tick > l.frame {
			l.frame = tick
		}
		return
	}
	for l.frame < tick {
		l.frame++
		l.advanceFrame()
	}
}

// advanceFrame steps the sim by one frame: a Life generation on the beat, then — every
// frame — the heat update (a newborn ignites toward white, a living cell eases toward its
// age-cooled sustain, a dead cell cools) and the halo rebuilt from it.
func (l *Life) advanceFrame() {
	if l.frame%ticksPerGen == 0 {
		l.step()
		l.confine()
	}
	for i := range l.heat {
		if l.alive[i] {
			// A just-born cell (age 0) targets the white-hot spark and RISES to it fast, so a
			// birth ignites over a couple of frames instead of flashing on in one. An older cell
			// targets its age-cooled sustain and eases DOWN off that peak — young cells glow hot,
			// a long-settled still-life dims to a low ember.
			target, ease := 1.0, riseEase
			if l.age[i] > 0 {
				a := float64(l.age[i]) / ageFull
				if a > 1 {
					a = 1
				}
				target = sustainHot + (sustainCold-sustainHot)*a
				ease = sustainEase
			}
			l.heat[i] += (target - l.heat[i]) * ease
		} else {
			l.heat[i] *= deathDecay
		}
	}
	l.bloom()
}

// bloom rebuilds the halo: the heat field blurred, which at renders time is composited
// back over the sharp field (see at). Two passes of a separable box blur approximate a
// Gaussian closely enough at this radius and cost two adds per pixel per pass. The kernel
// is identical on both axes, so blurring a D4-symmetric field leaves it D4-symmetric —
// the mandala's symmetry survives the glow (TestSymmetric checks this).
func (l *Life) bloom() {
	if bloomGain <= 0 || len(l.glow) == 0 {
		return
	}
	src := l.heat
	for pass := 0; pass < bloomPasses; pass++ {
		blurH(src, l.blurTmp, l.bw, l.bh, bloomRadius)
		blurV(l.blurTmp, l.glow, l.bw, l.bh, bloomRadius)
		src = l.glow
	}
}

// blurLine box-blurs one line of the board with a sliding window: it carries a running
// sum and moves it one step at a time, so the cost is O(1) per pixel whatever the radius
// (the obvious re-add-every-tap version measured ~8ms a frame on a large pane — enough to
// eat the 30fps budget on its own). base is the first element's index, n the line's
// length, stride the step between elements: 1 along a row, bw down a column. Taps off the
// end are simply absent from the sum, which fades the board's outermost pixels — they sit
// outside the disc, already black under the vignette.
func blurLine(src, dst []float64, base, n, stride, r int) {
	if n == 0 {
		return
	}
	norm := 1 / float64(2*r+1)
	sum := 0.0
	for k := 0; k <= r && k < n; k++ {
		sum += src[base+k*stride]
	}
	for x := 0; x < n; x++ {
		dst[base+x*stride] = sum * norm
		if add := x + r + 1; add < n {
			sum += src[base+add*stride]
		}
		if drop := x - r; drop >= 0 {
			sum -= src[base+drop*stride]
		}
	}
}

// blurH blurs every row of a w×h field; blurV every column.
func blurH(src, dst []float64, w, h, r int) {
	for y := 0; y < h; y++ {
		blurLine(src, dst, y*w, w, 1, r)
	}
}

func blurV(src, dst []float64, w, h, r int) {
	for x := 0; x < w; x++ {
		blurLine(src, dst, x, h, w, r)
	}
}

// confine forces every pixel outside the live disc to dead, clearing its age and heat too. It
// runs after each generation in the ambient path — never inside step, which stays pure B3/S23
// so the canonical blinker/glider tests keep exercising unconfined Life. Intersecting the board
// with the D4-symmetric disc preserves the field's symmetry while keeping growth out of the
// square board's corners (where the diagonal reflection is no longer exact), shaping the field
// into the disc that the vignette fades to black at the corners.
func (l *Life) confine() {
	r2 := l.rad * l.rad
	for y := 0; y < l.bh; y++ {
		ry := y - l.cy
		for x := 0; x < l.bw; x++ {
			rx := x - l.cx
			if rx*rx+ry*ry > r2 {
				i := y*l.bw + x
				l.alive[i] = false
				l.age[i] = 0
				l.heat[i] = 0
			}
		}
	}
}

// step advances the board exactly one generation under B3/S23 with no wraparound, marking
// each newborn as age 0 (the per-frame heat update then ignites it toward white-hot), and —
// if the field has gone stagnant for long enough — injecting a fresh patch so it never dies
// to a static end-state.
func (l *Life) step() {
	changed := 0
	pop := 0
	for y := 0; y < l.bh; y++ {
		for x := 0; x < l.bw; x++ {
			n := l.liveNeighbours(x, y)
			i := y*l.bw + x
			was := l.alive[i]
			alive := (was && (n == 2 || n == 3)) || (!was && n == 3)
			l.next[i] = alive
			if alive {
				pop++
			}
			switch {
			case alive && !was: // birth
				// No instant spark here: the per-frame heat update ignites a newborn (age 0) UP
				// toward white over the next few frames, so a birth blooms rather than flashes.
				l.ageNext[i] = 0
			case alive && was: // survival
				a := l.age[i] + 1
				if a > ageCap {
					a = ageCap
				}
				l.ageNext[i] = a
			default: // dead
				l.ageNext[i] = 0
			}
			if alive != was {
				changed++
			}
		}
	}
	l.alive, l.next = l.next, l.alive
	l.age, l.ageNext = l.ageNext, l.age
	l.gen++

	// Rebloom the disc when the mandala has decayed — either frozen (few cells changing) or
	// thinned below the fill floor — so it never sits static or near-empty. Keyed on the
	// generation count, so replay reproduces.
	discArea := math.Pi * float64(l.rad*l.rad)
	stale := changed < int(stagnFrac*float64(l.bw*l.bh))
	thin := float64(pop) < popFloorFrac*discArea
	if stale || thin {
		l.stagn++
	} else {
		l.stagn = 0
	}
	if l.stagn >= stagnStreak {
		l.reseedRegion()
		l.stagn = 0
	}
}

// liveNeighbours counts the up-to-8 live cells around (x, y). No wraparound: cells off
// the board count as dead, so patterns die or reflect at the walls rather than
// tunnelling to the far edge.
func (l *Life) liveNeighbours(x, y int) int {
	n := 0
	for dy := -1; dy <= 1; dy++ {
		yy := y + dy
		if yy < 0 || yy >= l.bh {
			continue
		}
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			xx := x + dx
			if xx < 0 || xx >= l.bw {
				continue
			}
			if l.alive[yy*l.bw+xx] {
				n++
			}
		}
	}
	return n
}

// reseedRegion re-blooms the whole disc with a fresh symmetric soup once the field has gone
// stagnant, so a settled mandala springs back to life instead of freezing into a static
// symmetric still-life. It clears the disc first — a partial reseed would be asymmetric — and
// seeds with a per-generation salt so successive blooms differ. Keyed on gen, so replay
// reproduces.
func (l *Life) reseedRegion() {
	if l.rad < 3 {
		return
	}
	// Clear the disc's cells and their ages but LEAVE the heat field intact: the fresh cells
	// then glow up toward sustain over the next few frames while the cleared cells' trails keep
	// cooling, so the rebloom fades in continuously instead of snapping the whole disc to a new
	// colour. (The cold opening in reset still sparks, since it has no field to fade in over.)
	for y := l.cy - l.rad; y <= l.cy+l.rad; y++ {
		for x := l.cx - l.rad; x <= l.cx+l.rad; x++ {
			if !l.inDisc(x, y) {
				continue
			}
			i := y*l.bw + x
			l.alive[i] = false
			l.age[i] = 0
		}
	}
	l.seedSymmetric(reseedSalt ^ (uint32(l.gen) * 2654435761))
}

// Done reports whether a one-shot has resolved. Life is an ambient loop, not a
// one-shot, so it is always false — it reseeds rather than ending.
func (l *Life) Done() bool { return false }

// View renders the current heat field into a w×h pane as half-blocks: exactly h lines
// of w cells, or "" for a degenerate pane. Each cell packs two stacked pixels — fg is
// the top pixel's ember colour, bg the bottom's — times the radial vignette. The pane is
// a centred window into the square sim board (odd side ≥ max(w, 2h)), so on a wide pane it shows
// a full-width band through the middle of a frame-filling mandala, the far tips cropped and
// the corners faded to black. If the pane differs from the constructed size it re-seeds at
// the new size (an Animation owns its dimensions — references skill §B). Rendering does not
// mutate state, so View is stable across repeated calls at a fixed size for a fixed tick.
func (l *Life) View(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	if w != l.w || h != l.h {
		l.reset(w, h)
	}

	// Offset each pane cell into the centred window on the square board.
	ox, oy := (l.bw-w)/2, (l.bh-2*h)/2

	var b strings.Builder
	// Each cell is \x1b[38;2;…;48;2;…m▀ — at most 39 bytes; each row adds a reset and a
	// newline.
	b.Grow(w*h*40 + h*5)

	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			px := ox + c
			pyTop, pyBot := oy+2*r, oy+2*r+1
			top := ember(l.at(px, pyTop), l.vigRadial(float64(px)+0.5, float64(pyTop)+0.5))
			bot := ember(l.at(px, pyBot), l.vigRadial(float64(px)+0.5, float64(pyBot)+0.5))
			// Screen-locked Bayer dither of the low bit: the halo and the vignette paint wide
			// smooth gradients, which band visibly at 8 bits per channel. Keyed on screen
			// position, so a given value always dithers the same way and the gradient never
			// shimmers or crawls as the field moves.
			dt := ((bayer4[(2*r)&3][c&3]+0.5)/16 - 0.5) / 255.0
			db := ((bayer4[(2*r+1)&3][c&3]+0.5)/16 - 0.5) / 255.0
			appendCell(&b, chan8(top.r+dt), chan8(top.g+dt), chan8(top.b+dt),
				chan8(bot.r+db), chan8(bot.g+db), chan8(bot.b+db))
		}
		b.WriteString("\x1b[0m")
		if r < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// at returns the lit value at pixel (x, y) — the sharp heat with its halo composited over
// it — or 0 outside the grid. The composite is a screen blend rather than a clamped sum:
// the halo can only ever lift a pixel toward white and never overshoots, so raising the
// gain deepens the glow instead of flattening the core into a white disc (a clamped sum
// blows out past a gain of about 1.5, where the screen blend still reads at 2).
func (l *Life) at(x, y int) float64 {
	if x < 0 || x >= l.bw || y < 0 || y >= l.bh {
		return 0
	}
	i := y*l.bw + x
	return 1 - (1-clamp01(l.heat[i]))*(1-clamp01(bloomGain*l.glow[i]))
}

// ember maps a lit value onto the glass ramp. The vignette is applied to the ramp
// COORDINATE first (so fading toward the rim walks the palette down through gold, ruby,
// magenta and violet to cobalt — the concentric glass banding) and only then as a gentle
// luminance falloff, which is what still carries the corners to true black.
func ember(lit, vg float64) rgb {
	c := palette(lit * (1 - vigTone + vigTone*vg))
	k := vg
	if lumPow != 1 {
		k = math.Pow(vg, lumPow)
	}
	return rgb{c.r * k, c.g * k, c.b * k}
}

// rampSteps is the resolution of the baked palette. The output is 8-bit per channel and
// dithered, so 512 steps are far finer than anything that can survive quantization.
const rampSteps = 512

// rampLUT is glassStops baked into a table once at init. Interpolating in OKLab costs a
// dozen-odd transcendentals per sample — far too much to run twice per cell per frame
// (measured: ~3× the whole frame budget) — and the ramp is a pure function of one scalar,
// so it is evaluated once here and indexed thereafter.
var rampLUT = buildRamp()

func buildRamp() [rampSteps]rgb {
	var lut [rampSteps]rgb
	last := glassStops[len(glassStops)-1]
	for i := range lut {
		t := float64(i) / float64(rampSteps-1)
		lut[i] = last.c
		for j := 1; j < len(glassStops); j++ {
			hi := glassStops[j]
			if t > hi.t {
				continue
			}
			lo := glassStops[j-1]
			// Smoothstep across the segment so the ramp is C1 at every stop: a plain linear
			// blend leaves a kink there, which a wide gradient shows as a Mach band.
			k := sstep((t - lo.t) / (hi.t - lo.t))
			a, b := toOklab(lo.c), toOklab(hi.c)
			lut[i] = fromOklab(oklab{lerp(a.L, b.L, k), lerp(a.a, b.a, k), lerp(a.b, b.b, k)})
			break
		}
	}
	return lut
}

// palette maps a 0..1 lit value onto the baked glass ramp.
func palette(t float64) rgb {
	return rampLUT[int(clamp01(t)*(rampSteps-1)+0.5)]
}

// oklab is a colour in OKLab: perceptual lightness plus two opponent axes. Blending there
// keeps a gradient's chroma and lightness even, where an sRGB lerp between two saturated
// stops dips through a darker, muddier midpoint.
type oklab struct{ L, a, b float64 }

func toOklab(c rgb) oklab {
	r, g, b := srgbToLin(c.r), srgbToLin(c.g), srgbToLin(c.b)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return oklab{
		L: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		a: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		b: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// fromOklab is the inverse, clamped: a blend of two in-gamut colours can leave sRGB, and
// a ramp entry has to be a real colour.
func fromOklab(c oklab) rgb {
	l := c.L + 0.3963377774*c.a + 0.2158037573*c.b
	m := c.L - 0.1055613458*c.a - 0.0638541728*c.b
	s := c.L - 0.0894841775*c.a - 1.2914855480*c.b
	l, m, s = l*l*l, m*m*m, s*s*s
	return rgb{
		r: clamp01(linToSrgb(+4.0767416621*l - 3.3077115913*m + 0.2309699292*s)),
		g: clamp01(linToSrgb(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s)),
		b: clamp01(linToSrgb(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s)),
	}
}

func srgbToLin(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linToSrgb(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// sstep is the classic smoothstep on an already-normalized 0..1 parameter.
func sstep(t float64) float64 { return t * t * (3 - 2*t) }

// vigRadial is the radial vignette: ~1 across a broad bright core, easing to 0 at the disc rim,
// so even filling the frame the mandala fades to black rather than meeting a hard edge
// (references/craft.md). It is keyed on the disc, not the pane, so the frame's corners — outside
// the disc — are pure black. x and y are pixel coordinates; distance is measured to the integer
// disc centre. vigPow is the softness knob (smaller = broader, brighter core; larger = tighter).
func (l *Life) vigRadial(x, y float64) float64 {
	if l.rad <= 0 {
		return 0
	}
	dx, dy := x-float64(l.cx), y-float64(l.cy)
	t := math.Sqrt(dx*dx+dy*dy) / float64(l.rad)
	if t >= 1 {
		return 0
	}
	return math.Pow(1-t*t, vigPow)
}

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

// chan8 clamps a 0..1 channel to a 0..255 byte.
func chan8(x float64) uint8 {
	return uint8(int(math.Round(clamp01(x) * 255)))
}

// appendCell writes one half-block cell — fg is the top pixel, bg the bottom — as an
// SGR truecolor sequence plus the upper-half-block glyph. Hand-rolled with strconv so
// the per-cell hot path carries no fmt reflection or allocation.
func appendCell(b *strings.Builder, fr, fg, fb, br, bg, bb uint8) {
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
	b.WriteString("m▀")
}

func writeChan(b *strings.Builder, v uint8) {
	var s [3]byte
	b.Write(strconv.AppendUint(s[:0], uint64(v), 10))
}
