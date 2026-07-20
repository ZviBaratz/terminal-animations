# Fonts

Two faces, both SIL OFL 1.1, both self-hosted — the site has no CDN and no build
step, so the woff2 files are committed rather than fetched.

| role | family | version | subset |
|---|---|---|---|
| display | Departure Mono | 1.500 | 4,140 B |
| body / UI | JetBrains Mono Regular | 2.304 | 16,348 B |
| body / UI | JetBrains Mono Bold | 2.304 | 16,632 B |

**37 KB total.** Verified coverage in the subsetted files, not the upstream ones:
95/95 ASCII, 128/128 box drawing (U+2500–257F), 32/32 block elements
(U+2580–259F) in all three.

## Why these two

Departure Mono is a pixel face on an **11px grid** — `unitsPerEm` is 550 and its
design pixel is 50 units, so it is only crisp at font sizes that are multiples of
11. That is why the display scale is 11/22/33/44 and nothing between; a wordmark
at 16px or 24px renders blurry. Do not add an intermediate display size.

JetBrains Mono carries the body and the dev panel because it is
ttfautohint-processed, which is what makes it legible at the 12–14px the panel
runs at. Its hinting is deliberately **kept** through subsetting for that reason;
Departure's is dropped, since hinting is meaningless on a pixel grid.

Neither face has braille (U+2800–28FF) — checked, not assumed. That is why the
gallery's resolution-ladder samples are drawn by the painter via `glyphs.js`
rather than typeset: no font can set the sextant, octant, or braille rungs.

## Licensing

Both are OFL 1.1 and **neither declares a Reserved Font Name** — the phrase
appears only in the standard OFL definitions section, not in either copyright
line. So these subsets keep their original family names, which a reserved name
would have forbidden. (IBM Plex Mono was rejected for exactly that: it reserves
"Plex", and subsetting is modification.)

The OFL requires the licence text to travel with the font files; that is what
`LICENSE-DepartureMono.txt` and `LICENSE-JetBrainsMono.txt` are for. It requires
nothing in the UI and imposes nothing on this repo's MIT-licensed code. The
subsets are themselves OFL.

- Departure Mono — Copyright 2022–2024 Helena Zhang.
  <https://github.com/rektdeckard/departure-mono/releases/tag/v1.500>
- JetBrains Mono — Copyright 2020 The JetBrains Mono Project Authors.
  <https://github.com/JetBrains/JetBrainsMono/releases/tag/v2.304>

## Regenerating

Needs `fonttools` and `brotli`; neither is a runtime dependency.

```sh
uv venv .v && uv pip install -p .v fonttools brotli

.v/bin/pyftsubset DepartureMono-Regular.otf \
  --unicodes="U+0020-007E,U+00B7,U+00D7,U+0394,U+2014,U+2022,U+2192,U+2500-257F,U+2580-259F" \
  --layout-features='' --no-hinting --desubroutinize \
  --flavor=woff2 --output-file=departure-mono-subset.woff2

.v/bin/pyftsubset JetBrainsMono-Regular.ttf \
  --unicodes="U+0020-007E,U+00A0-00FF,U+0394,U+2013-2014,U+2018-201D,U+2022,U+2026,U+2192,U+26A0,U+2500-257F,U+2580-259F" \
  --layout-features='' --flavor=woff2 \
  --output-file=jetbrains-mono-regular-subset.woff2   # and again for Bold
```
