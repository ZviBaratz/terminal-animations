package torus

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
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

// TestBrailleBits pins the U+2800 dot numbering. It is column-major for the historic
// 6-dot cell and only then appends dots 7/8 as a bottom row, so the obvious
// 1<<(row*2+col) is wrong on three of the four rows — this is the single easiest way
// to get a braille renderer subtly, silently wrong.
func TestBrailleBits(t *testing.T) {
	want := [4][2]uint8{
		{0x01, 0x08}, // dot1 dot4
		{0x02, 0x10}, // dot2 dot5
		{0x04, 0x20}, // dot3 dot6
		{0x40, 0x80}, // dot7 dot8 — the appended row
	}
	if brailleBit != want {
		t.Fatalf("brailleBit table drifted: got %v, want %v", brailleBit, want)
	}
	// Every bit must be distinct and together they must cover all eight dots.
	var all uint8
	for row := 0; row < 4; row++ {
		for col := 0; col < 2; col++ {
			b := brailleBit[row][col]
			if all&b != 0 {
				t.Fatalf("brailleBit[%d][%d] = %#x collides with an earlier dot", row, col, b)
			}
			all |= b
		}
	}
	if all != 0xFF {
		t.Fatalf("brailleBit covers %#x, want all eight dots (0xff)", all)
	}
	// And the whole block must land inside the braille range.
	if got := rune(0x2800 + int(all)); got != '⣿' {
		t.Fatalf("all dots lit = %q, want '⣿'", got)
	}
}

// TestShape: the frame contract — exactly h lines of exactly w visible cells, counted
// by RUNE (a braille glyph is three bytes), or "" for a degenerate pane.
func TestShape(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {1, 1}, {200, 50}, {13, 7}, {2, 40}}
	for _, s := range sizes {
		lines := visibleCells(Frame(s.w, s.h, 3))
		if len(lines) != s.h {
			t.Fatalf("Frame(%d,%d): got %d lines, want %d", s.w, s.h, len(lines), s.h)
		}
		for r, ln := range lines {
			if n := len([]rune(ln)); n != s.w {
				t.Fatalf("Frame(%d,%d) line %d: got %d cells, want %d", s.w, s.h, r, n, s.w)
			}
		}
	}
	for _, s := range []struct{ w, h int }{{0, 10}, {10, 0}, {0, 0}, {-4, 5}, {5, -4}} {
		if got := Frame(s.w, s.h, 1); got != "" {
			t.Fatalf("Frame(%d,%d): degenerate pane must be \"\", got %q", s.w, s.h, got)
		}
	}
}

// TestNoPanic: any (w, h, tick) must render, including tiny, zero-area and negative.
func TestNoPanic(t *testing.T) {
	for _, w := range []int{0, 1, 2, 3, 40, 120} {
		for _, h := range []int{0, 1, 2, 3, 24, 60} {
			for _, tick := range []int{-5, 0, 1, 7, 999, 100000} {
				_ = Frame(w, h, tick)
			}
		}
	}
}

// TestDeterministic: a pure Frame is byte-stable — no wall clock, no math/rand.
func TestDeterministic(t *testing.T) {
	for _, tick := range []int{0, 1, 37, 719} {
		if Frame(60, 20, tick) != Frame(60, 20, tick) {
			t.Fatalf("Frame(60,20,%d) is not byte-stable across calls", tick)
		}
	}
}

