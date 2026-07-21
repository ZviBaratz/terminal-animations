// Package bust is a looping terminal animation of a classical marble bust silkscreened under a
// slow, hypnotic wash of color: one bust fills the pane, an invisible 3×3 grid divides it into
// nine color zones, and a gentle diagonal tint gradient rotates around the color wheel forever.
//
// It is the author-animation skill's "silkscreen the subject, cycle the palette" pattern
// (references/tools.md §Baking, references/palette-cycle-kit.md) — the answer to why the first
// cuts of this example fell flat. A terminal is *bad* at subtle: a photographic bust rendered
// "accurately" at half-block resolution collapses into banded hair and a muddy color cast (an
// ellipse-panned still, then a pseudo-3D turn — both underwhelmed). A terminal is *spectacular*
// at bold flat color. So we stop rendering the marble accurately and treat it as a screenprint
// source: posterize its luminance into a few flat tonal bands and recolor those bands, the way
// Warhol silkscreened Marilyn. The classical form, no longer fighting the medium, becomes the
// asset. A first pop-art cut tiled nine clashing busts; this refinement keeps the silkscreen
// but makes it *hypnotic* — one bust, coordinated color, a slow breath instead of a strobe:
//
//   - Source & matte: a still of a marble bust on a watermarked white field. clean.py mattes
//     the whole bust off its background and bakes a compact luminance + alpha asset
//     (bust_lum.png). The stock watermark is erased and never enters the repo. See clean.py.
//   - Subject: one still, fit once across the whole pane — no motion is baked, and no per-panel
//     tiling. Frame posterizes the baked luminance into four flat bands and maps each band (and
//     the silhouette's background) to a colorway ink.
//   - Grid: a 3×3 color-zone overlay with no visible seams. It never moves and never cuts the
//     bust; it only chooses which zone's colorway recolors each pixel of the one continuous
//     head, so the grid is felt purely through color.
//   - Motion — all color, no geometry. Each zone crossfades through a set of analogous colorways
//     whose base hue steps evenly around the wheel, phase-offset by position (rippleSpread keeps
//     neighbors close in hue) so a gentle recoloring gradient drifts diagonally and, over the
//     loop, rotates through every hue. Every term is a function of a looping phase, so the loop
//     is seamless (Frame(w,h,0) == Frame(w,h,period); see bust_test.go).
//   - Fidelity tier: half blocks (▀). Each cell carries two independent 24-bit pixels
//     (fg = top, bg = bottom), so the visible grid is w × 2h truecolor pixels.
//   - Deterministic & offline: bust_lum.png is embedded (go:embed), decoded once at init;
//     Frame is a pure function of (w, h, tick) — no clock, no rand, no run-time converter.
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

// bust_lum.png is the baked subject: an L+A (grayscale luminance + alpha) matte of the whole
// bust, contrast-stretched so the marble's shading fills the tonal ramp. Regenerate with
// `python3 clean.py <src.png> bust_lum.png` (then refresh the golden).
//
//go:embed bust_lum.png
var lumPNG []byte

var (
	// la holds the baked asset as tight L,A pairs: la[(y*assetW+x)*2 + {0:L,1:A}].
	// Immutable after init, which keeps Frame pure and safe for concurrent callers.
	la             []byte
	assetW, assetH int
)

// init decodes the embedded L+A asset once into a tight byte slice.
func init() {
	img, err := png.Decode(bytes.NewReader(lumPNG))
	if err != nil {
		panic("bust: decoding embedded bust_lum.png: " + err.Error())
	}
	b := img.Bounds()
	assetW, assetH = b.Dx(), b.Dy()
	if assetW <= 0 || assetH <= 0 {
		panic("bust: bust_lum.png has empty bounds; re-run clean.py")
	}
	// Draw into straight NRGBA; a grayscale source lands as R=G=B=luminance, alpha in A.
	buf := image.NewNRGBA(image.Rect(0, 0, assetW, assetH))
	draw.Draw(buf, buf.Bounds(), img, b.Min, draw.Src)
	la = make([]byte, assetW*assetH*2)
	for i := 0; i < assetW*assetH; i++ {
		la[i*2] = buf.Pix[i*4]     // luminance (R channel)
		la[i*2+1] = buf.Pix[i*4+3] // alpha
	}
}

// period is the loop length in ticks: every zone cycles through all len(colorways) colorways
// exactly once per period, so tick 0 and tick period render identically. Long on purpose — a
// full rotation around the hue wheel takes ~24s at 30fps, a slow hypnotic breath.
const period = 720

// grid is the number of color zones per axis (a 3×3 overlay on the one bust).
const grid = 3

// rippleSpread is the per-zone diagonal phase offset, in colorway-steps across the whole grid.
// Small (< 1) keeps neighboring zones close in hue, so the recoloring reads as one coordinated
// gradient drifting across the bust rather than nine clashing panels — the mellow-vs-pop lever.
const rippleSpread = 0.75

// vPlace vertically places the head in the pane (0 = top, 1 = bottom). Just above center leaves
// a little more room below for the neck and shoulder.
const vPlace = 0.42

