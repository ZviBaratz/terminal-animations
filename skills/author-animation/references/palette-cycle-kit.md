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

## 2 · Colorways set the whole mood — clashing pop *or* analogous calm

The palette is the identity, and its *harmony* is the mood knob. Two families, opposite feels:

- **Deliberately clashing** flat colorways (consecutive entries far apart on the wheel) read as
  **pop art** — electric, aggressive, Warhol. Hand-design these; don't compute them.
- **Analogous** colorways whose base hue steps *evenly and slightly* around the wheel read as
  **hypnotic** — coordinated, mellow, a slow tint drifting through every hue. These you *can*
  compute (see below), and they don't become a "generic rainbow" **provided any single frame
  stays in a narrow hue band** — i.e. the spatial spread is small (§4). A rainbow is the failure
  where the whole screen shows the full wheel at once; a narrow window drifting slowly is not.

Each colorway is a flat **background** plus one ink per luminance band:

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

For the **analogous / hypnotic** family, generate the stations instead of hand-writing them —
step the base hue evenly around the wheel, and build each colorway as a small analogous ramp
(a slight hue spread, moderate saturation, rising value from a deep shadow to a light tint)
over a dark, low-saturation field of the same hue family, so the lit form reads against a
coordinated background (this is `examples/bust`):

```go
func analogousColorway(h float64) colorway { // h in turns; hsv2rgb wraps, so offsets may wrap
	return colorway{
		bg: hsv2rgb(h-0.06, 0.42, 0.22), // dark, muted field of the same hue family
		band: [4]rgb{
			hsv2rgb(h-0.02, 0.55, 0.32), // deepest shadow
			hsv2rgb(h, 0.66, 0.60),      // mid
			hsv2rgb(h+0.02, 0.52, 0.86), // light
			hsv2rgb(h+0.03, 0.16, 1.00), // highlight — a light tint, not pure white
		},
	}
}
var colorways = func() []colorway {
	const count = 9
	cws := make([]colorway, count)
	for i := range cws {
		cws[i] = analogousColorway(float64(i) / count) // evenly spaced, cyclic
	}
	return cws
}()
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

Index the colorway by a **continuous phase plus a spatial offset**, so the recoloring reads as a
*directed wave*, not a uniform flicker. The spatial offset can be **discrete** — a region index
like a grid's diagonal `(gx+gy)`, giving crisp tiles — or **continuous** — a screen coordinate
like `(fx+fy)`, giving a smooth gradient with no edges (see the variants below). The seam is free
when the index advances by exactly `len(colorways)` over one period:

```go
const period = 240
const rippleSpread = 1.0 // colorway-steps of spatial spread across the grid (see below)
n := len(colorways)
phase := float64(((tick%period)+period)%period) / period // 0..1

