# The atmosphere kit — bake the subject, synthesize the scene

`craft.md` says "layer a slow field behind the subject." This is the machinery that makes
that real, and the fix for the failure mode it exists to prevent: **a matted still, panned
in an ellipse, over a baked backdrop.** That is a photograph with a Ken Burns move — one
flat plane, nothing alive. It is the RED baseline for a subject animation, never a
deliverable.

The pattern that beats it: **bake only the subject, with an alpha channel; synthesize the
atmosphere around it at run time.** A light and a fog that *move* cannot be frozen into the
frames — the moment you bake them flat you are back to a panned photo. So the shipped
artifact keeps the subject's alpha and composites it, every tick, over a live scene:

```
backdrop (lit space)  →  mist behind  →  SUBJECT over (relit)  →  rim  →  mist in front  →  vignette + dither
```

Every atmosphere term is a pure function of the loop phase `θ = 2π·(tick mod period)/period`,
so the whole thing stays deterministic, offline, and seamless — the subject sheet is indexed
`tick mod period`, the light and mist are sinusoids of `θ`, and `Frame(w,h,0) ==
Frame(w,h,period)` still holds. The baking half — matte the subject with alpha, optionally a
pseudo-3D turn — is in `tools.md` §Baking. (This kit is the *composite-over-a-live-scene*
pattern; for the other subject technique, silkscreening, see `palette-cycle-kit.md`.)

## Why an alpha channel, and why premultiplied

The subject is baked as **premultiplied RGBA** — colour already multiplied by coverage `α`.
Two payoffs: the build-time downscale of a hard subject edge carries no dark fringe, and the
runtime composite is the one-line "over" rule `premult + backdrop·(1−α)`. Bake it with
`bilinear` (not a ringing kernel like Lanczos) so the premultiplied edge doesn't overshoot
into a bright cutout halo. Decode it straight (Go: draw into `image.NRGBA`, whose bytes are
your premultiplied values verbatim — no colour-model premultiply is re-applied).

## The kit (paste, then tune the constants by eye)

Noise + easing — lifted from `examples/nebula`; pure functions of coordinates, so `Frame`
stays snapshot-testable:

```go
func hash2(ix, iy int) float64 { // integer lattice → [0,1)
	h := uint32(ix)*0x27d4eb2d ^ uint32(iy)*0x165667b1
	h ^= h >> 15; h *= 0x2c1b3c6d; h ^= h >> 12; h *= 0x297a2d39; h ^= h >> 15
	return float64(h) / float64(1<<32)
}
func valueNoise(x, y float64) float64 { // bilinear lattice noise, quintic fade, [0,1]
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(ix), y-float64(iy)
	ux := fx * fx * fx * (fx*(fx*6-15) + 10)
	uy := fy * fy * fy * (fy*(fy*6-15) + 10)
	a, b, c, d := hash2(ix, iy), hash2(ix+1, iy), hash2(ix, iy+1), hash2(ix+1, iy+1)
	return a + (b-a)*ux + (c-a)*uy + (a-b-c+d)*ux*uy
}
func fbm(x, y float64, oct int) float64 { // summed octaves → [0,1]
	sum, amp, norm := 0.0, 0.5, 0.0
	for i := 0; i < oct; i++ {
		sum += amp * valueNoise(x, y); norm += amp; amp *= 0.5; x *= 2; y *= 2
	}
	return sum / norm
}
func sstep(e0, e1, x float64) float64 { // smoothstep
	t := (x - e0) / (e1 - e0)
	if t < 0 { t = 0 } else if t > 1 { t = 1 }
	return t * t * (3 - 2*t)
}
var bayer4 = [4][4]float64{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
```

A **moving key light** — a warm directional gain that orbits on `θ`; this is the "dramatic
lighting" lever. It returns per-channel gains (warmer where lit, cooler in shadow) plus the
raw `key` term so the rim can reuse it:

```go
func movingLight(fx, fy, aspect, theta float64) (gr, gg, gb, key float64) {
	lx := 0.5 + 0.34*math.Cos(theta) // light orbits the subject on the loop phase
	ly := 0.40 + 0.12*math.Sin(theta)
	dl := math.Hypot((fx-lx)*aspect, fy-ly) // aspect keeps the pool circular on any pane
	key = sstep(0.92, 0.05, dl)
	gain := 0.44 + 1.05*key                        // 0.44 ambient floor → dramatic falloff
	return gain * (1 + 0.14*key), gain * (1 + 0.03*key), gain * (1 - 0.12*key), key
}
```

The **layered composite**, one pane pixel `(c, py)`. This *is* the ordering above; read it
as the recipe:

