#!/usr/bin/env python3
"""Build a professional PPTX from a JSON spec produced by the LLM.

Spec shape (all fields required unless marked optional):
{
  "title": str,
  "language": "en" | "uz" | "ru" | ...,
  "palette": {"primary": "#hex", "accent": "#hex", "bg": "#hex", "text": "#hex", "muted": "#hex"},
  "fonts":   {"heading": "Inter", "body": "Inter"},
  "slides": [
    {"layout": "title",      "title": str, "subtitle": str},
    {"layout": "section",    "title": str, "subtitle": str (optional)},
    {"layout": "bullets",    "title": str, "bullets": [str, ...], "notes": str (optional)},
    {"layout": "two_column", "title": str, "left_title": str, "left_bullets": [...], "right_title": str, "right_bullets": [...]},
    {"layout": "quote",      "quote": str, "author": str},
    {"layout": "stat",       "title": str, "stat_value": str, "stat_label": str, "support": str (optional)},
    {"layout": "thanks",     "title": str, "subtitle": str}
  ]
}

Usage:
  python3 build_pptx.py <spec.json> <output.pptx>
"""
import json
import sys
from pptx import Presentation
from pptx.util import Inches, Pt, Emu
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import PP_ALIGN, MSO_ANCHOR
from pptx.oxml.ns import qn
from lxml import etree

SLIDE_W = Inches(13.333)  # 16:9 widescreen
SLIDE_H = Inches(7.5)


def hex_to_rgb(h):
    h = h.lstrip("#")
    return RGBColor(int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16))


def darken(h, amount=0.15):
    h = h.lstrip("#")
    r, g, b = int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)
    r = max(0, int(r * (1 - amount)))
    g = max(0, int(g * (1 - amount)))
    b = max(0, int(b * (1 - amount)))
    return f"#{r:02x}{g:02x}{b:02x}"


def lighten(h, amount=0.15):
    h = h.lstrip("#")
    r, g, b = int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)
    r = min(255, int(r + (255 - r) * amount))
    g = min(255, int(g + (255 - g) * amount))
    b = min(255, int(b + (255 - b) * amount))
    return f"#{r:02x}{g:02x}{b:02x}"


def add_solid_bg(slide, color_hex):
    bg = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, SLIDE_W, SLIDE_H)
    bg.line.fill.background()
    bg.fill.solid()
    bg.fill.fore_color.rgb = hex_to_rgb(color_hex)
    bg.shadow.inherit = False
    # send to back
    spTree = bg._element.getparent()
    spTree.remove(bg._element)
    spTree.insert(2, bg._element)
    return bg


def add_textbox(slide, x, y, w, h, text, font_name, size_pt, color_hex, bold=False, align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP):
    tb = slide.shapes.add_textbox(x, y, w, h)
    tf = tb.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    tf.margin_left = Emu(0)
    tf.margin_right = Emu(0)
    tf.margin_top = Emu(0)
    tf.margin_bottom = Emu(0)
    p = tf.paragraphs[0]
    p.alignment = align
    run = p.add_run()
    run.text = text
    run.font.name = font_name
    run.font.size = Pt(size_pt)
    run.font.bold = bold
    run.font.color.rgb = hex_to_rgb(color_hex)
    return tb


def add_accent_bar(slide, palette, x, y, w, h):
    bar = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, x, y, w, h)
    bar.line.fill.background()
    bar.fill.solid()
    bar.fill.fore_color.rgb = hex_to_rgb(palette["accent"])
    bar.shadow.inherit = False
    return bar


def render_title_slide(slide, spec, palette, fonts):
    add_solid_bg(slide, palette["bg"])
    # Big accent bar on left
    bar = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, Inches(0.45), SLIDE_H)
    bar.line.fill.background()
    bar.fill.solid()
    bar.fill.fore_color.rgb = hex_to_rgb(palette["accent"])
    bar.shadow.inherit = False
    # Title — large, bold, primary color
    add_textbox(slide, Inches(1.2), Inches(2.4), Inches(11), Inches(2.5),
                spec["title"], fonts["heading"], 54, palette["primary"], bold=True)
    # Subtitle — medium, muted
    if spec.get("subtitle"):
        add_textbox(slide, Inches(1.2), Inches(4.5), Inches(11), Inches(1.5),
                    spec["subtitle"], fonts["body"], 22, palette.get("muted", darken(palette["text"], 0.3)))


