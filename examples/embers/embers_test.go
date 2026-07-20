package embers

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

// TestShape: exactly h lines of exactly w visible cells, across a spread of sizes;
// "" for a degenerate pane.
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

// TestNoPanic: never panics for any (w, h, tick), including tiny, zero-area, and large ticks.
func TestNoPanic(t *testing.T) {
	for _, w := range []int{0, 1, 2, 3, 40, 120} {
		for _, h := range []int{0, 1, 2, 3, 24, 60} {
			for _, tick := range []int{0, 1, 7, 999, 100000} {
				_ = Frame(w, h, tick)
			}
		}
	}
}

// TestDeterministic: the whole composite is pure — fresco.Render is pinned to TrueColor
// and the ember layer is hash-driven — so Frame is byte-stable for a given (w, h, tick).
func TestDeterministic(t *testing.T) {
	for _, tick := range []int{0, 5, 42, 1000} {
		if Frame(64, 20, tick) != Frame(64, 20, tick) {
			t.Fatalf("Frame(64,20,%d) is not stable across calls", tick)
		}
	}
}

// No TestLoopSeam here: embers is free-running — the galaxy advances linearly by tick and
// the ember field scrolls linearly by tick — so no tick reproduces an earlier frame and
// there is no seam to pin. A seamless θ-loop does; see examples/nebula. (SKILL.md §B.)

// TestFieldLayerPresent gives the fresco integration teeth: at a normal size the parsed
// galaxy must light *some* cells (the field is really there) but not *all* of them (there
// is empty sky for the embers to drift across). If the provider call broke and returned an
// empty field, litFrac would be 0 and this fails — the composition would silently become
// "embers on black" without it.
func TestFieldLayerPresent(t *testing.T) {
	const w, h = 80, 24
	grid := parseField(w, h, 7)
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
		t.Fatal("fresco field lit no cells — the provider integration is dark")
	}
	if lit == total {
		t.Fatalf("fresco field lit every cell (%d) — no empty sky for the embers", total)
	}
}

// TestEmberLayerSparse gives the ember layer teeth: sampled over a normal pane at a given
// tick, some cells must carry an ember (the layer is really there) but only a minority
// (embers are a sparse drift, not a flood). If the layer vanished the count is 0; if it
// filled the pane the sparsity bound trips — either way the "layered composition" claim
// would be false, and this catches it.
func TestEmberLayerSparse(t *testing.T) {
	const w, h = 80, 24
	const tick = 7.0
	present := 0
	for py := 0; py < h; py++ {
		for c := 0; c < w; c++ {
			if emberLum(c, py, tick) > emberCut {
				present++
			}
		}
	}
	total := w * h
	if present == 0 {
		t.Fatal("no embers present — the foreground layer is empty")
	}
	if present > total/3 {
		t.Fatalf("embers cover %d/%d cells — too dense to read as a sparse drift", present, total)
	}
}

// TestGolden: pin the exact bytes of one composited frame — fresco field, dimmed, plus the
// ember layer. Run with -update to regenerate. (A same-machine byte pin, as for the other
// free-running example; the loop guarantee a golden can't give lives in the layer tests above.)
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