```go
fx := (float64(c) + 0.5) / float64(w)
fy := (float64(py) + 0.5) / rows2          // rows2 = 2*h; aspect := float64(w) / rows2
floor := sstep(0.15, 1.05, fy)             // 0 at top → 1 at the base, for pooling mist

// 1 · backdrop: a cool, dark, lit space — a soft glow drifting behind the subject.
dg := math.Hypot((fx-gx)*aspect, fy-gy)    // gx,gy: glow centre, gx drifts with sin(θ)
glow := sstep(0.72, 0.0, dg)
bl := 0.020 + 0.24*glow - 0.06*floor
if bl < 0 { bl = 0 }
br, bg, bb := bl*0.86*0.9, bl*0.86*0.98, bl*1.28   // a touch blue

// 2 · mist behind: low-frequency fbm haze, advected periodically, pooling low.
mb := sstep(0.46, 0.86, fbm(fx*2.4+mdx, fy*2.4+mdy, 4)) * (0.30 + 0.70*floor) * 0.55
br += mb * 0.42; bg += mb * 0.47; bb += mb * 0.58

// 3 · subject "over", relit: sample the premultiplied subject frame at the centered native px.
sr, sg, sb, sa := subj(f, py-2*padY, c-padX)   // 0,0,0,0 where the subject isn't
gr, gg, gbl, key := movingLight(fx, fy, aspect, theta)
sr *= gr; sg *= gg; sb *= gbl
m := 1 - sa
r, g, b := sr+br*m, sg+bg*m, sb+bb*m

// 4 · rim: darken the silhouette where the light DOESN'T rake it (so the edge recedes into
// the backdrop instead of reading as a uniform cutout halo), warm glow only where it does.
if edge := sa * (1 - aMinNeighbour); edge > 0.01 {
	dark := edge * (1 - key) * 0.55
	r *= 1 - dark; g *= 1 - dark; b *= 1 - dark
	lit := edge * key * 0.9
	r += lit * 0.95; g += lit * 0.80; b += lit * 0.55
}

// 5 · mist in front: thin, faster wisps that dissolve the base into fog.
mf := sstep(0.55, 0.95, fbm(fx*3.6+fdx, fy*3.6+fdy, 3)) * (0.10 + 0.85*floor) * 0.42
r += mf * 0.66; g += mf * 0.70; b += mf * 0.82

// 6 · vignette + ordered dither, then clamp to bytes.
vig := 0.34 + 0.66*math.Pow(math.Sin(math.Pi*fx)*math.Sin(math.Pi*clamp01(fy)), 0.35)
dd := (bayer4[py&3][c&3] - 7.5) / 255.0
// → chan8(r*vig+dd), chan8(g*vig+dd), chan8(b*vig+dd)
```

`subj` reads premultiplied RGBA from the baked sheet and returns `0,0,0,0` out of range, so
the backdrop shows wherever the subject isn't. `aMinNeighbour` is the min alpha of the four
neighbours — high inside, low at the silhouette, so `sa·(1−aMin)` isolates the edge for the
rim. Emit two pixels per cell (`py = 2r` top, `2r+1` bottom) as a half block `▀` with the
shared `appendCell`/`writeChan`/`chan8` emitter every example already copies.

## The knobs (sweep by eye, per `craft.md` — never by arithmetic)

The scene is defined by these taste constants; change one at a time against the `ansi2png`
filmstrip:

- **light** — `0.44` ambient floor + `1.05` key gain (contrast/drama), orbit radius `0.34`,
  warm/cool tint `±0.14 / −0.12`, falloff `sstep(0.92, 0.05, …)` (soft vs hard key).
- **mist** — behind: freq `2.4`, gate `sstep(0.46, 0.86)`, amount `0.55`; front: freq `3.6`,
  gate `0.55, 0.95`, amount `0.42`; both pool via `floor`. Advection `mdx/mdy/fdx/fdy` are
  `k·sin(θ)`/`k·cos(θ)` so the drift loops.
- **rim** — darken `0.55`, warm-glow `0.9`. Too high → cutout halo; too low → the subject
  floats detached from the light.
- **backdrop** — base `0.020`, glow `0.24` and radius `0.72`, blue bias `1.28`, floor
  darkening `0.06`.
- **turn** (a bake-time knob, if the subject is baked with a pseudo-3D turn) — yaw amplitude
  and keystone insets; see `tools.md` §Baking.

## When NOT to reach for this

If the subject is itself procedural (a torus, an SDF, a field), you already own its pixels
and normals — layer with `examples/torus`'s inline wash instead; you don't need a baked
alpha sheet. This kit is specifically for a **photographic / baked subject** that has no
geometry to light, where the atmosphere is what turns a cut-out into a scene.
