package life

import (
	"flag"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "regenerate the golden frame in testdata/")

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visibleCells returns the frame's lines with all (zero-width) SGR escapes removed,
// i.e. exactly the cells the terminal draws.
func visibleCells(frame string) []string {
	if frame == "" {
		return nil
	}
	lines := strings.Split(frame, "\n")
	for i, ln := range lines {
		lines[i] = sgr.ReplaceAllString(ln, "")
	}
	return lines
}

// --- test helpers: drive the unexported board directly ----------------------

// clearBoard removes every live cell (and its heat), so a test can plant an isolated
// pattern into an otherwise empty field (New seeds soup + gliders, which these tests
// don't want).
func clearBoard(l *Life) {
	for i := range l.alive {
		l.alive[i] = false
		l.heat[i] = 0
		l.age[i] = 0
		l.ageNext[i] = 0
	}
	l.gen = 0
	l.frame = 0
	l.stagn = 0
}

// setDots marks each pixel-grid coordinate alive.
func setDots(l *Life, dots [][2]int) {
	for _, d := range dots {
		l.alive[d[1]*l.bw+d[0]] = true
	}
}

// liveSet returns the set of live pixel coordinates — the geometry, which is what the
// Life rules are about (independent of the heat/glow layer on top).
func liveSet(l *Life) map[[2]int]bool {
	s := map[[2]int]bool{}
	for y := 0; y < l.bh; y++ {
		for x := 0; x < l.bw; x++ {
			if l.alive[y*l.bw+x] {
				s[[2]int{x, y}] = true
			}
		}
	}
	return s
}

