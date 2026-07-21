# The palette-cycle kit — silkscreen the subject, cycle the palette

`atmosphere-kit.md` makes a photographic subject work by *lighting* it — compositing it over
a live, moving scene. This kit is the other answer, and often the better one in a terminal:
**stop rendering the subject realistically and screenprint it.** Posterize its luminance into
a few flat tones and recolor those tones. The motion is the color, not the geometry.

**Why this beats realism here.** A terminal is *bad at subtle* and *spectacular at bold.* At
half-block resolution a photograph's gentle gradients collapse into banded mud and a muddy
color cast — the failure mode that sank the first cuts of `examples/bust`. Flat, saturated,
high-contrast color is exactly what truecolor half-blocks render best. Warhol silkscreened
Marilyn for the same reason a terminal wants this: reduce the image to a few flat inks and
the graphic power goes *up*, not down. A recognizable subject (a face, a logo, a landmark)
is the ideal source — the form survives being reduced to four tones.

The pipeline:

```
bake: matte + luminance + contrast-stretch  →  bust_lum.png (L + alpha)
run:  posterize L → band  →  colorway ink (crossfaded, per panel, per tick)  →  half blocks
```

Everything at run time is a pure function of `(x, y, tick)`, so it stays deterministic,
offline, and seamless.

## 1 · Bake only a luminance + alpha matte

No color, no motion, no lighting is baked — just the subject's tones and its silhouette. In
pure Pillow (see `examples/bust/clean.py` for the full white-on-white matte):

- **Matte** the subject off its background (flood only the true background color; keep *every*
  component above a size floor so a highlight can't split off part of the body).
- **Contrast-stretch** the subject's own tonal range to fill `0..255` — a photo's real range
  is narrow, and without the stretch the posterizer dumps everything into one band and the
  form vanishes. Map the subject's `p2..p98` luminance to `0..255`.
- **De-speckle** (median + a light blur) *before* the stretch, and downscale first: any stock
  watermark or sensor noise, amplified across a band boundary, surfaces as a jagged band edge.
  Posterization is unforgiving — verify the emitted asset by eye.
- Emit a compact **L + alpha** PNG (tens of KB). Decode it once with `go:embed`.

## 2 · Curated colorways, not algorithmic hue-spin

Hand-design the palette. A set of *deliberately clashing* flat colorways reads as pop art; an
even hue-rotation reads as a generic rainbow. Each colorway is a flat **background** plus one
ink per luminance band:

```go
type rgb struct{ r, g, b float64 } // 0..255, kept float for clean interpolation

type colorway struct {
	bg   rgb
	band [4]rgb // deepest shadow (0) → highlight (3)
}

// Consecutive entries differ strongly so a crossfade sweeps vivid hues; the set is cyclic.
var colorways = []colorway{
	{rgb{0, 199, 178}, [4]rgb{{150, 0, 90}, {255, 40, 130}, {255, 225, 40}, {255, 248, 220}}},
	{rgb{255, 45, 120}, [4]rgb{{40, 20, 110}, {150, 50, 200}, {60, 210, 230}, {250, 252, 255}}},
	// … as many as the ripple needs (bust ships nine)
}

// posterize maps luminance 0..1 to a band index — the whole silkscreen look is here.
func posterize(lum float64) int {
	b := int(lum * 4)
	if b > 3 {
		b = 3
	} else if b < 0 {
		b = 0
	}
	return b
}
```

## 3 · Crossfade in hue space, or it turns to mud

Blending two clashing colorways channel-by-channel in RGB passes through gray at the midpoint
— the death of pop. Interpolate in **HSV along the shorter hue arc**, keeping saturation and
value high, so the transition sweeps through vivid hues:

```go
func hueLerp(c0, c1 rgb, t float64) rgb {
	h0, s0, v0 := rgb2hsv(c0)
	h1, s1, v1 := rgb2hsv(c1)
	if s0 < 0.05 { // a (near) gray has no hue — adopt the other's so it saturates cleanly
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
// rgb2hsv / hsv2rgb: the standard conversions (channels 0..255 ↔ h,s,v in 0..1).
```

## 4 · Move the color, not the geometry — and loop seamlessly

Index each region's colorway by a **continuous phase plus a spatial offset**, so the
recoloring reads as a *directed wave*, not a uniform flicker. For a grid, the panel's diagonal
position `(gx+gy)` is the offset; for a single subject, use a band index or a screen
coordinate. The seam is free when the index advances by exactly `len(colorways)` over one
period:

```go
const period = 240
n := len(colorways)
phase := float64(((tick%period)+period)%period) / period // 0..1

// per panel (gx,gy): a hue-aware crossfade between consecutive colorways
f := float64(n)*phase + float64(gx+gy) // diagonal offset ⇒ the wave travels
i0 := ((int(math.Floor(f)) % n) + n) % n
i1 := (i0 + 1) % n
frac := f - math.Floor(f)
eff := lerpColorway(colorways[i0], colorways[i1], smoothstep(frac)) // hueLerp each slot
```

Over one period `f` advances by exactly `n`, so every region returns to its start and
`Frame(w,h,0) == Frame(w,h,period)` — byte-identical. Pin it with a loop-seam test.

## 5 · Compose the pixel

Per half-block pixel: fit the subject into its region, sample luminance + alpha, posterize,
pick the ink, and blend it over the flat background across the silhouette edge (a crisp but
not jagged outline):

```go
lum, alpha := sample(ax, ay)      // bilinear read of the baked L,A
ink := eff.band[posterize(lum)]
px := lerpRGB(eff.bg, ink, smoothstep2(0.35, 0.65, alpha)) // flat bg outside the subject
```

**Contain-fit for robustness.** Scale the subject to *fit* its region (letterboxed), not to
fill it — a near-square head in a wide panel would otherwise crop to an unreadable slice. The
flat background fills the margins as a color field (which is on-brand, not empty space). A
small `fill` factor (~1.15×) lets it dominate like a real portrait while cropping only a
sliver.

## Tuning knobs (sweep by eye against the `ansi2png` filmstrip)

- **the colorways** — the whole identity. Clashing and flat; design them, don't compute them.
- **`period`** — loop length; slower reads as more hypnotic.
- **posterization band count / thresholds** — four flat tones is the silkscreen sweet spot;
  more bands = more detail but less graphic punch.
- **the spatial offset** — which direction the wave travels (diagonal, radial, per-band).
- **`fill` / placement** — how much the subject dominates its region.
- **optional grain** — a `bayer4` dither at band edges for a screenprint texture (default off;
  hard edges are the authentic silkscreen look).

## When NOT to reach for this

- The subject's appeal *is* its subtle shading or true color (a sunset, a portrait's skin) —
  then light it (`atmosphere-kit.md`) or climb the resolution ladder, don't posterize it.
- A field animation with no subject — this kit is for a recognizable form reduced to flat ink.
- You want literal 3D or a genuine expression change — that needs geometry this doesn't touch.
