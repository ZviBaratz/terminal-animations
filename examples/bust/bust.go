// Package bust is a looping terminal animation of a classical marble bust: the matted
// subject is baked from a still PNG (a real image tool, at build time), and the *atmosphere*
// around it — drifting mist, a sweeping directional light, a lit backdrop — is synthesized
// live in Go. Replays offline; no converter at run time.
//
// It is the author-animation skill's "bake the subject, synthesize the atmosphere" pattern
// (references/tools.md §Baking, references/atmosphere-kit.md), the fix for the flat first
// cut of this example (a matted still panned in an ellipse). The design:
//
//   - Source & matte: a still of a marble bust on a watermarked white field. clean.py mattes
//     the whole bust (head + shoulders) off its background into an RGBA cut-out; the stock
//     watermark is removed and never enters the repo. See bake.sh / clean.py.
//   - Subject motion — a seamless pseudo-3D turn. A still can't show the statue's back, so
//     bake.sh warps the cut-out with a gentle perspective keystone (yaw = A·sinθ) that reads
//     as turning and loops. The N premultiplied-RGBA turn frames are stacked into frames.png.
//   - Atmosphere — synthesized here, per tick, as pure functions of a looping phase θ: a
//     lit backdrop, drifting fbm mist behind and in front of the bust, and a warm key light
//     that orbits the head (with a cool silhouette rim). None of this is baked — a light and
//     a fog that move cannot be frozen into the frames — which is the whole point of keeping
//     an alpha channel: the subject composites over a *live* scene.
//   - Fidelity tier: half blocks (▀). Each cell carries two independent 24-bit pixels
//     (fg = top, bg = bottom), so the visible grid is w × 2h truecolor pixels.
//   - Deterministic & offline: frames.png is embedded (go:embed) and decoded once at init;
//     Frame is a pure function of (w, h, tick). Byte-identical every `period` ticks — every
//     atmosphere term is a sinusoid of θ and the subject is indexed tick mod period — so the
//     loop is seamless (see bust_test.go).
package bust

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"
)

// frames.png is the baked subject sheet: the pseudo-3D turn frames stacked vertically, each
// frameW × (2*frameH) pixels, premultiplied RGBA. Regenerate with ./bake.sh (then refresh
// the golden).
//
//go:embed frames.png
var framesPNG []byte

const (
	frameW = 140 // native cell columns in a baked frame
	frameH = 70  // native cell rows    (a frame is 2*frameH = 140 pixel rows tall)

	pxPerFrame = frameW * (2 * frameH) // pixels in one baked frame
)

var (
	// pix holds every baked frame's premultiplied RGBA, frames stacked in order:
	// pix[(f*pxPerFrame + py*frameW + px)*4 + {0:R,1:G,2:B,3:A}], colour already × alpha.
	// Immutable after init, which keeps Frame pure and safe for concurrent callers.
	pix []byte
	// period is the loop length in ticks — the number of baked turn frames. Frame indexes
	// the sheet by tick mod period, so Frame(w,h,0) and Frame(w,h,period) are byte-identical.
	period int
)

// init decodes the embedded sheet once into a tight premultiplied-RGBA byte slice.
func init() {
	img, err := png.Decode(bytes.NewReader(framesPNG))
	if err != nil {
		panic("bust: decoding embedded frames.png: " + err.Error())
	}
	b := img.Bounds()
	w, htot := b.Dx(), b.Dy()
	if w != frameW || htot%(2*frameH) != 0 {
		panic("bust: frames.png has unexpected dimensions; re-run ./bake.sh")
	}
	period = htot / (2 * frameH)

	// Draw into NRGBA (straight RGBA); the PNG's bytes are our premultiplied values, and
	// NRGBA copies them verbatim — no colour-model premultiply is applied.
	buf := image.NewNRGBA(image.Rect(0, 0, w, htot))
	draw.Draw(buf, buf.Bounds(), img, b.Min, draw.Src)
	pix = make([]byte, w*htot*4)
	copy(pix, buf.Pix)
}

// ---------------------------------------------------------------------------
// Atmosphere-kit helpers (value noise + fbm + easing), lifted from examples/nebula.
// Pure functions of integer/float coordinates, so Frame stays deterministic.
// ---------------------------------------------------------------------------

// hash2 hashes an integer lattice point to a float in [0,1).
func hash2(ix, iy int) float64 {
	h := uint32(ix)*0x27d4eb2d ^ uint32(iy)*0x165667b1
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return float64(h) / float64(1<<32)
}