def render_section_slide(slide, spec, palette, fonts):
    # Section breaks use primary as the BG, white text — strong visual reset
    add_solid_bg(slide, palette["primary"])
    add_textbox(slide, Inches(1), Inches(2.8), Inches(11.3), Inches(1.8),
                spec["title"], fonts["heading"], 60, "#ffffff", bold=True,
                align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.MIDDLE)
    if spec.get("subtitle"):
        add_textbox(slide, Inches(1), Inches(4.7), Inches(11.3), Inches(1),
                    spec["subtitle"], fonts["body"], 22, lighten(palette["primary"], 0.7))
    # Accent strip at bottom
    add_accent_bar(slide, palette, Inches(1), Inches(6.5), Inches(2), Inches(0.08))


def render_bullets_slide(slide, spec, palette, fonts):
    add_solid_bg(slide, palette["bg"])
    # Title
    add_textbox(slide, Inches(0.75), Inches(0.55), Inches(12), Inches(0.9),
                spec["title"], fonts["heading"], 32, palette["primary"], bold=True)
    # Title underline accent
    add_accent_bar(slide, palette, Inches(0.75), Inches(1.45), Inches(0.7), Inches(0.06))
    # Bullets
    tb = slide.shapes.add_textbox(Inches(0.75), Inches(1.9), Inches(12), Inches(5.0))
    tf = tb.text_frame
    tf.word_wrap = True
    tf.margin_left = Emu(0)
    bullets = spec.get("bullets", [])
    for i, b in enumerate(bullets):
        p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
        p.space_after = Pt(12)
        # Bullet marker as actual unicode • for cross-platform consistency
        run_dot = p.add_run()
        run_dot.text = "•  "
        run_dot.font.name = fonts["body"]
        run_dot.font.size = Pt(22)
        run_dot.font.bold = True
        run_dot.font.color.rgb = hex_to_rgb(palette["accent"])
        run = p.add_run()
        run.text = b
        run.font.name = fonts["body"]
        run.font.size = Pt(22)
        run.font.color.rgb = hex_to_rgb(palette["text"])
    if spec.get("notes"):
        slide.notes_slide.notes_text_frame.text = spec["notes"]


def render_two_column_slide(slide, spec, palette, fonts):
    add_solid_bg(slide, palette["bg"])
    add_textbox(slide, Inches(0.75), Inches(0.55), Inches(12), Inches(0.9),
                spec["title"], fonts["heading"], 30, palette["primary"], bold=True)
    add_accent_bar(slide, palette, Inches(0.75), Inches(1.45), Inches(0.7), Inches(0.06))
    # Left column box
    lcard = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, Inches(0.75), Inches(2.0), Inches(5.95), Inches(4.8))
    lcard.line.color.rgb = hex_to_rgb(lighten(palette["muted"] if palette.get("muted") else palette["primary"], 0.6))
    lcard.line.width = Pt(0.75)
    lcard.fill.solid()
    lcard.fill.fore_color.rgb = hex_to_rgb(lighten(palette["bg"], 0.0)) if palette["bg"].lower() != "#ffffff" else hex_to_rgb("#fafafa")
    lcard.shadow.inherit = False
    # Right column box
    rcard = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, Inches(6.85), Inches(2.0), Inches(5.95), Inches(4.8))
    rcard.line.color.rgb = lcard.line.color.rgb
    rcard.line.width = Pt(0.75)
    rcard.fill.solid()
    rcard.fill.fore_color.rgb = lcard.fill.fore_color.rgb
    rcard.shadow.inherit = False

    def column(x, ctitle, items):
        add_textbox(slide, x + Inches(0.35), Inches(2.3), Inches(5.3), Inches(0.6),
                    ctitle, fonts["heading"], 20, palette["accent"], bold=True)
        tb = slide.shapes.add_textbox(x + Inches(0.35), Inches(3.0), Inches(5.3), Inches(3.5))
        tf = tb.text_frame
        tf.word_wrap = True
        tf.margin_left = Emu(0)
        for i, item in enumerate(items):
            p = tf.paragraphs[0] if i == 0 else tf.add_paragraph()
            p.space_after = Pt(8)
            rd = p.add_run(); rd.text = "•  "
            rd.font.name = fonts["body"]; rd.font.size = Pt(16); rd.font.bold = True
            rd.font.color.rgb = hex_to_rgb(palette["accent"])
            r = p.add_run(); r.text = item
            r.font.name = fonts["body"]; r.font.size = Pt(16)
            r.font.color.rgb = hex_to_rgb(palette["text"])

    column(Inches(0.75), spec["left_title"], spec.get("left_bullets", []))
    column(Inches(6.85), spec["right_title"], spec.get("right_bullets", []))


