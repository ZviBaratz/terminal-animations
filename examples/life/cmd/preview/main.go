// cmd/preview — the standalone preview + frame-renderer for the life animation. This is
// scripts/preview/ (main.go.tmpl + size_*.go) with render() wired to a life.Life: a live
// full-terminal loop plus a headless frames dump.
//
//	go run ./cmd/preview                       # live, fills the terminal, follows resizes (Ctrl-C to quit)
//	go run ./cmd/preview frames 5              # dump 5 frames at 80×24 (structure + colour check)
//	go run ./cmd/preview frames 90 3           # dump 90 frames strided by 3 — one generation per frame
//	go run ./cmd/preview frames 90 3 160 44    # …at an explicit 160×44 pane (a big field for the PNG gate)
//	go run ./cmd/preview frames 20 1 100 28 300 # …20 frames from tick 300 (window in on a later moment)
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ZviBaratz/terminal-animations/examples/life"
)

func main() {
	// life is stateful: it carries the Game-of-Life board and advances a generation
	// every few frames. Construct once (render closes over it) and drive it with the
	// absolute tick; View re-seeds itself if the pane is later resized. It never
	// resolves, so done is always false.
	l := life.New(80, 24)
	render := func(w, h, tick int) (frame string, done bool) {
		l.Update(tick)
		return l.View(w, h), l.Done()
	}

	// frames mode: a deterministic dump for the headless gate (ansi2png). Kept at a fixed
	// size by default so goldens/filmstrips are reproducible; a stride spreads the ticks so
	// each dumped frame is a fresh generation, and the size is overridable.
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
		// An optional start tick (frames N stride W H start) windows the dump onto a later
		// stretch of this stateful sim, so a rebloom or other mid-run moment can be inspected.
		start := 0
		if len(os.Args) > 6 {
			if v, err := strconv.Atoi(os.Args[6]); err == nil && v >= 0 {
				start = v
			}
		}
		// Warm-up render, discarded: life owns its dimensions and re-seeds when View is
		// asked for a pane it was not constructed at, so without this the FIRST dumped
		// frame is the fresh seed rather than the tick asked for — which silently makes a
		// one-frame dump identical at every tick, and a sweep of any constant that acts
		// through the sim's stepping identical at every value (a View-time constant still
		// moves, since View runs on the frozen seed — so the symptom looks selective).
		// Rendering once and throwing it away moves the re-seed before the dump.
		// (Same guard as scripts/preview/main.go.tmpl; pinned by TestFramesDumpIsHonest.)
		render(w, h, 0)

		for i := 0; i < n; i++ {
			tick := start + i*stride
			frame, _ := render(w, h, tick)
			fmt.Printf("--- frame %d ---\n%s\n", tick, frame)
		}
		return
	}

	// Live loop: take over the terminal screen — alternate buffer, hidden cursor — and
	// render at the real terminal size every frame (so it fills the pane and reflows on
	// resize). When stdout is not a TTY (e.g. piped), skip the screen setup/teardown codes
	// so they don't pollute the stream.
	_, _, isTTY := terminalSize(os.Stdout.Fd())
	if isTTY {
		fmt.Print("\x1b[?1049h\x1b[?25l") // enter alt screen + hide cursor
	}
	// restore is idempotent (sync.Once): the deferred call, the signal handler, and the
	// resolving-done path may each invoke it, but the teardown must run once.
	var restoreOnce sync.Once
	restore := func() {
		restoreOnce.Do(func() {
			if isTTY {
				fmt.Print("\x1b[?25h\x1b[?1049l") // show cursor + leave alt screen
			}
		})
	}
	defer restore()
	// A signal-terminated Go program does NOT run deferred funcs, so a bare Ctrl-C would
	// leave the alt screen up and the cursor hidden. Catch it, restore, exit.
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
			// A resolving one-shot finished: leave the alt screen and reprint the final
			// frame in the primary buffer so it persists after we exit.
			restore()
			fmt.Println(frame)
			return
		}
		time.Sleep(time.Second / 30)
	}
}
