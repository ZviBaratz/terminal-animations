package plasma

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

// TestDeterministic: a pure Frame is byte-stable for a given (w, h, tick).
func TestDeterministic(t *testing.T) {
	for _, tick := range []int{0, 5, 42, 1000} {
		if Frame(64, 20, tick) != Frame(64, 20, tick) {
			t.Fatalf("Frame(64,20,%d) is not stable across calls", tick)
		}
	}
}

// No TestLoopSeam here: plasma is free-running — linear time, and its sine rates share no
// short common period — so no tick reproduces an earlier frame and there is no seam to pin.
// A seamless θ-loop does; see examples/nebula's TestLoopSeam. (SKILL.md §B.)

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
