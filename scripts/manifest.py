#!/usr/bin/env python3
"""Write web/animations.json — what the gallery and viewer read.

Keyed on the .wasm modules actually present in web/, so the index can never link
to a viewer with no module behind it. Each entry is that animation's meta.json,
which lives beside the animation because it describes the animation: title,
blurb, ladder rung, accent colour, loop shape, and which tick to still for the
poster.

An animation with no meta.json still appears — on rung 1, with no blurb — rather
than vanishing from the index without explanation.

Stdlib only, like scripts/ansi2png.py.

    python3 scripts/manifest.py <repo-root>
"""

import glob
import json
import os
import sys


def build(root):
    out = {}
    for wasm in sorted(glob.glob(os.path.join(root, "web", "*.wasm"))):
        name = os.path.basename(wasm)[:-len(".wasm")]
        meta = os.path.join(root, "examples", name, "meta.json")
        if os.path.exists(meta):
            with open(meta, encoding="utf-8") as f:
                out[name] = json.load(f)
        else:
            out[name] = {"title": name, "rung": 1}
            print(f"  note: {name} has no meta.json — defaulting to rung 1")
    return out


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "."
    animations = build(root)
    path = os.path.join(root, "web", "animations.json")
    with open(path, "w", encoding="utf-8") as f:
        json.dump(animations, f, separators=(",", ":"), ensure_ascii=False)
    print("  built:", ", ".join(animations) or "(nothing)")


if __name__ == "__main__":
    main()
