#!/usr/bin/env python3
"""clean.py SRC OUT — matte the marble bust off its watermarked field and write it as a
clean RGBA cut-out (subject pixels + a feathered alpha, everything else transparent).

Pure Pillow (no numpy/OpenCV/scipy). The subject is bright white marble on a bright white
field carrying a faint tiled "pngtree" watermark — a white-on-white matte, the hard case.

The separation exploits that the *field* is near-pure white (255) and flat, while the
marble, however bright, is textured and stays below ~245 even at its highlights. So we
flood-fill only the near-pure-white background inward from the borders (BR = 250) and take
the subject as everything the flood did not reach. Two properties matter:

  - The threshold is high on purpose. The bust's shoulders exit the *bottom* frame edge as
    bright marble (~236); the old BR = 210 flooded straight up into them from the border and
    amputated the body. At BR = 250 that marble is not background, so the whole bust — head,
    neck, and shoulders — survives. (Verified by rendering the silhouette.)
  - We keep *every* connected component above a size floor, not just the largest. A bright
    highlight can split the subject into head + torso blobs; keep-largest would drop one.
    The size floor drops the isolated watermark letters (small islands in the white field).

The result is emitted with alpha so the animation can composite it over a *runtime* backdrop
and *runtime* mist/light — the atmosphere is synthesized live in bust.go, not baked in here.
The watermarked source is only read; it is never copied into the repo.
"""
import sys
from collections import deque
from PIL import Image, ImageChops, ImageFilter

BR = 250        # background is near-pure white (255); marble stays below this even lit (~236)
MIN_FRAC = 0.004  # keep components ≥ 0.4% of the image (drops watermark-letter islands)
MARGIN = 0.06     # transparent border kept around the subject bbox, as a fraction of bbox


def main():
    if len(sys.argv) != 3:
        sys.exit("usage: clean.py SRC OUT")
    src_path, out_path = sys.argv[1], sys.argv[2]
    im = Image.open(src_path).convert("RGB")
    W, H = im.size
    src = im.load()

    # 1) flood-fill the near-pure-white background inward from every border pixel.
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
            if 0 <= nx < W and 0 <= ny < H:
                i = ny * W + nx
                if not bg[i] and min(src[nx, ny]) >= BR:
                    bg[i] = 1
                    dq.append((nx, ny))

    # 2) subject = every 4-connected non-background component above the size floor.
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
                    if 0 <= nx < W and 0 <= ny < H:
                        j = ny * W + nx
                        if not bg[j] and not lab[j]:
                            lab[j] = 1
                            st.append((nx, ny))
            if len(comp) >= floor:
                kept += len(comp)
                for x, y in comp:
                    mp[x, y] = 255

    if kept == 0:
        sys.exit("clean.py: matte kept no subject — check BR/MIN_FRAC against the source")

    # 3) feather: erode 1px to shave the bright anti-alias rim, then soften to a smooth alpha.
    mask = mask.filter(ImageFilter.MinFilter(3))
    alpha = mask.filter(ImageFilter.GaussianBlur(1.0))

    # 3b) dissolve the torso base: the shoulders are cut flat by the source frame, which would
    #     read as a hard horizontal edge. Ramp alpha to 0 over the bottom of the subject bbox
    #     so the base fades out — the animation's mist then pools where the bust dissolves.
    bbox = alpha.getbbox()
    if bbox:
        _, _, _, y1 = bbox
        y0 = bbox[1]
        fade = int((y1 - y0) * 0.16)
        if fade > 0:
            ramp = Image.new("L", (W, H), 255)
            rp = ramp.load()
            for yy in range(max(0, y1 - fade), H):
                t = min(1.0, (yy - (y1 - fade)) / fade)
                v = int(255 * (1 - t))
                for xx in range(W):
                    rp[xx, yy] = v
            alpha = ImageChops.multiply(alpha, ramp)

    # 4) emit an RGBA cut-out cropped to the subject bbox plus a small transparent margin,
    #    so the animation has room to turn the bust and show backdrop around it.
    rgba = im.convert("RGBA")
    rgba.putalpha(alpha)
    bbox = alpha.getbbox()
    if bbox:
        x0, y0, x1, y1 = bbox
        mx = int((x1 - x0) * MARGIN)
        my = int((y1 - y0) * MARGIN)
        x0, y0 = max(0, x0 - mx), max(0, y0 - my)
        x1, y1 = min(W, x1 + mx), min(H, y1 + my)
        rgba = rgba.crop((x0, y0, x1, y1))
    rgba.save(out_path)
    print(f"clean.py: subject {kept / (W * H):.1%}, cut-out {rgba.size}, wrote {out_path}",
          file=sys.stderr)


if __name__ == "__main__":
    main()