// TestLoopSeam: every time-varying term rides one phase θ at an integer harmonic, so
// tick 0 and tick period feed identical inputs. This is the one loop guarantee a
// same-machine golden cannot give (references/craft.md).
func TestLoopSeam(t *testing.T) {
	// The last two are past refSpan, so they exercise a STRETCHED period rather than
	// the base one — without them every case here would run at 720 and the
	// size-scaling path would go unchecked.
	sizes := []struct{ w, h int }{{80, 24}, {40, 12}, {1, 1}, {17, 9}, {210, 60}, {160, 45}}
	for _, s := range sizes {
		period := Period(s.w, s.h)
		if Frame(s.w, s.h, 0) != Frame(s.w, s.h, period) {
			t.Fatalf("Frame(%d,%d): seam at tick 0 vs period(%d) is not byte-identical", s.w, s.h, period)
		}
		if Frame(s.w, s.h, 0) != Frame(s.w, s.h, 2*period) {
			t.Fatalf("Frame(%d,%d): seam at tick 0 vs 2·period is not byte-identical", s.w, s.h)
		}
		if Frame(s.w, s.h, 7) != Frame(s.w, s.h, 7+period) {
			t.Fatalf("Frame(%d,%d): seam at tick 7 vs 7+period is not byte-identical", s.w, s.h)
		}
	}
}

// TestPeriodIsMinimal: `period` must be the TRUE period, not a multiple of it.
//
// This is the trap the tiltY constant exists to defeat. A torus tumbled by integer
// harmonics about two coordinate axes repeats at period/2: at that tick the
// accumulated rotation is a product of π-rotations about coordinate axes, and every
// one of those maps the torus (and its evenly spaced wire rings) onto itself, so the
// rendered frame is identical. The loop would then be half its advertised length and
// the demo GIF would contain two identical halves. The fixed oblique pre-tilt breaks
// the degeneracy; without it this test fails at period/2.
// It deliberately compares the DOT GRID (SGR stripped), not the raw frame: the
// backdrop wash also varies with θ, so a whole-frame comparison would differ on the
// background alone and pass no matter what the torus did.
//
// It also asserts a large FRACTION of cells differ rather than mere inequality.
// Exact inequality is not enough: sin(π) is 1.22e-16 rather than 0, so even a
// perfectly degenerate half-period render differs in a handful of dots. Measured, the
// degenerate case (tiltY = 0) differs in 3.8% of lit cells while the real one differs
// in 96% — so a 25% floor separates them cleanly with room to spare.
func TestPeriodIsMinimal(t *testing.T) {
	const w, h = 80, 24
	const minDiff = 0.25
	base := visibleCells(Frame(w, h, 0))
	for _, div := range []int{2, 3, 4} {
		period := Period(w, h)
		other := visibleCells(Frame(w, h, period/div))
		diff, lit := 0, 0
		for r := range base {
			a, b := []rune(base[r]), []rune(other[r])
			for c := range a {
				if a[c] != ' ' || b[c] != ' ' {
					lit++
				}
				if a[c] != b[c] {
					diff++
				}
			}
		}
		if lit == 0 {
			t.Fatalf("Frame(%d,%d): nothing drawn at tick 0 or period/%d", w, h, div)
		}
		if frac := float64(diff) / float64(lit); frac < minDiff {
			t.Fatalf("Frame(%d,%d): only %.1f%% of lit cells differ between tick 0 and "+
				"period/%d (want >=%.0f%%) — the true period is period/%d, not %d. The "+
				"oblique pre-tilt (tiltY) is not breaking the torus's rotational symmetry.",
				w, h, frac*100, div, minDiff*100, div, period)
		}
	}
}

