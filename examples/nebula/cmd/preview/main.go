// cmd/preview — the standalone preview + frame-renderer for the nebula splash
// animation. This is scripts/preview/ (main.go.tmpl + size_*.go) with render() wired
// to nebula.Frame: a live full-terminal loop plus a headless frames dump.
//
//	go run ./cmd/preview                      # live, fills the terminal, follows resizes (Ctrl-C to quit)
//	go run ./cmd/preview frames 5             # dump 5 frames at 80×24 (structure + colour check)
//	go run ./cmd/preview frames 90 12         # dump 90 frames strided by 12 (ticks 0,12,…) — shows motion
//	go run ./cmd/preview frames 90 12 200 56  # …at an explicit 200×56 pane
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ZviBaratz/terminal-animations/examples/nebula"
)

func main() {
	// nebula is a pure, free-running field: frame N is a function of (w, h, N) alone.
	render := func(w, h, tick int) (frame string, done bool) {
		return nebula.Frame(w, h, tick), false
	}

	// frames mode: a deterministic dump for the headless gate (ansi2png). Kept at a
	// fixed size by default so goldens/filmstrips are reproducible; a stride spreads
	// the ticks so a slow loop still shows motion, and the size is overridable.
	//   frames N [stride] [w h]
	if len(os.Args) > 1 && os.Args[1] == "frames" {
		n, stride, w, h := 3, 1, 80, 24
		if len(os.Args) > 2 {
			if v, err := strconv.Atoi(os.Args[2]); err == nil && v > 0 {
				n = v
			}
		}
		if len(os.Args) > 3 {
			if v, err := strconv.Atoi(os.Args[3]); err == nil && v > 0 {
				stride = v
			}
		}
		// Width and height are a pair: only override the pane when BOTH are given
		// (frames N stride W H), so a lone width can't be mistaken for a height.
		if len(os.Args) > 5 {
			wv, werr := strconv.Atoi(os.Args[4])
			hv, herr := strconv.Atoi(os.Args[5])
			if werr == nil && herr == nil && wv > 0 && hv > 0 {
				w, h = wv, hv
			}
		}
		for i := 0; i < n; i++ {
			tick := i * stride
			frame, _ := render(w, h, tick)
			fmt.Printf("--- frame %d ---\n%s\n", tick, frame)
		}
		return
	}

	// Live loop: take over the terminal screen — alternate buffer, hidden cursor —
	// and render at the real terminal size every frame (so it fills the pane and
	// reflows on resize). When stdout is not a TTY (e.g. piped), skip the screen
	// setup/teardown codes so they don't pollute the stream.
	_, _, isTTY := terminalSize(os.Stdout.Fd())
	if isTTY {
		fmt.Print("\x1b[?1049h\x1b[?25l") // enter alt screen + hide cursor
	}
	// restore is idempotent (sync.Once): the deferred call, the signal handler, and
	// the resolving-done path may each invoke it, but the teardown must run once.
	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(func() {
			if isTTY {
				fmt.Print("\x1b[?25h\x1b[?1049l") // show cursor + leave alt screen
			}
		})
	}
	defer restore()
	// A signal-terminated Go program does NOT run deferred funcs, so a bare Ctrl-C
	// would leave the alt screen up and the cursor hidden. Catch it, restore, exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		restore()
		os.Exit(0)
	}()

	lastW, lastH := 0, 0
	for tick := 0; ; tick++ {
		w, h, ok := terminalSize(os.Stdout.Fd())
		if !ok {
			w, h = 80, 24 // not a TTY (e.g. piped): fall back to the scaffold size
		}
		if w != lastW || h != lastH {
			fmt.Print("\x1b[2J") // size changed: clear stale cells before repainting
			lastW, lastH = w, h
		}
		frame, done := render(w, h, tick)
		fmt.Printf("\x1b[H%s", frame) // home + paint
		if done {
			// A resolving one-shot finished: leave the alt screen and reprint the
			// final frame in the primary buffer so it persists after we exit.
			restore()
			fmt.Println(frame)
			return
		}
		time.Sleep(time.Second / 30)
	}
}
