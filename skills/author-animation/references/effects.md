# The effects catalog — springboards, not recipes

The canon of terminal / demoscene motion. Use these to *combine*, not to copy: the
mesmerizing pieces layer two or three of these (a plasma wash under a starfield under a
focal vignette), reinterpret one into a new palette or resolution tier, or drive one
with another. Each entry is the core algorithm in a sentence or two — enough to
reimplement — plus what makes it *read*.

## Fields (pure `f(pos, tick)` — loop forever, no state)

- **Plasma** — sum of sines / distance fields: `v = sin(x/a) + sin(y/b) + sin(dist(x,y,cx,cy)/d)`,
  add `tick` to the phases; map `v` → a cyclic palette. Smooth organic colour flow.
  *Reads via* luminance, not glyph density — the classic gradient case.
- **Tunnel** — precompute polar coords per cell: `angle=atan2(y,x)`, `depth=k/radius`;
  index a texture by `(angle+tick, depth+tick)`; fog by radius. The `1/radius` mapping
  makes it recede.
- **Metaballs** — `f(p)=Σ rᵢ²/|p−cᵢ|²` over moving centres; draw the isosurface
  `f>threshold`, shade by field magnitude. Blobs merge and split — organic, liquid.
- **Rotozoom** — sample a tiled texture through an inverse rotate+scale per cell;
  animate angle and zoom. A spinning, zooming plane.
- **Fire (Doom/PSX)** — bottom row = hot random seed; each cell upward = the
  cell-below minus a small random decay (+ optional drift), clamped ≥0; value → a
  white→yellow→red→black palette. Cheap, iterative, unreasonably convincing.

## Stateful / 3D / resolving (carry a grid, particles, or a z-buffer)

- **Starfield warp** — stars with depth `z`; each frame `z -= speed`, project
  `sx=x/z, sy=y/z`; nearer = faster + brighter (parallax). Respawn past the plane.
  *The warp read comes from streaks* — draw the near/fast stars as a line from previous
  to current projected position, not a dot.
- **Raymarched SDF** — per cell, march a ray by the signed-distance value (sphere/box
  SDFs combined with smooth-min); on hit, normal from the SDF gradient, shade `N·L` →
  luminance ramp. Real 3D scenes in text.
- **Spinning donut** (Sloane `donut.c`) — a torus swept by `(θ,φ)`; rotate each point by
  animation angles, perspective-project `x'=K1·x/(K2+z)`, keep a **per-cell z-buffer**;
  luminance = surface-normal · light → `".,-~:;=!*#$@"`. The z-buffer + N·L is what makes
  it read as a lit solid.
- **Conway's Life** — next cell alive iff (neighbours==3) or (alive && neighbours==2).
  Emergent gliders/oscillators — great ambient texture. Its goldens are crisp (see the
  skill's Test section).
- **Digital rain (Matrix)** — per column: a bright falling head + a fading tail (a
  brightness ramp down the trail), random glyph churn, respawn at top. (This *is*
  fresco's `rain` — reach for the provider before rebuilding it.)
- **Boids / flocking** — each agent steers by three local rules within a radius:
  separation, alignment, cohesion. Emergent flocks from local rules only.

## Combining — where the magic is

The RED baseline for "build a starfield" is a competent single effect in flat ASCII.
The step up:
- **Layer** a slow field (plasma / nebula wash on colour luminance) *behind* a sprite or
  particle system, with a focal **vignette** (`craft.md`) tying them together.
- **Reinterpret the tier** — the same starfield in **half-blocks** or **braille**
  (`techniques.md`) is a different, far higher-fidelity object than the `.·+*#@` version.
- **Drive one with another** — let a plasma field modulate a rain's colour, or fire's
  heat displace a tunnel.
- **Design the palette** (`craft.md` two channels; `techniques.md` depth/dither) instead
  of picking "grey → white → cyan" by reflex.

Showcase canon to study: **Bad Apple!!** (silhouette benchmark), **16colo.rs** (the ANSI
art scene archive), `donut.c` derivatives, asciinema recordings.
