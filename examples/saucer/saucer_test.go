package saucer

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

// shownTick finds a tick at which the saucer is fully on screen at the given size —
// used by the sprite tests so they never depend on a hand-picked magic tick.
func shownTick(w, h int) (int, bool) {
	for tick := 0; tick < slot*4; tick++ {
		fl := flightAt(w, h, tick)
		if fl.shown && fl.cx >= float64(spriteW)/2 && fl.cx <= float64(w)-float64(spriteW)/2 {
			return tick, true
		}
	}
	return 0, false
}

// TestShape: exactly h lines of exactly w visible cells, across a spread of sizes,
// including a tick where the saucer is on screen (so the sprite can't break the width);
// "" for a degenerate pane.
func TestShape(t *testing.T) {
	on, ok := shownTick(80, 24)
	if !ok {
		t.Fatal("no on-screen tick found for 80x24 — flightAt never shows the saucer")
	}
	sizes := []struct{ w, h int }{{80, 24}, {1, 1}, {200, 50}, {13, 7}, {2, 40}}
	for _, s := range sizes {
		for _, tick := range []int{3, on} {
			lines := visibleCells(Frame(s.w, s.h, tick))
			if len(lines) != s.h {
				t.Fatalf("Frame(%d,%d,%d): got %d lines, want %d", s.w, s.h, tick, len(lines), s.h)
			}
			for r, ln := range lines {
				if n := len([]rune(ln)); n != s.w {
					t.Fatalf("Frame(%d,%d,%d) line %d: got %d cells, want %d", s.w, s.h, tick, r, n, s.w)
				}
			}
		}
	}
	for _, s := range []struct{ w, h int }{{0, 10}, {10, 0}, {0, 0}, {-4, 5}, {5, -4}} {
		if got := Frame(s.w, s.h, 1); got != "" {
			t.Fatalf("Frame(%d,%d): degenerate pane must be \"\", got %q", s.w, s.h, got)
		}
	}
}

// TestNoPanic: never panics for any (w, h, tick), including tiny, zero-area, negative,
// and large ticks that span many pass slots.
func TestNoPanic(t *testing.T) {
	for _, w := range []int{0, 1, 2, 3, 40, 120} {
		for _, h := range []int{0, 1, 2, 3, 24, 60} {
			for _, tick := range []int{-7, 0, 1, 7, 540, 999, 100000} {
				_ = Frame(w, h, tick)
			}
		}
	}
}

// TestDeterministic: the whole composite is pure — fresco.Render pinned to TrueColor,
// every other layer hash-driven — so Frame is byte-stable for a given (w, h, tick),
// including a tick with the saucer (and its beam) on screen.
func TestDeterministic(t *testing.T) {
	on, ok := shownTick(64, 20)
	if !ok {
		t.Fatal("no on-screen tick found for 64x20")
	}
	for _, tick := range []int{0, 5, 42, on, 1000} {
		if Frame(64, 20, tick) != Frame(64, 20, tick) {
			t.Fatalf("Frame(64,20,%d) is not stable across calls", tick)
		}
	}
}

// No TestLoopSeam here: saucer is free-running — the aurora advances linearly by tick and
// the saucer's timeline is a linear function of tick — so no tick reproduces an earlier
// frame and there is no seam to pin. A seamless θ-loop does; see examples/nebula.

// TestSkyLayerPresent gives the fresco integration teeth: at a normal size the parsed
// aurora must light *some* cells (the field is really there) but not *all* of them (there
// is dark sky for the stars and the saucer to read against). If the provider call broke
// and returned an empty field this would fail — the scene would silently lose its sky.
func TestSkyLayerPresent(t *testing.T) {
	const w, h = 80, 24
	grid := parseSky(w, h, 7)
	lit := 0
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if grid[r][c].lit {
				lit++
			}
		}
	}
	total := w * h
	if lit == 0 {
		t.Fatal("fresco aurora lit no cells — the provider integration is dark")
	}
	if lit == total {
		t.Fatalf("fresco aurora lit every cell (%d) — no dark sky for stars or the saucer", total)
	}
}

