// cmd/wasm — the browser harness entrypoint for the nebula animation.
//
// This is scripts/harness/main.go.tmpl with render() wired to nebula.Frame.
// Build and serve it with:
//
//	scripts/harness.sh examples/nebula      # → http://localhost:8731/?anim=nebula
//
// nebula loops every 1080 ticks, so setting the harness's compare Δ to 1080
// should render two pixel-identical panes — TestLoopSeam, made visible.
//
//go:build js && wasm

package main

import (
	"syscall/js"
	"unsafe"

	"github.com/ZviBaratz/terminal-animations/examples/nebula"
)

func main() {
	// nebula is a pure, free-running field: frame N is a function of (w, h, N).
	render := func(w, h, tick int) (frame string, done bool) {
		return nebula.Frame(w, h, tick), false
	}

	// A fixed loop, independent of the pane. Hardcoded rather than exported, since
	// the loop length is a harness concern and not part of what nebula offers a
	// caller — but that leaves this literal a hand-kept copy of the unexported
	// `period` constant in nebula.go. TestLoopSeam pins that constant, not this
	// copy, so nothing catches the two drifting apart. Change one, change both.
	period := func(w, h int) int { return 1080 }

	// renderFrame(w, h, tick, out Int32Array) -> done bool
	//
	// Decodes one frame into the caller's Int32Array (3 ints per cell: packed fg,
	// packed bg, glyph codepoint) and reports whether a resolving animation has
	// finished. The ANSI string stays the contract; this is only transport —
	// marshaling a ~360KB string across the JS boundary every frame costs far
	// more than a memcpy of the decoded grid.
	//
	// The decoded grid is held across calls rather than reallocated per frame; JS
	// is single-threaded and these calls are serialised, so it needs no guarding.
	var cells []int32
	js.Global().Set("renderFrame", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 4 {
			return false
		}
		w, h, tick := args[0].Int(), args[1].Int(), args[2].Int()
		if w <= 0 || h <= 0 {
			return false
		}

		frame, done := render(w, h, tick)

		// Reuse the grid across frames, growing only when the pane does. A fresh
		// make() per frame is ~110KB of garbage at 192x48, or >3MB/s at 30fps —
		// enough to keep the Go heap inflated and the collector busy for no gain.
		// Mirrors Pane.ensure on the JS side, which grows the same way.
		if n := w * h * int32sPerCell; cap(cells) < n {
			cells = make([]int32, n)
		} else {
			cells = cells[:n]
		}
		decode(frame, w, h, cells)

		// CopyBytesToJS wants a byte view of the int32 slice. Go/wasm is always
		// little-endian, so the JS Int32Array reads these back without a swap.
		raw := unsafe.Slice((*byte)(unsafe.Pointer(&cells[0])), len(cells)*4)
		js.CopyBytesToJS(js.Global().Get("Uint8Array").New(args[3].Get("buffer")), raw)
		return done
	}))

	// animPeriod(w, h) -> loop length in ticks, or 0 for one that never repeats.
	//
	// The viewer drives the tick slider's range and the compare Δ from this. Taking
	// the pane matters: a loop whose length scales with the pane re-derives on every
	// resize, instead of leaving a Δ that was right at one size and silently wrong
	// at the next.
	js.Global().Set("animPeriod", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return 0
		}
		w, h := args[0].Int(), args[1].Int()
		if w <= 0 || h <= 0 {
			return 0
		}
		return period(w, h)
	}))

	// Signal readiness, then park — the Go runtime must stay alive to service calls.
	js.Global().Call("onWasmReady")
	select {}
}

// --- ANSI → cells (copied verbatim; no need to edit below) -------------------

// Cell layout: 3 int32s per cell — packed fg, packed bg, glyph codepoint.
// Colours are 0xRRGGBB so JS can hand them straight to a canvas fill style.
const int32sPerCell = 3

// defaultFG/defaultBG apply before any SGR sets a colour, and after a reset.
const (
	defaultFG = 0xE5E5E5
	defaultBG = 0x000000
)