def render_quote_slide(slide, spec, palette, fonts):
    add_solid_bg(slide, palette["bg"])
    # Big quote mark
    qm = slide.shapes.add_textbox(Inches(0.8), Inches(0.8), Inches(2), Inches(2))
    qmp = qm.text_frame.paragraphs[0]
    qmr = qmp.add_run(); qmr.text = "“"
    qmr.font.name = fonts["heading"]; qmr.font.size = Pt(140); qmr.font.bold = True
    qmr.font.color.rgb = hex_to_rgb(palette["accent"])
    # Quote text
    add_textbox(slide, Inches(1.5), Inches(2.5), Inches(10.3), Inches(3),
                spec["quote"], fonts["heading"], 30, palette["text"], bold=False,
                align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.TOP)
    # Author
    add_textbox(slide, Inches(1.5), Inches(5.7), Inches(10.3), Inches(0.6),
                "— " + spec["author"], fonts["body"], 18, palette.get("muted", darken(palette["text"], 0.3)))


def render_stat_slide(slide, spec, palette, fonts):
    add_solid_bg(slide, palette["bg"])
    add_textbox(slide, Inches(0.75), Inches(0.55), Inches(12), Inches(0.9),
                spec["title"], fonts["heading"], 28, palette["primary"], bold=True)
    add_accent_bar(slide, palette, Inches(0.75), Inches(1.45), Inches(0.7), Inches(0.06))
    # Huge stat
    add_textbox(slide, Inches(0.75), Inches(2.6), Inches(12), Inches(2.2),
                spec["stat_value"], fonts["heading"], 110, palette["accent"], bold=True,
                align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
    # Stat label
    add_textbox(slide, Inches(0.75), Inches(4.8), Inches(12), Inches(0.7),
                spec["stat_label"], fonts["body"], 22, palette["text"],
                align=PP_ALIGN.CENTER)
    if spec.get("support"):
        add_textbox(slide, Inches(2), Inches(5.7), Inches(9.3), Inches(1.0),
                    spec["support"], fonts["body"], 16,
                    palette.get("muted", darken(palette["text"], 0.3)),
                    align=PP_ALIGN.CENTER)


def render_thanks_slide(slide, spec, palette, fonts):
    add_solid_bg(slide, palette["primary"])
    add_textbox(slide, Inches(1), Inches(2.8), Inches(11.3), Inches(1.8),
                spec["title"], fonts["heading"], 72, "#ffffff", bold=True,
                align=PP_ALIGN.LEFT, anchor=MSO_ANCHOR.MIDDLE)
    if spec.get("subtitle"):
        add_textbox(slide, Inches(1), Inches(4.7), Inches(11.3), Inches(1),
                    spec["subtitle"], fonts["body"], 20, lighten(palette["primary"], 0.7))
    add_accent_bar(slide, palette, Inches(1), Inches(6.5), Inches(2.5), Inches(0.1))


LAYOUTS = {
    "title":      render_title_slide,
    "section":    render_section_slide,
    "bullets":    render_bullets_slide,
    "two_column": render_two_column_slide,
    "quote":      render_quote_slide,
    "stat":       render_stat_slide,
    "thanks":     render_thanks_slide,
}


def main():
    spec_path = sys.argv[1]
    out_path  = sys.argv[2]
    with open(spec_path) as f:
        spec = json.load(f)

    prs = Presentation()
    prs.slide_width  = SLIDE_W
    prs.slide_height = SLIDE_H

    palette = spec["palette"]
    fonts   = spec["fonts"]

    for slide_spec in spec["slides"]:
        layout = slide_spec.get("layout", "bullets")
        blank = prs.slide_layouts[6]  # blank
        slide = prs.slides.add_slide(blank)
        renderer = LAYOUTS.get(layout, render_bullets_slide)
        try:
            renderer(slide, slide_spec, palette, fonts)
        except Exception as e:
            # Fallback to a plain title-only slide so a malformed AI spec doesn't kill the whole deck
            render_section_slide(slide, {"title": slide_spec.get("title", "Slide"), "subtitle": str(e)[:80]}, palette, fonts)

    prs.save(out_path)
    print(f"OK {out_path}")


if __name__ == "__main__":
    main()
