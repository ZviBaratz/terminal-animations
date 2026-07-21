#!/usr/bin/env python3
"""clean.py SRC OUT — matte the marble bust off its watermarked field and write it as a
compact luminance + alpha asset (grayscale L, feathered alpha, everything else transparent).

Pure Pillow (no numpy/OpenCV/scipy). The subject is bright white marble on a bright white
field carrying a faint tiled "pngtree" watermark — a white-on-white matte, the hard case.

The animation (bust.go) is a Warhol pop-art grid: it treats the bust as a *silkscreen
source*, posterizing this luminance into a few flat tonal bands and recoloring each band per
panel. So this script's whole job is to deliver a clean, well-separated luminance ramp of the
subject plus its silhouette — the color is done entirely at runtime.

Pipeline:

  1) Matte. Flood-fill only the near-pure-white *field* inward from the borders (BR = 250) and
     take the subject as everything the flood did not reach. The threshold is high on purpose:
     the shoulders exit the bottom frame edge as bright marble (~236); a lower BR floods up
     into them and amputates the body. Keep *every* connected component above a size floor, not
     just the largest — a bright highlight can split head from torso — which also drops the
     isolated watermark letters (small islands in the field).
  2) Feather the alpha (erode 1px to shave the bright anti-alias rim, then soften).
  3) Luminance. Convert the cut-out to L, downscale to a compact fixed height, then
     median + light blur to erase the faint watermark *over the marble* (downscaling already
     shrinks the tiled text toward nothing; median kills the residual speckle). Downscaling
     before de-speckling and before the contrast stretch matters: it keeps a stray watermark
     stroke from being amplified across a posterization band boundary at run time.
  4) Contrast-stretch the luminance across the subject's own tonal range (p2..p98 → 0..255) so
     the marble's low-contrast shading fills the full ramp and the face reads once posterized.

The watermarked source is only read; it is never copied into the repo. Verify the emitted
asset is watermark-free by eye before committing it (posterization is unforgiving).
"""
import sys
from collections import deque
from PIL import Image, ImageChops, ImageFilter

BR = 250          # background is near-pure white (255); marble stays below this even lit (~236)
MIN_FRAC = 0.004  # keep components ≥ 0.4% of the image (drops watermark-letter islands)
MARGIN = 0.04     # transparent border kept around the subject bbox, as a fraction of bbox
TARGET_H = 220    # baked asset height in px (width follows the crop's aspect); tens-of-KB PNG
LO_PCT, HI_PCT = 2.0, 98.0  # luminance percentiles mapped to 0..255 (contrast stretch)


def flood_background(src, W, H):
    """Return a bytearray flag map: 1 where a border-connected near-white field pixel is."""
    bg = bytearray(W * H)
    dq = deque()

    def seed(x, y):
        i = y * W + x
        if not bg[i] and min(src[x, y]) >= BR:
            bg[i] = 1
            dq.append((x, y))

    for x in range(W):
        seed(x, 0); seed(x, H - 1)
    for y in range(H):
        seed(0, y); seed(W - 1, y)
    while dq:
        x, y = dq.popleft()
        for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
            nx, ny = x + dx, y + dy
            if 0 <= nx < W and 0 <= ny < H and not bg[ny * W + nx] and min(src[nx, ny]) >= BR:
                bg[ny * W + nx] = 1
                dq.append((nx, ny))
    return bg


def subject_mask(bg, W, H):
    """Return an L mask (255 = subject) of every non-background component above the size floor."""
    lab = bytearray(W * H)
    mask = Image.new("L", (W, H), 0)
    mp = mask.load()
    floor = int(W * H * MIN_FRAC)
    kept = 0
    for sy in range(H):
        for sx in range(W):
            si = sy * W + sx
            if bg[si] or lab[si]:
                continue
            comp, st = [], [(sx, sy)]
            lab[si] = 1
            while st:
                x, y = st.pop()
                comp.append((x, y))
                for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                    nx, ny = x + dx, y + dy
                    if 0 <= nx < W and 0 <= ny < H and not bg[ny * W + nx] and not lab[ny * W + nx]:
                        lab[ny * W + nx] = 1
                        st.append((nx, ny))
            if len(comp) >= floor:
                kept += len(comp)
                for x, y in comp:
                    mp[x, y] = 255
    return mask, kept


def stretch_lut(lum, alpha):
    """Build a 256-entry LUT mapping the subject's p2..p98 luminance to 0..255."""
    hist = lum.histogram(mask=alpha.point(lambda a: 255 if a > 8 else 0))
    total = sum(hist)
    if total == 0:
        return list(range(256))
    lo_target, hi_target = total * LO_PCT / 100.0, total * HI_PCT / 100.0
    acc, lo, hi = 0, 0, 255
    for v, n in enumerate(hist):
        if acc <= lo_target:
            lo = v
        if acc <= hi_target:
            hi = v
        acc += n
    if hi <= lo:
        hi = lo + 1
    return [0 if v <= lo else 255 if v >= hi else round((v - lo) * 255.0 / (hi - lo))
            for v in range(256)]


def main():
    if len(sys.argv) != 3:
        sys.exit("usage: clean.py SRC OUT")
    src_path, out_path = sys.argv[1], sys.argv[2]
    im = Image.open(src_path).convert("RGB")
    W, H = im.size

    bg = flood_background(im.load(), W, H)
    mask, kept = subject_mask(bg, W, H)
    if kept == 0:
        sys.exit("clean.py: matte kept no subject — check BR/MIN_FRAC against the source")

    # Feather: erode 1px to shave the bright anti-alias rim, then soften to a smooth alpha.
    alpha = mask.filter(ImageFilter.MinFilter(3)).filter(ImageFilter.GaussianBlur(1.0))

    # Crop the color image and alpha to the subject bbox + a small margin.
    bbox = alpha.getbbox()
    x0, y0, x1, y1 = bbox
    mx, my = int((x1 - x0) * MARGIN), int((y1 - y0) * MARGIN)
    x0, y0 = max(0, x0 - mx), max(0, y0 - my)
    x1, y1 = min(W, x1 + mx), min(H, y1 + my)
    lum = im.convert("L").crop((x0, y0, x1, y1))
    alpha = alpha.crop((x0, y0, x1, y1))

    # Downscale to the compact asset size (aspect-preserving), then erase the watermark over the
    # marble (downscale shrinks the tiled text; median + light blur kill the residual speckle).
    cw, ch = lum.size
    tw = max(1, round(TARGET_H * cw / ch))
    lum = lum.resize((tw, TARGET_H), Image.BILINEAR)
    alpha = alpha.resize((tw, TARGET_H), Image.BILINEAR)
    lum = lum.filter(ImageFilter.MedianFilter(3)).filter(ImageFilter.GaussianBlur(0.6))

    # Contrast-stretch the marble's own tonal range so the face reads once posterized.
    lum = lum.point(stretch_lut(lum, alpha))

    # Emit L+A; zero the luminance outside the silhouette so a stray field pixel can't leak ink.
    lum = ImageChops.multiply(lum, alpha.point(lambda a: 255 if a > 8 else 0))
    Image.merge("LA", (lum, alpha)).save(out_path, optimize=True)
    print(f"clean.py: subject {kept / (W * H):.1%}, asset {lum.size}, wrote {out_path}",
          file=sys.stderr)


if __name__ == "__main__":
    main()
