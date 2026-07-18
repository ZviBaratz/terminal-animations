// cmd/preview — the standalone preview + frame-renderer for the nebula splash
// animation. This is scripts/preview.go.tmpl with render() wired to nebula.Frame,
// plus live full-terminal sizing (the scaffold's fixed 80×24 only filled a corner).
//
//	go run ./cmd/preview              # live, fills the terminal, follows resizes (Ctrl-C to quit)
//	go run ./cmd/preview frames 5     # dump 5 frames at 80×24 (structure + colour check)
//	go run ./cmd/preview frames 3 160 50   # dump 3 frames at an explicit size
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ZviBaratz/terminal-animations/examples/nebula"
)

func main() {
	// frames mode: a deterministic dump for the headless gate (ansi2png). Kept at a
	// fixed size by default so goldens/filmstrips are reproducible; size is overridable.
	if len(os.Args) > 1 && os.Args[1] == "frames" {
		n, w, h := 3, 80, 24
		if len(os.Args) > 2 {
			if v, err := strconv.Atoi(os.Args[2]); err == nil && v > 0 {
				n = v
			}
		}
		if len(os.Args) > 3 {
			if v, err := strconv.Atoi(os.Args[3]); err == nil && v > 0 {
				w = v
			}
		}
		if len(os.Args) > 4 {
			if v, err := strconv.Atoi(os.Args[4]); err == nil && v > 0 {
				h = v
			}
		}
		for tick := 0; tick < n; tick++ {
			fmt.Printf("--- frame %d ---\n%s\n", tick, nebula.Frame(w, h, tick))
		}
		return
	}

	// Live loop: use the alternate screen buffer, hide the cursor, and render at the
	// real terminal size every frame (so it fills the pane and reflows on resize).
	fmt.Print("\x1b[?1049h\x1b[?25l") // enter alt screen + hide cursor
	restore := func() { fmt.Print("\x1b[?25h\x1b[?1049l") }
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
		fmt.Printf("\x1b[H%s", nebula.Frame(w, h, tick)) // home + paint
		time.Sleep(time.Second / 30)
	}
}