func sameSet(a, b map[[2]int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func shiftSet(s map[[2]int]bool, dx, dy int) map[[2]int]bool {
	out := map[[2]int]bool{}
	for k := range s {
		out[[2]int{k[0] + dx, k[1] + dy}] = true
	}
	return out
}

// --- tests ------------------------------------------------------------------

// TestShape: exactly h lines of exactly w visible cells, across a spread of sizes;
// "" for a degenerate pane.
func TestShape(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {1, 1}, {200, 50}, {13, 7}, {2, 40}, {150, 46}}
	for _, s := range sizes {
		l := New(s.w, s.h)
		l.Update(3)
		lines := visibleCells(l.View(s.w, s.h))
		if len(lines) != s.h {
			t.Fatalf("View(%d,%d): got %d lines, want %d", s.w, s.h, len(lines), s.h)
		}
		for r, ln := range lines {
			if n := len([]rune(ln)); n != s.w {
				t.Fatalf("View(%d,%d) line %d: got %d cells, want %d", s.w, s.h, r, n, s.w)
			}
		}
	}
	for _, s := range []struct{ w, h int }{{0, 10}, {10, 0}, {0, 0}, {-4, 5}, {5, -4}} {
		l := New(s.w, s.h)
		l.Update(1)
		if got := l.View(s.w, s.h); got != "" {
			t.Fatalf("View(%d,%d): degenerate pane must be \"\", got %q", s.w, s.h, got)
		}
	}
}

// TestNoPanic: never panics for any (w, h, tick), including tiny, zero-area, negative
// and large ticks. Unlike a pure Frame, a stateful sim costs O(tick) to advance, so
// the huge ticks — which exercise many generations and several stagnation reseeds —
// run only on small panes; every pane still sees a short advance.
func TestNoPanic(t *testing.T) {
	for _, w := range []int{0, 1, 2, 3, 40, 120} {
		for _, h := range []int{0, 1, 2, 3, 24, 60} {
			for _, tick := range []int{-5, 0, 1, 7, 30} {
				l := New(w, h)
				l.Update(tick)
				_ = l.View(w, h)
			}
		}
	}
	for _, w := range []int{0, 1, 2, 3, 5} {
		for _, h := range []int{0, 1, 2, 3, 5} {
			for _, tick := range []int{600, 3000} {
				l := New(w, h)
				l.Update(tick)
				_ = l.View(w, h)
			}
		}
	}
}

// TestHalfBlock: every visible cell is the upper-half-block glyph — the half-block
// tier packs two stacked pixels per cell (fg top, bg bottom), so the drawn rune is
// always ▀ regardless of state.
func TestHalfBlock(t *testing.T) {
	l := New(12, 5)
	l.Update(7)
	for _, ln := range visibleCells(l.View(12, 5)) {
		for _, r := range ln {
			if r != '▀' {
				t.Fatalf("visible glyph %q, want '▀' (half-block)", string(r))
			}
		}
	}
}

// TestBlinkerPeriod2: a 3-cell blinker in isolation returns to its start geometry
// after exactly 2 generations (and is genuinely different in between).
func TestBlinkerPeriod2(t *testing.T) {
	l := New(20, 8) // dot grid 40×32 — room to keep the blinker off the walls
	clearBoard(l)
	setDots(l, [][2]int{{10, 10}, {11, 10}, {12, 10}}) // horizontal bar
	start := liveSet(l)

	l.step()
	if sameSet(start, liveSet(l)) {
		t.Fatal("blinker unchanged after 1 generation — it should be vertical now")
	}
	l.step()
	if !sameSet(start, liveSet(l)) {
		t.Fatal("blinker did not return to its start geometry after 2 generations")
	}
}

// TestGliderShift: a glider returns to its original shape shifted by (1,1) after 4
// generations — the defining property of the spaceship, and a check that the rules,
// ageing and (lack of) wraparound all leave it intact.
func TestGliderShift(t *testing.T) {
	l := New(20, 10) // dot grid 40×40
	clearBoard(l)
	l.stamp(glider[:], 5, 5, 0) // south-east heading
	start := liveSet(l)

	for i := 0; i < 4; i++ {
		l.step()
	}
	got := liveSet(l)
	wantShifted := shiftSet(start, 1, 1)
	if !sameSet(wantShifted, got) {
		t.Fatalf("glider after 4 gens = %v, want start shifted by (1,1) = %v", got, wantShifted)
	}
}

// TestUpdateIdempotent: calling Update(N) twice in a row leaves the same state as a
// single call — the target generation is tick/ticksPerGen and stepping only moves
// forward, so a repeated tick never double-steps.
func TestUpdateIdempotent(t *testing.T) {
	const w, h, tick = 50, 16, 30
	a := New(w, h)
	a.Update(tick)
	once := a.View(w, h)
	a.Update(tick) // again — must be a no-op
	twice := a.View(w, h)
	if once != twice {
		t.Fatal("Update(N) called twice changed the state")
	}
	// And a fresh instance advanced to the same tick in one call matches.
	b := New(w, h)
	b.Update(tick)
	if b.View(w, h) != once {
		t.Fatal("Update(N) is not equivalent to a single advance from a fresh board")
	}
}

// TestDeterministic: two independently constructed boards advanced to the same tick
// render byte-identically — the seed, the ageing and the stagnation reseed are all
// hash/gen-keyed, never math/rand or a wall clock.
func TestDeterministic(t *testing.T) {
	for _, tick := range []int{0, 5, 42, 900} {
		a := New(48, 14)
		a.Update(tick)
		b := New(48, 14)
		b.Update(tick)
		if a.View(48, 14) != b.View(48, 14) {
			t.Fatalf("two boards at tick %d are not byte-identical", tick)
		}
	}
}

// TestDone: Life is an ambient loop, never a one-shot — Done stays false even after a
// long run.
func TestDone(t *testing.T) {
	l := New(40, 12)
	for _, tick := range []int{0, 1, 100, 5000} {
		l.Update(tick)
		if l.Done() {
			t.Fatalf("Done() = true at tick %d, want always false", tick)
		}
	}
}

// TestSymmetric: the field stays exactly symmetric under the seed's dihedral group for every
// generation — Life's rules are isotropic, so a symmetric seed can only evolve into symmetric
// states, and confining growth to the (symmetric) disc keeps it that way. This is the whole
// idea of the piece, so pin it across a spread of ticks. One board is advanced through the
// ticks in order (Update is forward-only).
func TestSymmetric(t *testing.T) {
	const w, h = 64, 32 // pixel grid 64×64, disc comfortably interior
	l := New(w, h)
	for _, tick := range []int{0, 5, 25, 130, 400} {
		l.Update(tick)
		cx, cy := l.cx, l.cy
		for p := range liveSet(l) {
			dx, dy := p[0]-cx, p[1]-cy
			// Axis mirrors (present under both D2 and D4).
			mustLive(t, l, tick, cx-dx, cy+dy)
			mustLive(t, l, tick, cx+dx, cy-dy)
			mustLive(t, l, tick, cx-dx, cy-dy)
			if symDiagonal { // diagonal transpose about the centre and its mirrors (D4)
				mustLive(t, l, tick, cx+dy, cy+dx)
				mustLive(t, l, tick, cx-dy, cy+dx)
				mustLive(t, l, tick, cx+dy, cy-dx)
				mustLive(t, l, tick, cx-dy, cy-dx)
			}
		}
	}
}

// TestGlowSymmetric: the halo is symmetric too. The blur is what the eye actually sees —
// a kernel that treated x and y differently (or one axis's edges differently) would tilt
// the mandala's light even though the live set stayed perfectly symmetric, so pin the
// blurred field under the same group as the cells.
//
// Checked over the disc, which is the visible region: the blur also bleeds a radius or two
// beyond it, out where the mirror of the board's outermost column falls off the far edge
// (the centre is bw/2, so column 0 has no counterpart) — those pixels sit outside the disc,
// where the vignette is already zero.
func TestGlowSymmetric(t *testing.T) {
	const w, h = 64, 32
	l := New(w, h)
	for _, tick := range []int{0, 5, 25, 130} {
		l.Update(tick)
		cx, cy := l.cx, l.cy
		for y := 0; y < l.bh; y++ {
			for x := 0; x < l.bw; x++ {
				g := l.glow[y*l.bw+x]
				if g == 0 || !l.inDisc(x, y) {
					continue
				}
				dx, dy := x-cx, y-cy
				mustGlow(t, l, tick, cx-dx, cy+dy, g)
				mustGlow(t, l, tick, cx+dx, cy-dy, g)
				mustGlow(t, l, tick, cx-dx, cy-dy, g)
				if symDiagonal {
					mustGlow(t, l, tick, cx+dy, cy+dx, g)
					mustGlow(t, l, tick, cx-dy, cy+dx, g)
					mustGlow(t, l, tick, cx+dy, cy-dx, g)
					mustGlow(t, l, tick, cx-dy, cy-dx, g)
				}
			}
		}
	}
}

func mustLive(t *testing.T, l *Life, tick, x, y int) {
	t.Helper()
	if x < 0 || x >= l.bw || y < 0 || y >= l.bh || !l.alive[y*l.bw+x] {
		t.Fatalf("tick %d: symmetry broken — image (%d,%d) is not live", tick, x, y)
	}
}

// mustGlow: the mirrored pixel carries the same halo. Summation order differs between a
// row pass and a column pass, so compare within a float epsilon rather than exactly.
func mustGlow(t *testing.T, l *Life, tick, x, y int, want float64) {
	t.Helper()
	if x < 0 || x >= l.bw || y < 0 || y >= l.bh {
		t.Fatalf("tick %d: glow symmetry broken — image (%d,%d) is off the board", tick, x, y)
	}
	if got := l.glow[y*l.bw+x]; math.Abs(got-want) > 1e-9 {
		t.Fatalf("tick %d: glow symmetry broken at (%d,%d): got %g, want %g", tick, x, y, got, want)
	}
}

// TestPalette: every baked ramp entry is a real colour, and the ramp climbs from cold to
// hot. Heat is the ramp coordinate and also the vignette's coordinate, so a dip would show
// as a ring of the mandala that is darker than the colder ring outside it.
func TestPalette(t *testing.T) {
	lum := func(c rgb) float64 { return 0.2126*c.r + 0.7152*c.g + 0.0722*c.b }

	// The designed stops climb strictly — that part is exact.
	for i := 1; i < len(glassStops); i++ {
		if lo, hi := lum(glassStops[i-1].c), lum(glassStops[i].c); hi <= lo {
			t.Fatalf("glassStops[%d] (lum %g) is not brighter than [%d] (lum %g)", i, hi, i-1, lo)
		}
	}

	// The baked ramp inherits it, within a tolerance: interpolating OKLab's lightness
	// linearly does not hold *relative* luminance exactly monotone where chroma swings
	// (the violet→magenta run bulges by ~2e-4, a sixteenth of one 8-bit step). Allow well
	// under a single output step; a mistyped stop would dip by orders more.
	const tol = 1.0 / 255
	prev := -1.0
	for i, c := range rampLUT {
		for _, ch := range []struct {
			name string
			v    float64
		}{{"r", c.r}, {"g", c.g}, {"b", c.b}} {
			if math.IsNaN(ch.v) || ch.v < 0 || ch.v > 1 {
				t.Fatalf("rampLUT[%d].%s = %g, want a finite value in [0,1]", i, ch.name, ch.v)
			}
		}
		if l := lum(c); l < prev-tol {
			t.Fatalf("rampLUT[%d]: luminance fell from %g to %g — the ramp must not dip", i, prev, l)
		} else if l > prev {
			prev = l
		}
	}
}

// TestGolden: pin the exact bytes of one frame. Run with -update to regenerate.
func TestGolden(t *testing.T) {
	const w, h, tick = 16, 4, 5
	path := filepath.Join("testdata", "golden_16x4_t5.txt")
	l := New(w, h)
	l.Update(tick)
	got := l.View(w, h)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run `go test -run TestGolden -update` to create): %v", err)
	}
	if got != string(want) {
		t.Fatalf("View at (%d,%d) tick %d drifted from golden %s", w, h, tick, path)
	}
}