// per region (gx,gy): a hue-aware crossfade between consecutive colorways
f := float64(n)*phase + rippleSpread*float64(gx+gy) // diagonal offset ⇒ the wave travels
i0 := ((int(math.Floor(f)) % n) + n) % n
i1 := (i0 + 1) % n
frac := f - math.Floor(f)
eff := lerpColorway(colorways[i0], colorways[i1], smoothstep(frac)) // hueLerp each slot
```

Over one period `f` advances by exactly `n`, so every region returns to its start and
`Frame(w,h,0) == Frame(w,h,period)` — byte-identical. **`rippleSpread` is free of the seam**
(a constant per-region offset), so it is a pure look knob: it sets how far apart neighboring
regions sit on the wheel. `1.0` is one colorway-step per diagonal (bold, pop); a value **< 1**
keeps neighbors close in hue — a coordinated gradient rather than clashing tiles. It and
`period` are the two "hypnotic, not seizure-inducing" levers: `rippleSpread` is *spatial*
contrast, `period` is *temporal* speed. Slow the period **and** shrink the spread for a mellow
drift; raise both for a fast pop wave.

**Three structural variants — pick by how much you want the grid to *show*.**

- **Tiled subjects (bold pop):** fit the subject *inside each region* (a small copy per cell).
  Nine busts, a Warhol wall. Each cell samples its own contained subject. Pairs with a clashing
  palette and `rippleSpread ≥ 1`.
- **One subject, discrete zone overlay:** fit the subject **once across the whole pane**, so it
  is one continuous image; the region a pixel falls in only *selects* its colorway. Nine color
  treatments over one recognizable form. The diagonal metric collapses — `(gx+gy)` gives *five*
  levels, not nine distinct tiles (anti-diagonal cells share a color); use `(gy*grid+gx)` for
  nine.
- **One subject, continuous field (mellow / hypnotic):** same single fit, but the spatial offset
  is a **continuous** screen coordinate, so the colorway varies smoothly per pixel — no zones, no
  edges (`examples/bust`). Compute the crossfade per pixel; you only need the `bg` and the one
  band you sample, so it's two `hueLerp`s, not a whole colorway:

  ```go
  fx, fy := float64(c)/paneW, float64(py)/paneH // 0..1 across the pane
  f := float64(n)*phase + hueSweep*(fx+fy)      // continuous ⇒ a smooth diagonal wash
  i0 := ((int(math.Floor(f)) % n) + n) % n
  c0, c1 := colorways[i0], colorways[(i0+1)%n]
  frac := smoothstep(f - math.Floor(f))
  bg  := hueLerp(c0.bg, c1.bg, frac)
  ink := hueLerp(c0.band[posterize(lum)], c1.band[posterize(lum)], frac)
  ```

> **The load-bearing lesson (learned the hard way in `examples/bust`): a hard boundary between
> two *near-identical* colors reads as a rendering artifact, not a design.** Discrete zones only
> look intentional when neighbors differ enough to be read as distinct tiles — i.e. a **bold**
> palette. The moment you go **analogous** for a mellow, hypnotic feel, drop the zones and use a
> **continuous** field: with tiny hue steps the subject's own band structure hides the zones when
> the boundary is soft, and turns them into a visible tear when it's hard. Mellow ⇒ continuous;
> bold ⇒ discrete tiles are fine.

## 5 · Compose the pixel

Per half-block pixel: fit the subject into its region, sample luminance + alpha, posterize,
pick the ink, and blend it over the flat background across the silhouette edge (a crisp but
not jagged outline):

```go
lum, alpha := sample(ax, ay)      // bilinear read of the baked L,A
ink := eff.band[posterize(lum)]
px := lerpRGB(eff.bg, ink, smoothstep2(0.35, 0.65, alpha)) // flat bg outside the subject
```

**Contain-fit for robustness.** Scale the subject to *fit* (letterboxed), not to fill — a
near-square head in a wide pane would otherwise crop to an unreadable slice. The flat
background fills the margins as a color field (on-brand, not empty space). Fit against the
**region** for the tiled variant, or against the **whole pane** for the one-subject overlay.
A `fill` factor > 1 (~1.15×) makes a tiled subject dominate its cell like a portrait; for one
big bust keep `fill` ≈ 1.0 so the crown isn't cropped.

## Tuning knobs (sweep by eye against the `ansi2png` filmstrip)

- **the colorways** — the whole identity. Clashing flat inks → pop; evenly-stepped analogous →
  hypnotic. Pick the family for the mood.
- **`period`** — loop length (temporal speed); slower reads as more hypnotic.
- **spatial spread** — how far the hue drifts across the frame. Discrete (`rippleSpread`, per
  region): `≥ 1` makes bold, distinct tiles (pop); `< 1` tries to coordinate them but risks the
  artifact above. Continuous (`hueSweep`, per pixel): the mellow choice — a smooth gradient with
  no edges; `0` makes the whole subject one uniform hue that only drifts in time. Either way it's
  the *spatial* partner of `period` (temporal): slow the period **and** keep the spread gentle for
  a hypnotic wash; raise both for a fast pop wave.
- **posterization band count / thresholds** — four flat tones is the silkscreen sweet spot;
  more bands = more detail but less graphic punch.
- **the spatial offset** — which direction/shape the wave travels (diagonal, radial, per-band),
  and whether it tiles subjects or overlays zones on one subject (§4).
- **`fill` / placement** — how much the subject dominates (per region, or once across the pane).
- **optional grain** — a `bayer4` dither at band edges for a screenprint texture (default off;
  hard edges are the authentic silkscreen look).

## When NOT to reach for this

- The subject's appeal *is* its subtle shading or true color (a sunset, a portrait's skin) —
  then light it (`atmosphere-kit.md`) or climb the resolution ladder, don't posterize it.
- A field animation with no subject — this kit is for a recognizable form reduced to flat ink.
- You want literal 3D or a genuine expression change — that needs geometry this doesn't touch.
