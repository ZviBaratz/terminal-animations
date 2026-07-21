// Package bust is a looping terminal animation of a classical marble bust reimagined as an
// Andy-Warhol pop-art grid: the same silkscreened head tiled 3×3, each panel in a bold flat
// colorway, with a diagonal wave of recoloring rippling across the grid forever.
//
// It is the author-animation skill's "silkscreen the subject, cycle the palette" pattern
// (references/tools.md §Baking, references/palette-cycle-kit.md) — the answer to why the first
// two cuts of this example fell flat. A terminal is *bad* at subtle: a photographic bust
// rendered "accurately" at half-block resolution collapses into banded hair and a muddy color
// cast (an ellipse-panned still, then a pseudo-3D turn — both underwhelmed). A terminal is
// *spectacular* at bold flat high-contrast color. So we stop rendering the marble accurately
// and treat it as a screenprint source: posterize its luminance into a few flat tonal bands
// and recolor those bands, the way Warhol silkscreened Marilyn. The classical form, no longer
// fighting the medium, becomes the asset. The design:
//
//   - Source & matte: a still of a marble bust on a watermarked white field. clean.py mattes
//     the whole bust off its background and bakes a compact luminance + alpha asset
//     (bust_lum.png). The stock watermark is erased and never enters the repo. See clean.py.
//   - Subject: one still — no motion is baked. Frame posterizes the baked luminance into four
//     flat bands and maps each band (and the silhouette's background) to a colorway ink.
//   - Motion — all color, no geometry. The grid never moves. Each panel crossfades through a
//     curated set of Warhol-pop colorways, phase-offset by its position so a recoloring wave
//     sweeps the 3×3 diagonally. Every term is a function of a looping phase, so the loop is
//     seamless (Frame(w,h,0) == Frame(w,h,period); see bust_test.go).
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

// period is the loop length in ticks: the grid cycles through all len(colorways) colorways
// exactly once per period, so tick 0 and tick period render identically. Chosen for a slow,
// hypnotic ripple (~8s at 30fps).
const period = 240

// grid is the panel count per axis (3×3 Warhol grid).
const grid = 3

// vPlace vertically places the head within its panel (0 = top, 1 = bottom). Just above center
// leaves a little more room below for the neck and shoulder.
const vPlace = 0.40

// fill scales the contained head past a pure letterbox so it dominates the panel like a real
// Warhol portrait — a little over 1 crops only a sliver of hair/neck while filling the frame.
const fill = 1.18

// gutter is the flat matte color framing and separating the panels — a near-black that makes
// the pop colors sing.
var gutter = rgb{14, 12, 20}

// ---------------------------------------------------------------------------
// Palette — curated Classic Warhol pop colorways.
// ---------------------------------------------------------------------------

// rgb is a color in 0..255 float channels (kept float for clean interpolation).
type rgb struct{ r, g, b float64 }

// colorway maps the posterized subject to flat inks: bg fills the silhouette's outside, and
// band[0..3] color the four luminance bands from deepest shadow (0) to highlight (3).
type colorway struct {
	bg   rgb
	band [4]rgb
}

// colorways are hand-designed, deliberately clashing 60s-silkscreen palettes. Consecutive
// entries differ strongly so the diagonal crossfade sweeps through vivid hues, and the set is
// cyclic (the last crossfades back into the first).
var colorways = []colorway{
	{rgb{0, 199, 178}, [4]rgb{{150, 0, 90}, {255, 40, 130}, {255, 225, 40}, {255, 248, 220}}},   // turquoise / magenta→pink→lemon
	{rgb{255, 45, 120}, [4]rgb{{40, 20, 110}, {150, 50, 200}, {60, 210, 230}, {250, 252, 255}}}, // hot-pink / indigo→purple→cyan
	{rgb{255, 120, 20}, [4]rgb{{0, 90, 110}, {0, 190, 180}, {180, 225, 40}, {255, 250, 225}}},   // orange / teal→turquoise→lime
	{rgb{255, 222, 30}, [4]rgb{{200, 20, 50}, {255, 80, 40}, {230, 40, 170}, {255, 250, 245}}},  // lemon / crimson→orange→magenta
	{rgb{95, 40, 180}, [4]rgb{{25, 18, 35}, {220, 30, 150}, {255, 140, 30}, {255, 230, 60}}},    // purple / black→magenta→orange
	{rgb{30, 200, 225}, [4]rgb{{25, 30, 110}, {90, 70, 210}, {255, 80, 160}, {250, 252, 255}}},  // cyan / navy→violet→pink
	{rgb{150, 215, 30}, [4]rgb{{200, 20, 140}, {230, 40, 50}, {255, 140, 25}, {255, 235, 70}}},  // lime / magenta→red→orange
	{rgb{225, 25, 150}, [4]rgb{{0, 100, 120}, {0, 195, 220}, {255, 225, 50}, {255, 250, 230}}},  // magenta / teal→cyan→lemon
	{rgb{255, 95, 90}, [4]rgb{{110, 40, 170}, {255, 55, 140}, {20, 200, 180}, {255, 248, 222}}}, // coral / purple→pink→turquoise
}

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
// Grid layout.
// ---------------------------------------------------------------------------