// TestPeriodScales pins the loop-length contract. The value 720 at 100×28 is not
// cosmetic: the demo recording recipe in README.md dumps exactly 720 ticks at that
// pane, so if this drifts the GIF stops closing on itself.
func TestPeriodScales(t *testing.T) {
	if got := Period(100, 28); got != basePeriod {
		t.Fatalf("Period(100,28) = %d, want %d — the recording recipe assumes this", got, basePeriod)
	}
	// Never faster than the base loop, however small the pane.
	for _, s := range []struct{ w, h int }{{1, 1}, {20, 6}, {44, 13}, {80, 24}, {0, 0}, {-3, 9}} {
		if got := Period(s.w, s.h); got != basePeriod {
			t.Fatalf("Period(%d,%d) = %d, want %d — small panes must not be sped up",
				s.w, s.h, got, basePeriod)
		}
	}
	// Past the reference the loop stretches, and monotonically.
	prev := basePeriod
	for _, s := range []struct{ w, h int }{{120, 32}, {160, 45}, {210, 60}, {260, 70}} {
		got := Period(s.w, s.h)
		if got <= basePeriod {
			t.Fatalf("Period(%d,%d) = %d, want > %d — a large pane must slow down",
				s.w, s.h, got, basePeriod)
		}
		if got < prev {
			t.Fatalf("Period(%d,%d) = %d went backwards from %d — must be monotonic in pane size",
				s.w, s.h, got, prev)
		}
		prev = got
	}
	// It scales with the SHORT axis in dots, so a pane that is merely wide does not
	// slow down — the torus is fitted to the short axis and has not grown.
	if Period(400, 28) != Period(100, 28) {
		t.Fatalf("Period(400,28) = %d, want %d — extra width alone does not scale the torus",
			Period(400, 28), Period(100, 28))
	}
}

// TestFramesDiffer catches a dead animation — one that renders but never moves.
func TestFramesDiffer(t *testing.T) {
	const w, h = 80, 24
	if Frame(w, h, 0) == Frame(w, h, 1) {
		t.Fatal("Frame(80,24): ticks 0 and 1 are identical — the torus is not turning")
	}
}

// TestFitsPane: the torus is scaled to the short axis with a margin, so no lit dot may
// land on the outermost rows or columns. Unlike the golden this is portable across
// machines, and it is what catches a projection, scale or perspective-divide
// regression that still happens to produce a well-formed frame.
func TestFitsPane(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {120, 40}, {40, 40}, {200, 20}}
	// A spread of ticks across the loop, so the check sees every attitude of the tumble.
	for _, s := range sizes {
		period := Period(s.w, s.h)
		for tick := 0; tick < period; tick += period / 12 {
			lines := visibleCells(Frame(s.w, s.h, tick))
			for r, ln := range lines {
				runes := []rune(ln)
				for c, ch := range runes {
					if ch == ' ' {
						continue
					}
					if r == 0 || r == len(lines)-1 || c == 0 || c == len(runes)-1 {
						t.Fatalf("Frame(%d,%d,%d): lit cell %q at the border (row %d, col %d) "+
							"— the torus is clipping the pane", s.w, s.h, tick, ch, r, c)
					}
				}
			}
		}
	}
}

// No TestHiddenLineRemoval here, deliberately. It is tempting to assert that occlusion
// lights fewer cells than no occlusion, but summed over the whole loop at 100×28 the
// hidden-line pipeline only removes ~19% of lit cells (the far side of a sparse
// wireframe largely projects onto the same cells as the near side), and the back-face
// cull and the depth test are so redundant that disabling either alone moves the
// count only ~4% — and on some individual frames not at all (0%). Any
// threshold tight enough to catch a regression would be fragile, and one loose enough
// to be stable would catch nothing — a test that passes with the whole occluder ripped
// out is worse than no test, because it reads as coverage. Occlusion is a *visual*
// property and is verified at the beauty gate (see README.md), which is what the gate
// is for. What is pinned here instead is everything that IS crisply checkable: shape,
// no-panic, determinism, the seam, the true period, and the fit.

// TestGolden: pin the exact bytes of one frame. Run with -update to regenerate.
// Same-machine only — math.Cos/Sin and float64 rounding are not bit-identical across
// arch/OS (references/craft.md); the portable guarantees are the tests above.
func TestGolden(t *testing.T) {
	const w, h, tick = 16, 4, 5
	path := filepath.Join("testdata", "golden_16x4_t5.txt")
	got := Frame(w, h, tick)
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
		t.Fatalf("Frame(%d,%d,%d) drifted from golden %s", w, h, tick, path)
	}
}