// fill scales the contained head relative to a pure letterbox. Just under 1 leaves a thin
// breathing margin so the crown isn't flush to the pane edge; the near-square head letterboxes
// with drifting flat-color side fields on a wide pane.
const fill = 0.95

// ---------------------------------------------------------------------------
// Palette — analogous drift. Colorways whose base hue steps evenly around the wheel, so the
// crossfade drifts smoothly through every hue over one loop while any single frame stays inside
// a narrow, coordinated hue band (mellow, not clashing).
// ---------------------------------------------------------------------------

// rgb is a color in 0..255 float channels (kept float for clean interpolation).
type rgb struct{ r, g, b float64 }

// colorway maps the posterized subject to flat inks: bg fills the silhouette's outside, and
// band[0..3] color the four luminance bands from deepest shadow (0) to highlight (3).
type colorway struct {
	bg   rgb
	band [4]rgb
}

// colorwayCount is the number of hue stations around the wheel; the loop crossfades from each
// to the next, so over one period the bust rotates once through every hue.
const colorwayCount = 9

// analogousColorway builds one station at base hue h (turns, [0,1)). The four bands are an
// analogous ramp — a small hue spread, moderate saturation, rising value from a deep shadow to
// a near-white highlight — over a dark, low-saturation field of the same hue family, so the
// lit form reads against a coordinated background. hsv2rgb wraps the hue, so offsets may go
// negative or past 1 freely.
func analogousColorway(h float64) colorway {
	return colorway{
		bg: hsv2rgb(h-0.06, 0.42, 0.22),
		band: [4]rgb{
			hsv2rgb(h-0.02, 0.55, 0.32), // deepest shadow
			hsv2rgb(h, 0.66, 0.60),      // mid
			hsv2rgb(h+0.02, 0.52, 0.86), // light
			hsv2rgb(h+0.03, 0.16, 1.00), // highlight (a light tint, not pure white)
		},
	}
}

// colorways are the analogous stations, evenly spaced around the hue wheel and cyclic (the last
// crossfades back into the first, closing the loop where the wheel closes).
var colorways = func() []colorway {
	cws := make([]colorway, colorwayCount)
	for i := range cws {
		cws[i] = analogousColorway(float64(i) / colorwayCount)
	}
	return cws
}()

// ---------------------------------------------------------------------------
// Color interpolation — hue-aware, so a crossfade between two clashing colorways sweeps
// through vivid hues instead of desaturating to mud at the midpoint.
// ---------------------------------------------------------------------------

func rgb2hsv(c rgb) (h, s, v float64) {
	r, g, b := c.r/255, c.g/255, c.b/255
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	v = mx
	d := mx - mn
	if mx > 0 {
		s = d / mx
	}
	if d == 0 {
		return 0, s, v
	}
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h /= 6
	if h < 0 {
		h++
	}
	return h, s, v
}

func hsv2rgb(h, s, v float64) rgb {
	if s <= 0 {
		return rgb{v * 255, v * 255, v * 255}
	}
	h = math.Mod(h, 1)
	if h < 0 {
		h++
	}
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return rgb{r * 255, g * 255, b * 255}
}

// hueLerp blends c0→c1 at t along the shorter hue arc, keeping saturation and value high so
// the transition stays vivid. When one endpoint is (near) gray its hue is undefined, so it
// adopts the other's hue and the blend is a clean saturate/desaturate.
func hueLerp(c0, c1 rgb, t float64) rgb {
	h0, s0, v0 := rgb2hsv(c0)
	h1, s1, v1 := rgb2hsv(c1)
	if s0 < 0.05 {
		h0 = h1
	}
	if s1 < 0.05 {
		h1 = h0
	}
	dh := h1 - h0
	if dh > 0.5 {
		dh--
	} else if dh < -0.5 {
		dh++
	}
	return hsv2rgb(h0+dh*t, s0+(s1-s0)*t, v0+(v1-v0)*t)
}

func lerpColorway(a, b colorway, t float64) colorway {
	out := colorway{bg: hueLerp(a.bg, b.bg, t)}
	for i := range out.band {
		out.band[i] = hueLerp(a.band[i], b.band[i], t)
	}
	return out
}

