#!/usr/bin/env python3
"""Draw the block-registry wordmark into doc/img/.

The logo is generated rather than hand-edited so it stays reproducible and can
be re-rendered at any size: run "make logo" after changing anything here.

It is deliberately block's mark with a second word beside it: this repository is
where block's recipes are written, and the two sites should read as one project.
The source of the shape is kept in sync by hand — it is thirty lines and it
changes about never.
"""

import os
import sys

from PIL import Image, ImageDraw, ImageFont

BG = (13, 17, 23)  # GitHub dark, the same ground the documentation site paints
FG = (230, 237, 243)
ACCENT = (88, 166, 255)
MUTED = (145, 152, 161)

HACK_BOLD = "/usr/share/fonts/truetype/hack/Hack-Bold.ttf"
HACK_REGULAR = "/usr/share/fonts/truetype/hack/Hack-Regular.ttf"

SCALE = 4  # draw oversampled, then downsample: PIL has no antialiased shapes


def draw_mark(d, x, y, w, h):
    """A padlock whose body is three stacked blocks."""
    # Shackle, behind and above the body: an arch with two legs running down
    # into it. Narrower than the body, or the whole mark reads as a basket.
    sw = w * 0.13
    arch_w = w * 0.52
    ax = x + (w - arch_w) / 2
    arch_h = h * 0.30
    body_top = y + h * 0.38
    d.arc(
        [ax, y + sw / 2, ax + arch_w, y + sw / 2 + arch_h * 2],
        start=180,
        end=360,
        fill=ACCENT,
        width=int(sw),
    )
    for side in (ax + sw / 2, ax + arch_w - sw / 2):
        d.line([side, y + sw / 2 + arch_h, side, body_top + sw], fill=ACCENT, width=int(sw))

    # Body: one solid block with two slots milled out, so it reads as three
    # bars locked together rather than three loose ones.
    body = [x, body_top, x + w, y + h]
    d.rounded_rectangle(body, radius=w * 0.10, fill=ACCENT)
    bar_h = (y + h - body_top) / 5.0
    for i in (1, 3):
        top = body_top + bar_h * i
        d.rounded_rectangle(
            [x + w * 0.14, top, x + w * 0.86, top + bar_h],
            radius=bar_h * 0.42,
            fill=BG,
        )


def render_mark(size, out):
    """The mark on its own, square — what a 64x64 favicon can actually show."""
    S = size * SCALE
    im = Image.new("RGB", (S, S), BG)
    d = ImageDraw.Draw(im)
    h = int(S * 0.74)
    w = int(h * 0.80)
    draw_mark(d, (S - w) // 2, (S - h) // 2, w, h)
    im.resize((size, size), Image.LANCZOS).save(out, optimize=True)
    print(f"wrote {out} ({size}x{size})")


def render(width, height, out):
    W, H = width * SCALE, height * SCALE
    im = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(im)

    mark_h = int(H * 0.62)
    mark_w = int(mark_h * 0.80)
    gap = int(H * 0.10)

    # The wordmark is long enough to run off a 3:1 canvas at a size picked from
    # the height, so pick it from the width that is actually left over instead.
    size = int(H * 0.44)
    while size > 8:
        bold = ImageFont.truetype(HACK_BOLD, size)
        light = ImageFont.truetype(HACK_REGULAR, size)
        b = d.textbbox((0, 0), "block", font=bold)
        r = d.textbbox((0, 0), "-registry", font=light)
        total = mark_w + gap + (b[2] - b[0]) + (r[2] - r[0])
        if total <= W * 0.88:
            break
        size -= 2

    mx = (W - total) // 2
    my = (H - mark_h) // 2
    draw_mark(d, mx, my, mark_w, mark_h)
    ty = (H - (b[3] + b[1])) // 2
    tx = mx + mark_w + gap - b[0]
    d.text((tx, ty), "block", font=bold, fill=FG)
    d.text((tx + (b[2] - b[0]), ty), "-registry", font=light, fill=MUTED)

    im.resize((width, height), Image.LANCZOS).save(out, optimize=True)
    print(f"wrote {out} ({width}x{height})")


def main():
    root = os.path.join(os.path.dirname(os.path.abspath(__file__)), os.pardir)
    img = os.path.join(root, "doc", "img")
    os.makedirs(img, exist_ok=True)
    render(1800, 600, os.path.join(img, "block-registry-logo.png"))
    render_mark(512, os.path.join(img, "block-registry-mark.png"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
