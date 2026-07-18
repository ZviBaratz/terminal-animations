package nebula

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

// TestNoPanic: never panics for any (w, h, tick), including tiny, zero-area, negative
// and large ticks.
func TestNoPanic(t *testing.T) {
	for _, w := range []int{0, 1, 2, 3, 40, 120} {
		for _, h := range []int{0, 1, 2, 3, 24, 60} {
			for _, tick := range []int{-5, 0, 1, 7, 999, 100000} {
				_ = Frame(w, h, tick)
			}
		}
	}
}

// TestDeterministic: a pure Frame is byte-stable for a given (w, h, tick).
func TestDeterministic(t *testing.T) {
	for _, tick := range []int{0, 5, 42, 1000} {
		if Frame(64, 20, tick) != Frame(64, 20, tick) {
			t.Fatalf("Frame(64,20,%d) is not stable across calls", tick)
		}
	}
}

// TestLoopSeam: the forever-loop is truly seamless. Because every time term derives
// from θ = 2π·(tick mod period)/period, tick=0 and tick=period feed identical inputs,
// so the frames must be byte-identical — and one period later, and at an arbitrary
// offset. This is the loop the brief asks for, pinned exactly.
func TestLoopSeam(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {40, 12}, {1, 1}, {17, 9}}
	for _, s := range sizes {
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

// TestFramesDiffer: the field is actually alive — consecutive frames differ, and so do
// frames a beat apart. Guards against a dead/static animation slipping the beauty gate.
func TestFramesDiffer(t *testing.T) {
	const w, h = 80, 24
	if Frame(w, h, 0) == Frame(w, h, 1) {
		t.Fatal("Frame(80,24): tick 0 and tick 1 are identical — the field is static")
	}
	if Frame(w, h, 0) == Frame(w, h, period/4) {
		t.Fatal("Frame(80,24): tick 0 and a quarter-period later are identical — no motion")
	}
}

// TestGolden: pin the exact bytes of one frame. Run with -update to regenerate.
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