func lerpRGB(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// ---------------------------------------------------------------------------
// Subject sampling & posterization.
// ---------------------------------------------------------------------------

// sample bilinearly reads the baked asset at pixel coordinate (fx, fy), returning luminance
// and alpha in 0..1. Out-of-range coordinates clamp to the edge.
func sample(fx, fy float64) (lum, alpha float64) {
	if fx < 0 {
		fx = 0
	} else if fx > float64(assetW-1) {
		fx = float64(assetW - 1)
	}
	if fy < 0 {
		fy = 0
	} else if fy > float64(assetH-1) {
		fy = float64(assetH - 1)
	}
	x0, y0 := int(fx), int(fy)
	x1, y1 := x0+1, y0+1
	if x1 >= assetW {
		x1 = assetW - 1
	}
	if y1 >= assetH {
		y1 = assetH - 1
	}
	tx, ty := fx-float64(x0), fy-float64(y0)
	l00, a00 := laAt(x0, y0)
	l10, a10 := laAt(x1, y0)
	l01, a01 := laAt(x0, y1)
	l11, a11 := laAt(x1, y1)
	lum = bilerp(l00, l10, l01, l11, tx, ty)
	alpha = bilerp(a00, a10, a01, a11, tx, ty)
	return
}

func laAt(x, y int) (lum, alpha float64) {
	i := (y*assetW + x) * 2
	return float64(la[i]) / 255, float64(la[i+1]) / 255
}

func bilerp(v00, v10, v01, v11, tx, ty float64) float64 {
	top := v00 + (v10-v00)*tx
	bot := v01 + (v11-v01)*tx
	return top + (bot-top)*ty
}

// posterize maps luminance 0..1 to a band index 0..3 (four flat tones — the silkscreen look).
func posterize(lum float64) int {
	b := int(lum * 4)
	if b > 3 {
		b = 3
	} else if b < 0 {
		b = 0
	}
	return b
}

// ---------------------------------------------------------------------------
// Frame.
// ---------------------------------------------------------------------------

// Frame renders the color-washed bust at `tick` into exactly h lines of exactly w visible cells
// (or "" for a degenerate pane). Each cell is a half block ▀ — foreground = top pixel,
// background = bottom — so the visible field is w × 2h truecolor pixels. Pure in (w, h, tick)
// and byte-identical every `period` ticks.
func Frame(w, h, tick int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	phase := float64(((tick%period)+period)%period) / period
	n := len(colorways)

	// Resolve each of the 3×3 zones' effective colorway once: a hue-aware crossfade between
	// consecutive colorways, indexed by a continuous phase plus a small diagonal offset so a
	// gentle recoloring gradient drifts across the bust. rippleSpread keeps neighboring zones
	// close in hue (coordinated, not clashing). Over one period the index advances by exactly n
	// for every zone, so each returns to its start ⇒ a seamless loop.
	var eff [grid][grid]colorway
	for gy := 0; gy < grid; gy++ {
		for gx := 0; gx < grid; gx++ {
			f := float64(n)*phase + rippleSpread*float64(gx+gy)
			i0f := math.Floor(f)
			frac := f - i0f
			i0 := ((int(i0f) % n) + n) % n
			i1 := (i0 + 1) % n
			eff[gy][gx] = lerpColorway(colorways[i0], colorways[i1], sstep(0, 1, frac))
		}
	}

	// Fit the whole bust once across the entire pane (w × 2h pixels); every pane pixel maps
	// through this single fit, so the bust is one continuous image. The 3×3 grid only picks which
	// zone's colorway recolors each pixel — no per-panel busts, no seams.
	paneW, paneH := float64(w), float64(2*h)
	scale := math.Min(paneW/float64(assetW), paneH/float64(assetH)) * fill
	ox := (paneW - float64(assetW)*scale) / 2
	oy := (paneH - float64(assetH)*scale) * vPlace

	// pixel returns the RGB (0..255) of one pane pixel at column c, pixel row py.
	pixel := func(c, py int) rgb {
		gx := c * grid / w
		if gx >= grid {
			gx = grid - 1
		}
		gy := py * grid / (2 * h)
		if gy >= grid {
			gy = grid - 1
		}
		cw := eff[gy][gx]

		ax := (float64(c)+0.5-ox)/scale - 0.5
		ay := (float64(py)+0.5-oy)/scale - 0.5
		if ax < -0.5 || ax > float64(assetW)-0.5 || ay < -0.5 || ay > float64(assetH)-0.5 {
			return cw.bg // outside the bust → the zone's flat background field
		}

		lum, alpha := sample(ax, ay)
		ink := cw.band[posterize(lum)]
		// A crisp-but-not-jagged silhouette: blend ink over the flat background across the edge.
		return lerpRGB(cw.bg, ink, sstep(0.35, 0.65, alpha))
	}

	var b strings.Builder
	// A full cell is at most 39 bytes (\x1b[38;2;255;255;255;48;2;255;255;255m▀); each row
	// adds a 4-byte reset and a newline.
	b.Grow(w*h*39 + h*5)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			t := pixel(c, 2*r)
			bo := pixel(c, 2*r+1)
			appendCell(&b, b8(t.r), b8(t.g), b8(t.b), b8(bo.r), b8(bo.g), b8(bo.b))
		}
		b.WriteString("\x1b[0m")
		if r < h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// sstep is a smoothstep between edges e0 and e1, clamped to [0,1].
func sstep(e0, e1, x float64) float64 {
	t := (x - e0) / (e1 - e0)
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// b8 clamps and rounds a 0..255 float channel to a byte.
func b8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// appendCell writes one half-block cell — foreground = top pixel, background = bottom — as an
// SGR truecolor sequence. Hand-rolled with strconv so the per-cell hot path carries no fmt
// reflection or allocation (mirrors examples/nebula).
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

// writeChan appends one color channel's decimal digits (0–255) to b.
func writeChan(b *strings.Builder, v uint8) {
	var s [3]byte
	b.Write(strconv.AppendUint(s[:0], uint64(v), 10))
}
