"""
Convert "Veil" in Plus Jakarta Sans Variable (weight 500) to SVG path data.

Coordinate mapping:
  - Font Y axis goes up; SVG Y axis goes down — so Y is negated.
  - Baseline sits at BASELINE_Y in SVG space.
  - Text starts at TEXT_START_X in the 90×40 lockup viewBox.
"""

import sys
from fontTools.ttLib import TTFont
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.varLib.instancer import instantiateVariableFont

FONT_PATH = (
    "web/node_modules/@fontsource-variable/plus-jakarta-sans"
    "/files/plus-jakarta-sans-latin-wght-normal.woff2"
)
TEXT = "Veil"
FONT_SIZE = 24          # SVG coordinate units
BASELINE_Y = 20         # Y of the text baseline in the 40-unit-tall SVG
TEXT_START_X = 41       # X where the wordmark begins (after mark + gap)
LETTER_SPACING_EM = -0.025

font = TTFont(FONT_PATH)

# Instance the variable font at weight 500
font = instantiateVariableFont(font, {"wght": 500})

upm = font["head"].unitsPerEm
scale = FONT_SIZE / upm
letter_spacing_units = LETTER_SPACING_EM * upm

glyph_set = font.getGlyphSet()
cmap = font.getBestCmap()

path_parts = []
x_cursor = 0.0  # accumulated advance in font units

for char in TEXT:
    glyph_name = cmap[ord(char)]
    glyph = glyph_set[glyph_name]

    # Affine transform: (xx, xy, yx, yy, dx, dy)
    #   new_x = xx*x + yx*y + dx
    #   new_y = xy*x + yy*y + dy
    transform = (
        scale,                                # xx — x scale
        0,                                    # xy
        0,                                    # yx
        -scale,                               # yy — negate to flip Y axis
        TEXT_START_X + x_cursor * scale,      # dx — x translation
        BASELINE_Y,                           # dy — y translation (baseline)
    )

    svg_pen = SVGPathPen(glyph_set)
    glyph.draw(TransformPen(svg_pen, transform))

    commands = svg_pen.getCommands()
    if commands:
        path_parts.append(commands)

    x_cursor += glyph.width + letter_spacing_units

combined_d = " ".join(path_parts)

# Print metrics so we can size the viewBox correctly
total_width_svg = TEXT_START_X + x_cursor * scale
print(f"<!-- estimated total SVG width: {total_width_svg:.1f} -->", file=sys.stderr)
print(f"<!-- UPM: {upm}, scale: {scale:.4f} -->", file=sys.stderr)

print(f'<path d="{combined_d}" fill="currentColor"/>')