// valueNoise is bilinearly-interpolated lattice noise with a quintic fade, in [0,1].
func valueNoise(x, y float64) float64 {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(ix), y-float64(iy)
	ux := fx * fx * fx * (fx*(fx*6-15) + 10)
	uy := fy * fy * fy * (fy*(fy*6-15) + 10)
	a := hash2(ix, iy)
	b := hash2(ix+1, iy)
	c := hash2(ix, iy+1)
	d := hash2(ix+1, iy+1)
	return a + (b-a)*ux + (c-a)*uy + (a-b-c+d)*ux*uy
}

// fbm sums octaves of value noise (amplitude halving, frequency doubling) into [0,1].
func fbm(x, y float64, oct int) float64 {
	sum, amp, norm := 0.0, 0.5, 0.0
	for i := 0; i < oct; i++ {
		sum += amp * valueNoise(x, y)
		norm += amp
		amp *= 0.5
		x *= 2
		y *= 2
	}
	return sum / norm
}

func sstep(e0, e1, x float64) float64 {
	t := (x - e0) / (e1 - e0)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// bayer4 is the 4×4 ordered-dither matrix (values 0..15), applied per pixel so gradients
// don't band under 8-bit half-block quantization.
var bayer4 = [4][4]float64{
	{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5},
}

// ---------------------------------------------------------------------------
// Subject sampling (premultiplied RGBA of the baked turn frame).
// ---------------------------------------------------------------------------

// subj returns the premultiplied RGB and alpha (each 0..1) of one pixel of baked frame f at
// native pixel column nc, native pixel row nr (top of frame = 0). Out-of-range → fully
// transparent, i.e. the runtime backdrop shows through wherever the subject isn't.
func subj(f, nr, nc int) (r, g, b, a float64) {
	if nc < 0 || nc >= frameW || nr < 0 || nr >= 2*frameH {
		return 0, 0, 0, 0
	}
	i := (f*pxPerFrame + nr*frameW + nc) * 4
	return float64(pix[i]) / 255, float64(pix[i+1]) / 255, float64(pix[i+2]) / 255, float64(pix[i+3]) / 255
}

// alphaAt returns just the alpha (0..1) of a native subject pixel — used for the rim.
func alphaAt(f, nr, nc int) float64 {
	if nc < 0 || nc >= frameW || nr < 0 || nr >= 2*frameH {
		return 0
	}
	return float64(pix[(f*pxPerFrame+nr*frameW+nc)*4+3]) / 255
}

// ---------------------------------------------------------------------------
// Frame.
// ---------------------------------------------------------------------------

// Frame renders the scene at `tick` into exactly h lines of exactly w visible cells (or ""
// for a degenerate pane). The atmosphere (backdrop, mist, light) is a field that fills the
// whole pane; the native subject (frameW × frameH cells) is centered in it — cropped when
// the pane is smaller, showing more backdrop when larger. Each cell is a half block ▀ —
// foreground = top pixel, background = bottom — so the visible grid is w × 2h truecolor
// pixels. Pure in (w, h, tick) and byte-identical every `period` ticks.
func Frame(w, h, tick int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	f := ((tick % period) + period) % period
	theta := 2 * math.Pi * float64(f) / float64(period)

	padX := (w - frameW) / 2 // native column 0 sits at pane column padX (negative ⇒ crop)
	padY := (h - frameH) / 2
	rows2 := float64(2 * h)      // total pixel rows in the pane
	aspect := float64(w) / rows2 // pane pixel aspect, so radial fields stay circular
	sinT, cosT := math.Sin(theta), math.Cos(theta)

	// Per-frame scene parameters (functions of θ only ⇒ seamless):
	lx := 0.5 + 0.34*cosT // key light orbits the head
	ly := 0.40 + 0.12*sinT
	gx := 0.5 + 0.05*sinT // backdrop glow drifts gently
	gy := 0.44
	mdx, mdy := 0.22*sinT, 0.14*cosT      // mist-behind advection (periodic)
	fdx, fdy := 0.30*cosT, 0.05*sinT+0.30 // mist-front advection

	// shade computes the composited RGB (0..255) of one pane pixel at column c, pixel row py.
	shade := func(c, py int) (uint8, uint8, uint8) {
		fx := (float64(c) + 0.5) / float64(w)
		fy := (float64(py) + 0.5) / rows2

		// backdrop: a cool, dark, lit space — a soft glow behind the head over a deep base,
		// darkened toward the floor.
		dg := math.Hypot((fx-gx)*aspect, fy-gy)
		glow := sstep(0.72, 0.0, dg)
		floor := sstep(0.15, 1.05, fy) // 0 top → 1 bottom
		bl := 0.020 + 0.24*glow - 0.06*floor
		if bl < 0 {
			bl = 0
		}
		br := bl * 0.86
		bgr, bgg, bgb := br*0.9, br*0.98, bl*1.28 // slightly blue backdrop

		// mist behind: low-frequency fbm haze, pooling toward the floor.
		mb := fbm(fx*2.4+mdx, fy*2.4+mdy, 4)
		mb = sstep(0.46, 0.86, mb) * (0.30 + 0.70*floor) * 0.55
		bgr += mb * 0.42
		bgg += mb * 0.47
		bgb += mb * 0.58

		// subject: premultiplied colour + alpha of the baked turn frame, centered in the pane.
		nc := c - padX
		nr := py - 2*padY
		sr, sg, sb, sa := subj(f, nr, nc)

		// key light: a warm brightening that orbits — dramatic sweep across the marble.
		dl := math.Hypot((fx-lx)*aspect, fy-ly)
		key := sstep(0.92, 0.05, dl)
		gain := 0.44 + 1.05*key
		gr := gain * (1 + 0.14*key) // warmer where lit
		gg := gain * (1 + 0.03*key)
		gb := gain * (1 - 0.12*key) // cooler in shadow
		sr *= gr
		sg *= gg
		sb *= gb

		// composite the (premultiplied, relit) subject over the backdrop+mist.
		om := 1 - sa
		r := sr + bgr*om
		g := sg + bgg*om
		b := sb + bgb*om

		// rim: glow where the silhouette meets the backdrop, warm on the lit side. Sells the
		// backlight and hides the matte edge. Only meaningful in/adjacent to the native frame.
		if nc >= -1 && nc <= frameW && nr >= -1 && nr <= 2*frameH {
			aMin := math.Min(math.Min(alphaAt(f, nr, nc-1), alphaAt(f, nr, nc+1)),
				math.Min(alphaAt(f, nr-1, nc), alphaAt(f, nr+1, nc)))
			edge := sa * (1 - aMin)
			if edge > 0.01 {
				// Darken the silhouette where the light doesn't rake it, so the edge recedes
				// into the backdrop instead of reading as a uniform white cutout halo...
				dark := edge * (1 - key) * 0.55
				r *= 1 - dark
				g *= 1 - dark
				b *= 1 - dark
				// ...and add a warm rim only where the light does catch it.
				lit := edge * key * 0.9
				r += lit * 0.95
				g += lit * 0.80
				b += lit * 0.55
			}
		}

		// mist in front: thin, faster wisps that dissolve the base of the bust into fog.
		mf := fbm(fx*3.6+fdx, fy*3.6+fdy, 3)
		mf = sstep(0.55, 0.95, mf) * (0.10 + 0.85*floor) * 0.42
		r += mf * 0.66
		g += mf * 0.70
		b += mf * 0.82

		// vignette + ordered dither, then clamp to bytes.
		vig := 0.34 + 0.66*math.Pow(math.Sin(math.Pi*fx)*math.Sin(math.Pi*clamp01(fy)), 0.35)
		dd := (bayer4[py&3][c&3] - 7.5) / 255.0
		return chan8(r*vig + dd), chan8(g*vig + dd), chan8(b*vig + dd)
	}

	var b strings.Builder
	// A full cell is at most 39 bytes (\x1b[38;2;255;255;255;48;2;255;255;255m▀); each row
	// adds a 4-byte reset and a newline.
	b.Grow(w*h*39 + h*5)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			tr, tg, tb := shade(c, 2*r)
			br, bg, bb := shade(c, 2*r+1)
			appendCell(&b, tr, tg, tb, br, bg, bb)
		}
		b.WriteString("\x1b[0m")
		if r < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// chan8 clamps a 0..1 (with headroom) colour value to an 8-bit channel.
func chan8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// appendCell writes one half-block cell — foreground = top pixel, background = bottom — as
// an SGR truecolor sequence. Hand-rolled with strconv so the per-cell hot path carries no
// fmt reflection or allocation (mirrors examples/nebula).
func appendCell(b *strings.Builder, tr, tg, tb, br, bg, bb uint8) {
	b.WriteString("\x1b[38;2;")
	writeChan(b, tr)
	b.WriteByte(';')
	writeChan(b, tg)
	b.WriteByte(';')
	writeChan(b, tb)
	b.WriteString(";48;2;")
	writeChan(b, br)
	b.WriteByte(';')
	writeChan(b, bg)
	b.WriteByte(';')
	writeChan(b, bb)
	b.WriteString("m▀")
}

// writeChan appends one colour channel's decimal digits (0–255) to b.
func writeChan(b *strings.Builder, v uint8) {
	var s [3]byte
	b.Write(strconv.AppendUint(s[:0], uint64(v), 10))
}