// TestFramesDumpIsHonest: the FIRST frame of a dump must be the tick that was asked
// for, not the re-seed.
//
// cmd/preview constructs Life at 80×24 and wires render() over it, so a dump at any
// other pane hits View's resize path — which calls reset() and throws away whatever
// Update(tick) just computed. Without the discarded warm-up render in cmd/preview, the
// first dumped frame is therefore the fresh seed at *every* start tick, which is why
// this failed so quietly: nothing errors, every dump looks plausible, and a sweep of a
// taste constant comes out byte-identical at every value — reading as "this knob does
// nothing" rather than as a bug.
//
// The teeth: two one-frame dumps at well-separated start ticks. They are identical iff
// the seed is leaking through, so deleting the warm-up call from cmd/preview fails this.
// It shells out because the defect lives in main()'s wiring, not in the life package —
// testing New/Update/View directly would pass with the bug fully present.
func TestFramesDumpIsHonest(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain on PATH")
	}
	bin := filepath.Join(t.TempDir(), "preview")
	if out, err := exec.Command("go", "build", "-o", bin, "./cmd/preview").CombinedOutput(); err != nil {
		t.Fatalf("build cmd/preview: %v\n%s", err, out)
	}
	// A pane deliberately unequal to the 80×24 Life is constructed at, so View resizes.
	// The `--- frame N ---` header is dropped: it echoes the requested tick and so
	// differs even when the pixels below it are the identical re-seed — comparing whole
	// stdout would make this test pass with the bug fully present.
	dump := func(start int) string {
		args := []string{"frames", "1", "1", "100", "30", strconv.Itoa(start)}
		out, err := exec.Command(bin, args...).Output()
		if err != nil {
			t.Fatalf("preview %v: %v", args, err)
		}
		_, body, found := strings.Cut(string(out), "\n")
		if !found {
			t.Fatalf("preview %v: no frame body under the header", args)
		}
		return body
	}
	early, late := dump(3), dump(400)
	if early == late {
		t.Fatal("one-frame dumps at tick 3 and tick 400 have byte-identical pixels: the " +
			"first frame is the re-seed, not the tick asked for (warm-up render missing " +
			"in cmd/preview — see scripts/preview/main.go.tmpl)")
	}
}