// xterm256 resolves a 256-colour index to 0xRRGGBB: the 16 basics, the 6×6×6
// cube, then the 24-step greyscale ramp. Mirrors scripts/ansi2png.py.
func xterm256(n int) int {
	basic := [16]int{
		0x000000, 0xCD0000, 0x00CD00, 0xCDCD00, 0x0000EE, 0xCD00CD, 0x00CDCD, 0xE5E5E5,
		0x7F7F7F, 0xFF0000, 0x00FF00, 0xFFFF00, 0x5C5CFF, 0xFF00FF, 0x00FFFF, 0xFFFFFF,
	}
	switch {
	case n < 16:
		return basic[n]
	case n < 232:
		n -= 16
		steps := [6]int{0, 95, 135, 175, 215, 255}
		return steps[(n/36)%6]<<16 | steps[(n/6)%6]<<8 | steps[n%6]
	default:
		v := 8 + (n-232)*10
		return v<<16 | v<<8 | v
	}
}

// applySGR folds one ESC[...m parameter list into the current fg/bg.
func applySGR(params []int, fg, bg *int) {
	for i := 0; i < len(params); i++ {
		switch p := params[i]; {
		case p == 0:
			*fg, *bg = defaultFG, defaultBG
		case p == 38 || p == 48:
			target := fg
			if p == 48 {
				target = bg
			}
			// 38;2;r;g;b (truecolor) or 38;5;n (indexed)
			if i+1 < len(params) && params[i+1] == 2 && i+4 < len(params) {
				*target = params[i+2]<<16 | params[i+3]<<8 | params[i+4]
				i += 4
			} else if i+1 < len(params) && params[i+1] == 5 && i+2 < len(params) {
				*target = xterm256(params[i+2])
				i += 2
			}
		case p >= 30 && p <= 37:
			*fg = xterm256(p - 30)
		case p >= 90 && p <= 97:
			*fg = xterm256(p - 90 + 8)
		case p >= 40 && p <= 47:
			*bg = xterm256(p - 40)
		case p >= 100 && p <= 107:
			*bg = xterm256(p - 100 + 8)
		case p == 39:
			*fg = defaultFG
		case p == 49:
			*bg = defaultBG
		}
	}
}

// decode walks the ANSI frame and fills cells[] as a row-major w×h grid.
// Unset cells keep the default colours and a space glyph, so a short frame
// (or one that ends early) renders as background rather than garbage.
func decode(frame string, w, h int, cells []int32) {
	for i := range cells {
		switch i % int32sPerCell {
		case 0:
			cells[i] = defaultFG
		case 1:
			cells[i] = defaultBG
		case 2:
			cells[i] = ' '
		}
	}

	fg, bg := defaultFG, defaultBG
	row, col := 0, 0
	params := make([]int, 0, 16)

	for i := 0; i < len(frame); {
		c := frame[i]

		// CSI ... m — the only escape this subset emits. Any other final byte is
		// consumed and ignored so an unexpected sequence can't desync the grid.
		if c == 0x1b && i+1 < len(frame) && frame[i+1] == '[' {
			j := i + 2
			params = params[:0]
			cur, hasDigit := 0, false
			for j < len(frame) {
				d := frame[j]
				if d >= '0' && d <= '9' {
					cur, hasDigit = cur*10+int(d-'0'), true
					j++
					continue
				}
				if d == ';' {
					params = append(params, cur)
					cur, hasDigit = 0, false
					j++
					continue
				}
				break
			}
			if hasDigit || len(params) > 0 {
				params = append(params, cur)
			}
			if j < len(frame) {
				if frame[j] == 'm' {
					applySGR(params, &fg, &bg)
				}
				j++
			}
			i = j
			continue
		}

		if c == '\n' {
			row++
			col = 0
			i++
			continue
		}
		if c == '\r' {
			col = 0
			i++
			continue
		}

		// Decode one UTF-8 rune the cheap way: find its length, then convert.
		size := 1
		switch {
		case c >= 0xF0:
			size = 4
		case c >= 0xE0:
			size = 3
		case c >= 0xC0:
			size = 2
		}
		if i+size > len(frame) {
			size = 1
		}
		r := []rune(frame[i : i+size])
		i += size

		if row < h && col < w && len(r) > 0 {
			base := (row*w + col) * int32sPerCell
			cells[base] = int32(fg)
			cells[base+1] = int32(bg)
			cells[base+2] = int32(r[0])
		}
		col++
	}
}