// partition splits `total` cells into `grid` panels separated by 1-cell gutters and framed by
// a 1-cell border, returning each panel's start and size and an owner slice (owner[i] = panel
// index, or -1 for a gutter/border cell). When the pane is too small for a border+gutters it
// degrades gracefully to a bare split with no separators.
func partition(total int) (owner []int, start, size [grid]int) {
	owner = make([]int, max(total, 0))
	for i := range owner {
		owner[i] = -1
	}
	if total <= 0 {
		return
	}
	border, gut := 1, 1
	content := total - 2*border - (grid-1)*gut
	if content < grid {
		border, gut, content = 0, 0, total
	}
	base, extra := content/grid, content%grid
	pos := border
	for p := 0; p < grid; p++ {
		s := base
		if p < extra {
			s++
		}
		start[p], size[p] = pos, s
		for i := pos; i < pos+s && i < total; i++ {
			owner[i] = p
		}
		pos += s
		if p < grid-1 {
			pos += gut
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Frame.
// ---------------------------------------------------------------------------

// Frame renders the pop-art grid at `tick` into exactly h lines of exactly w visible cells (or
// "" for a degenerate pane). Each cell is a half block ▀ — foreground = top pixel, background =
// bottom — so the visible grid is w × 2h truecolor pixels. Pure in (w, h, tick) and
// byte-identical every `period` ticks.
func Frame(w, h, tick int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	phase := float64(((tick%period)+period)%period) / period
	n := len(colorways)

	// Resolve every panel's effective colorway once: a hue-aware crossfade between consecutive
	// colorways, indexed by a continuous phase plus the panel's diagonal offset. Over one period
	// the index advances by exactly n, so each panel returns to its start ⇒ a seamless loop.
	var eff [grid][grid]colorway
	for gy := 0; gy < grid; gy++ {
		for gx := 0; gx < grid; gx++ {
			f := float64(n)*phase + float64(gx+gy)
			i0f := math.Floor(f)
			frac := f - i0f
			i0 := ((int(i0f) % n) + n) % n
			i1 := (i0 + 1) % n
			eff[gy][gx] = lerpColorway(colorways[i0], colorways[i1], sstep(0, 1, frac))
		}
	}

	colOwner, colStart, colSize := partition(w)
	rowOwner, rowStart, rowSize := partition(h)

	// pixel returns the RGB (0..255) of one pane pixel at column c, pixel row py.
	pixel := func(c, py int) rgb {
		gx := colOwner[c]
		gy := rowOwner[py/2]
		if gx < 0 || gy < 0 {
			return gutter
		}
		pw, ph := colSize[gx], 2*rowSize[gy]
		if pw <= 0 || ph <= 0 {
			return gutter
		}
		lx := float64(c-colStart[gx]) + 0.5
		ly := float64(py-2*rowStart[gy]) + 0.5
		cw := eff[gy][gx]

		// contain-fit the whole head in the panel (letterboxed), so the face reads at any panel
		// aspect; the flat background fills the margins — a Warhol color field, not empty space.
		scale := math.Min(float64(pw)/float64(assetW), float64(ph)/float64(assetH)) * fill
		ox := (float64(pw) - float64(assetW)*scale) / 2
		oy := (float64(ph) - float64(assetH)*scale) * vPlace
		ax := (lx-ox)/scale - 0.5
		ay := (ly-oy)/scale - 0.5
		if ax < -0.5 || ax > float64(assetW)-0.5 || ay < -0.5 || ay > float64(assetH)-0.5 {
			return cw.bg // outside the contained head → flat background field
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
