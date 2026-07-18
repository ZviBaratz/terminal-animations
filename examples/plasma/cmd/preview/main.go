// cmd/preview — the standalone preview + frame-renderer for the plasma reference
// animation. This is scripts/preview.go.tmpl with render() wired to plasma.Frame.
//
//	go run ./cmd/preview            # inner loop: live, in colour, Ctrl-C to quit
//	go run ./cmd/preview frames 5   # dump N frames to stdout (structure + colour check)
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ZviBaratz/terminal-animations/examples/plasma"
)

func main() {
	const w, h = 80, 24

	// plasma is a pure, free-running field: frame N is a function of N alone.
	render := func(tick int) (frame string, done bool) {
		return plasma.Frame(w, h, tick), false
	}

	if len(os.Args) > 1 && os.Args[1] == "frames" {
		n := 3
		if len(os.Args) > 2 {
			if v, err := strconv.Atoi(os.Args[2]); err == nil && v > 0 {
				n = v
			}
		}
		for tick := 0; tick < n; tick++ {
			frame, _ := render(tick)
			fmt.Printf("--- frame %d ---\n%s\n", tick, frame)
		}
		return
	}

	fmt.Print("\x1b[?25l") // hide cursor
	restore := func() { fmt.Print("\x1b[?25h") }
	defer restore()
	// A signal-terminated Go program does NOT run deferred funcs, so a bare Ctrl-C
	// would leave the cursor hidden. Catch it, restore, then exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		restore()
		os.Exit(0)
	}()
	for tick := 0; ; tick++ {
		frame, done := render(tick)
		fmt.Printf("\x1b[H%s", frame) // home + paint
		if done {
			fmt.Println()
			return
		}
		time.Sleep(time.Second / 30)
	}
}