// TestSaucerComesAndGoes is the timeline's teeth: over one pass slot the saucer must be
// on screen for some ticks and absent for others — a flyby that punctuates a quiet sky,
// not a permanent fixture and not a saucer that never appears. Delete the gap logic (make
// it always shown) or break the window (never shown) and this goes red. It also checks
// the passes are *varied*: across several slots the saucer does not always fly the same
// direction and height, so it never reads as a loop.
func TestSaucerComesAndGoes(t *testing.T) {
	const w, h = 100, 40
	shown, absent := 0, 0
	for tick := 0; tick < slot; tick++ {
		if flightAt(w, h, tick).shown {
			shown++
		} else {
			absent++
		}
	}
	if shown == 0 {
		t.Fatal("the saucer never appears within a pass slot")
	}
	if absent == 0 {
		t.Fatal("the saucer is always on screen — there is no quiet gap between flybys")
	}

	// Variety: collect a signature per pass; more than one distinct signature means the
	// passes genuinely differ (deterministically), so it does not feel like a loop.
	sigs := map[string]bool{}
	for pass := 0; pass < 8; pass++ {
		mid := pass*slot + slot/2
		fl := flightAt(w, h, mid)
		if !fl.shown {
			// sample a few ticks around the middle to catch this pass on screen
			for d := -slot / 3; d <= slot/3; d += 10 {
				if f := flightAt(w, h, pass*slot+slot/2+d); f.shown {
					fl = f
					break
				}
			}
		}
		if fl.shown {
			dir := "L"
			if fl.cx > float64(w)/2 {
				dir = "R"
			}
			sigs[dir+string(rune('0'+int(fl.cy/6)))] = true
		}
	}
	if len(sigs) < 2 {
		t.Fatalf("passes do not vary (only %d distinct signature) — it will read as a loop", len(sigs))
	}
}

// TestSaucerPaints gives the sprite teeth: when the saucer is on screen it paints its
// solid-block hull (a glyph nothing else in the scene emits), and when it is absent that
// glyph is nowhere in the frame. Stop stamping the sprite and the on-screen count drops
// to zero — red.
func TestSaucerPaints(t *testing.T) {
	const w, h = 100, 40
	on, ok := shownTick(w, h)
	if !ok {
		t.Fatal("no on-screen tick found")
	}
	if n := strings.Count(Frame(w, h, on), "█"); n < spriteW {
		t.Fatalf("saucer on screen at tick %d but only %d block glyphs — sprite not painting", on, n)
	}
	// A tick deep in the quiet gap: the saucer (and its unique block glyph) must be gone.
	var gap int = -1
	for tick := 0; tick < slot; tick++ {
		if !flightAt(w, h, tick).shown {
			gap = tick
			break
		}
	}
	if gap < 0 {
		t.Fatal("no saucer-absent tick in the first slot")
	}
	if n := strings.Count(Frame(w, h, gap), "█"); n != 0 {
		t.Fatalf("saucer absent at tick %d but %d block glyphs present — a block leaked from elsewhere", gap, n)
	}
}

// TestStars gives the star layer teeth: the fixed star lattice is sparse (present but a
// small minority of cells) and it twinkles (a given star's brightness changes over time).
// Zero out the lattice or freeze the twinkle and this fails.
func TestStars(t *testing.T) {
	const w, h = 200, 60
	present := 0
	var sampleC, sampleR int = -1, -1
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if _, ok := isStar(c, r, 0); ok {
				present++
				if sampleC < 0 {
					sampleC, sampleR = c, r
				}
			}
		}
	}
	total := w * h
	if present == 0 {
		t.Fatal("no stars in the lattice — the night sky has no depth")
	}
	if present > total/4 {
		t.Fatalf("stars cover %d/%d cells — too dense to read as stars", present, total)
	}
	// Twinkle: the same star must vary in brightness over time.
	b0, _ := isStar(sampleC, sampleR, 0)
	bt, _ := isStar(sampleC, sampleR, 25)
	if b0 == bt {
		t.Fatalf("star (%d,%d) did not twinkle between t=0 and t=25 (both %v)", sampleC, sampleR, b0)
	}
}

// TestGolden: pin the exact bytes of one composited frame — the fresco aurora sky with
// stars, at a tick in the quiet gap. Run with -update to regenerate. (A same-machine byte
// pin, as for the other free-running example; the timeline/sprite guarantees a golden
// can't give live in the teeth tests above.)
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
