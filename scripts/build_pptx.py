#!/usr/bin/env python3
"""Build a professional PPTX from a JSON spec produced by the LLM.

v2 (2026-06 redesign): bigger typography, modern asymmetric layouts,
slide numbers, footer brand mark, decorative shapes, and a wider
layout vocabulary so the deck doesn't read as "all bullets".

Spec shape:
{
  "title": str,                     // deck title (shown on the title slide)
  "subtitle": str,                  // optional subtitle on the title slide
  "language": "en|uz|ru|es|ar",
  "footer": str,                    // brand mark shown at the bottom of every non-title slide (e.g. "Acme Corp · 2026")
  "palette": {
    "primary": "#hex",              // strongest brand color (headings, section bg)
    "accent":  "#hex",              // contrast color (bullets, numbers, bars)
    "bg":      "#hex",              // slide bg (usually #ffffff or near-white)
    "text":    "#hex",              // body text (high-contrast on bg)
    "muted":   "#hex"               // sub-text / footer / page number
  },
  "fonts": {"heading": "Inter|Roboto|Lato|Source Sans Pro|Poppins", "body": "Inter|Roboto|Lato"},
  "slides": [
    {"layout": "title",      "title": "...", "subtitle": "..." },
    {"layout": "agenda",     "title": "Agenda", "items": ["Section 1", "Section 2", ...]},
    {"layout": "section",    "title": "01 — Section name", "subtitle": "one-liner", "section_number": "01"},
    {"layout": "big_text",   "title": "...", "body": "one or two short sentences"},
    {"layout": "bullets",    "title": "...", "bullets": ["...", "..."], "notes": "..."},
    {"layout": "two_column", "title": "...", "left_title": "...", "left_bullets": [...], "right_title": "...", "right_bullets": [...]},
    {"layout": "comparison", "title": "...", "a_label": "Before", "a_body": "...", "b_label": "After", "b_body": "..."},
    {"layout": "timeline",   "title": "...", "items": [{"label": "Q1", "text": "..."}, ...]},   // 3-5 items
    {"layout": "stat",       "title": "...", "stat_value": "$4.2M", "stat_label": "Q4 revenue", "support": "optional"},
    {"layout": "quote",      "quote": "...", "author": "..."},
    {"layout": "thanks",     "title": "Thank you.", "subtitle": "optional"}
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

SLIDE_W = Inches(13.333)  # 16:9 widescreen
SLIDE_H = Inches(7.5)


# ─── color helpers ──────────────────────────────────────────────────────

def hex_to_rgb(h):
    h = h.lstrip("#")
    return RGBColor(int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16))


def mix(h, target, amount):
    """Mix `h` toward `target` by `amount` (0..1)."""
    h = h.lstrip("#")
    tg = target.lstrip("#")
    r1, g1, b1 = int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16)
    r2, g2, b2 = int(tg[0:2], 16), int(tg[2:4], 16), int(tg[4:6], 16)
    r = int(r1 + (r2 - r1) * amount)
    g = int(g1 + (g2 - g1) * amount)
    b = int(b1 + (b2 - b1) * amount)
    return f"#{r:02x}{g:02x}{b:02x}"


def lighten(h, amount=0.5):
    return mix(h, "#ffffff", amount)


def darken(h, amount=0.3):
    return mix(h, "#000000", amount)


# ─── primitives ─────────────────────────────────────────────────────────

def add_solid_bg(slide, color_hex):
    bg = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, 0, 0, SLIDE_W, SLIDE_H)
    bg.line.fill.background()
    bg.fill.solid()
    bg.fill.fore_color.rgb = hex_to_rgb(color_hex)
    bg.shadow.inherit = False
    spTree = bg._element.getparent()
    spTree.remove(bg._element)
    spTree.insert(2, bg._element)
    return bg


def add_textbox(slide, x, y, w, h, text, font_name, size_pt, color_hex,
                bold=False, italic=False, align=PP_ALIGN.LEFT,
                anchor=MSO_ANCHOR.TOP, line_spacing=None):
    tb = slide.shapes.add_textbox(x, y, w, h)
    tf = tb.text_frame
    tf.word_wrap = True
    tf.vertical_anchor = anchor
    tf.margin_left = Emu(0); tf.margin_right = Emu(0)
    tf.margin_top = Emu(0);  tf.margin_bottom = Emu(0)
    p = tf.paragraphs[0]
    p.alignment = align
    if line_spacing is not None:
        p.line_spacing = line_spacing
    run = p.add_run()
    run.text = text
    run.font.name = font_name
    run.font.size = Pt(size_pt)
    run.font.bold = bold
    run.font.italic = italic
    run.font.color.rgb = hex_to_rgb(color_hex)
    return tb


def add_rect(slide, x, y, w, h, fill_hex, line_hex=None):
    s = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, x, y, w, h)
    s.fill.solid()
    s.fill.fore_color.rgb = hex_to_rgb(fill_hex)
    if line_hex is None:
        s.line.fill.background()
    else:
        s.line.color.rgb = hex_to_rgb(line_hex)
    s.shadow.inherit = False
    return s


def add_rounded(slide, x, y, w, h, fill_hex, line_hex=None):
    s = slide.shapes.add_shape(MSO_SHAPE.ROUNDED_RECTANGLE, x, y, w, h)
    s.fill.solid()
    s.fill.fore_color.rgb = hex_to_rgb(fill_hex)
    if line_hex is None:
        s.line.fill.background()
    else:
        s.line.color.rgb = hex_to_rgb(line_hex)
        s.line.width = Pt(0.75)
    s.shadow.inherit = False
    return s


def add_circle(slide, x, y, d, fill_hex):
    s = slide.shapes.add_shape(MSO_SHAPE.OVAL, x, y, d, d)
    s.fill.solid()
    s.fill.fore_color.rgb = hex_to_rgb(fill_hex)
    s.line.fill.background()
    s.shadow.inherit = False
    return s


# ─── slide chrome (footer + page number) ────────────────────────────────

def add_chrome(slide, page_num, total, palette, fonts, footer_text):
    """Bottom footer with brand on the left + page n/N on the right.
    Skipped on title and thanks slides (caller decides)."""
    muted = palette.get("muted", "#888888")
    # Thin separator line
    sep = add_rect(slide, Inches(0.6), Inches(7.05), Inches(12.1), Emu(9525), muted)
    sep.fill.fore_color.rgb = hex_to_rgb(lighten(muted, 0.6))
    if footer_text:
        add_textbox(slide, Inches(0.6), Inches(7.15), Inches(8), Inches(0.3),
                    footer_text, fonts["body"], 10, muted)
    add_textbox(slide, Inches(11.2), Inches(7.15), Inches(1.6), Inches(0.3),
                f"{page_num} / {total}", fonts["body"], 10, muted,
                align=PP_ALIGN.RIGHT)


# ─── individual layout renderers ────────────────────────────────────────

def render_title_slide(slide, spec, palette, fonts, _ctx):
    add_solid_bg(slide, palette["bg"])
    # Large accent block on left
    add_rect(slide, 0, 0, Inches(0.65), SLIDE_H, palette["accent"])
    # Decorative thin lines (subtle, top-right corner)
    add_rect(slide, Inches(11.5), Inches(0.6), Inches(1.2), Emu(15875), palette["accent"])
    add_rect(slide, Inches(11.5), Inches(0.85), Inches(0.6), Emu(15875), palette["primary"])
    # Eyebrow
    eyebrow = spec.get("eyebrow") or "PRESENTATION"
    add_textbox(slide, Inches(1.4), Inches(2.0), Inches(11), Inches(0.4),
                eyebrow.upper(), fonts["body"], 13, palette["accent"], bold=True)
    # Title — huge
    add_textbox(slide, Inches(1.4), Inches(2.6), Inches(11), Inches(2.6),
                spec["title"], fonts["heading"], 60, palette["primary"], bold=True,
                line_spacing=1.05)
    # Subtitle
    if spec.get("subtitle"):
        add_textbox(slide, Inches(1.4), Inches(5.2), Inches(11), Inches(1.2),
                    spec["subtitle"], fonts["body"], 20,
                    palette.get("muted", darken(palette["text"], 0.3)),
                    line_spacing=1.3)


def render_section_slide(slide, spec, palette, fonts, _ctx):
    # Full-bleed primary with a contrasting accent block
    add_solid_bg(slide, palette["primary"])
    # Big accent shape (asymmetric)
    add_rect(slide, Inches(0), Inches(0), Inches(0.5), SLIDE_H, palette["accent"])
    # Section number (huge, transparent style with accent)
    sec_num = spec.get("section_number") or ""
    if sec_num:
        add_textbox(slide, Inches(1.2), Inches(1.0), Inches(11), Inches(1.3),
                    str(sec_num), fonts["heading"], 90, lighten(palette["primary"], 0.55),
                    bold=True)
    # Title
    add_textbox(slide, Inches(1.2), Inches(2.6), Inches(11.3), Inches(2.4),
                spec["title"], fonts["heading"], 56, "#ffffff", bold=True,
                line_spacing=1.05)
    # Subtitle
    if spec.get("subtitle"):
        add_textbox(slide, Inches(1.2), Inches(5.2), Inches(11.3), Inches(1.2),
                    spec["subtitle"], fonts["body"], 20, lighten(palette["primary"], 0.75),
                    line_spacing=1.3)
    # Accent bar at bottom
    add_rect(slide, Inches(1.2), Inches(6.6), Inches(2.5), Inches(0.08), palette["accent"])


def render_agenda_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    # Eyebrow
    add_textbox(slide, Inches(0.75), Inches(0.55), Inches(12), Inches(0.4),
                "AGENDA", fonts["body"], 12, palette["accent"], bold=True)
    # Title
    add_textbox(slide, Inches(0.75), Inches(0.9), Inches(12), Inches(1.0),
                spec.get("title") or "What we'll cover", fonts["heading"], 36, palette["primary"], bold=True)
    items = spec.get("items", [])[:6]
    # Two-column numbered list
    col_w = Inches(5.95)
    start_y = Inches(2.4)
    row_h = Inches(0.95)
    for i, item in enumerate(items):
        col = i % 2
        row = i // 2
        x = Inches(0.75) + (col_w + Inches(0.25)) * col
        y = start_y + row_h * row
        # Number circle
        add_circle(slide, x, y + Inches(0.05), Inches(0.55), palette["accent"])
        add_textbox(slide, x, y + Inches(0.05), Inches(0.55), Inches(0.55),
                    f"{i + 1:02d}", fonts["heading"], 14, "#ffffff", bold=True,
                    align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        # Item text
        add_textbox(slide, x + Inches(0.75), y, col_w - Inches(0.75), Inches(0.7),
                    item, fonts["body"], 16, palette["text"],
                    anchor=MSO_ANCHOR.MIDDLE)
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_big_text_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    # Small heading above
    add_textbox(slide, Inches(0.75), Inches(1.1), Inches(12), Inches(0.5),
                spec.get("title", ""), fonts["body"], 14, palette["accent"], bold=True)
    # Body — huge, calm
    add_textbox(slide, Inches(0.75), Inches(1.8), Inches(11.8), Inches(4.5),
                spec.get("body", ""), fonts["heading"], 42, palette["text"], bold=False,
                line_spacing=1.2)
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_bullets_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    # Eyebrow accent square
    add_rect(slide, Inches(0.75), Inches(0.7), Inches(0.18), Inches(0.18), palette["accent"])
    # Title
    add_textbox(slide, Inches(1.05), Inches(0.55), Inches(12), Inches(1.0),
                spec["title"], fonts["heading"], 34, palette["primary"], bold=True)
    # Bullets
    bullets = spec.get("bullets", [])[:6]
    y_start = Inches(2.1)
    for i, b in enumerate(bullets):
        y = y_start + Inches(i * 0.85)
        # Marker
        add_circle(slide, Inches(0.85), y + Inches(0.18), Inches(0.18), palette["accent"])
        # Bullet text
        add_textbox(slide, Inches(1.35), y, Inches(11.4), Inches(0.7),
                    b, fonts["body"], 20, palette["text"],
                    anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.25)
    if spec.get("notes"):
        slide.notes_slide.notes_text_frame.text = spec["notes"]
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_two_column_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    add_rect(slide, Inches(0.75), Inches(0.7), Inches(0.18), Inches(0.18), palette["accent"])
    add_textbox(slide, Inches(1.05), Inches(0.55), Inches(12), Inches(1.0),
                spec["title"], fonts["heading"], 32, palette["primary"], bold=True)

    card_bg = lighten(palette["primary"], 0.94) if palette["bg"].lower() in ("#ffffff", "#fff") else lighten(palette["bg"], 0.05)
    card_y = Inches(2.0); card_h = Inches(4.5)
    # Left card
    add_rounded(slide, Inches(0.75), card_y, Inches(5.95), card_h, card_bg)
    # Top-left accent strip
    add_rect(slide, Inches(0.75), card_y, Inches(0.18), card_h, palette["accent"])

    # Right card
    add_rounded(slide, Inches(6.85), card_y, Inches(5.95), card_h, card_bg)
    add_rect(slide, Inches(6.85), card_y, Inches(0.18), card_h, palette["primary"])

    def column(x, ctitle, items, accent):
        add_textbox(slide, x + Inches(0.45), card_y + Inches(0.35), Inches(5.0), Inches(0.5),
                    ctitle, fonts["heading"], 20, accent, bold=True)
        for i, item in enumerate(items[:5]):
            y = card_y + Inches(1.05 + i * 0.65)
            add_rect(slide, x + Inches(0.45), y + Inches(0.25), Inches(0.16), Inches(0.04), accent)
            add_textbox(slide, x + Inches(0.78), y, Inches(4.7), Inches(0.55),
                        item, fonts["body"], 15, palette["text"],
                        anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.3)

    column(Inches(0.75), spec.get("left_title", ""), spec.get("left_bullets", []), palette["accent"])
    column(Inches(6.85), spec.get("right_title", ""), spec.get("right_bullets", []), palette["primary"])
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_comparison_slide(slide, spec, palette, fonts, ctx):
    """Big A/B blocks — for 'before/after', 'old/new', 'us/them'."""
    add_solid_bg(slide, palette["bg"])
    add_rect(slide, Inches(0.75), Inches(0.7), Inches(0.18), Inches(0.18), palette["accent"])
    add_textbox(slide, Inches(1.05), Inches(0.55), Inches(12), Inches(1.0),
                spec.get("title", ""), fonts["heading"], 32, palette["primary"], bold=True)

    block_y = Inches(2.0); block_h = Inches(4.5)
    a_color = lighten(palette["text"], 0.55)
    side_bg = lighten(palette["bg"], 0.0) if palette["bg"].lower() != "#ffffff" else "#f7fafc"
    add_rounded(slide, Inches(0.75), block_y, Inches(5.95), block_h, side_bg)
    add_textbox(slide, Inches(1.1), block_y + Inches(0.3), Inches(5.3), Inches(0.5),
                spec.get("a_label", "Before"), fonts["heading"], 14, a_color, bold=True)
    add_textbox(slide, Inches(1.1), block_y + Inches(0.95), Inches(5.3), Inches(3.3),
                spec.get("a_body", ""), fonts["heading"], 26, a_color, bold=False,
                line_spacing=1.2)
    add_rounded(slide, Inches(6.85), block_y, Inches(5.95), block_h, palette["primary"])
    add_textbox(slide, Inches(7.2), block_y + Inches(0.3), Inches(5.3), Inches(0.5),
                spec.get("b_label", "After"), fonts["heading"], 14,
                lighten(palette["primary"], 0.75), bold=True)
    add_textbox(slide, Inches(7.2), block_y + Inches(0.95), Inches(5.3), Inches(3.3),
                spec.get("b_body", ""), fonts["heading"], 26, "#ffffff", bold=True,
                line_spacing=1.2)
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_timeline_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    add_rect(slide, Inches(0.75), Inches(0.7), Inches(0.18), Inches(0.18), palette["accent"])
    add_textbox(slide, Inches(1.05), Inches(0.55), Inches(12), Inches(1.0),
                spec.get("title", ""), fonts["heading"], 32, palette["primary"], bold=True)

    items = spec.get("items", [])[:5]
    n = max(1, len(items))
    line_y = Inches(3.5)
    add_rect(slide, Inches(1.0), line_y, Inches(11.3), Emu(19050), lighten(palette["text"], 0.7))
    step = Inches(11.3 / n)
    for i, it in enumerate(items):
        cx = Inches(1.0) + step * (i + 0.5) - Inches(0.3)
        add_circle(slide, cx, line_y - Inches(0.27), Inches(0.55), palette["accent"])
        add_textbox(slide, cx, line_y - Inches(0.27), Inches(0.55), Inches(0.55),
                    str(i + 1), fonts["heading"], 14, "#ffffff", bold=True,
                    align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE)
        label = it.get("label", "")
        add_textbox(slide, cx - Inches(0.85), line_y - Inches(1.15), Inches(2.25), Inches(0.5),
                    label, fonts["heading"], 16, palette["primary"], bold=True,
                    align=PP_ALIGN.CENTER)
        body = it.get("text", "")
        add_textbox(slide, cx - Inches(0.95), line_y + Inches(0.55), Inches(2.45), Inches(2.2),
                    body, fonts["body"], 13, palette["text"], align=PP_ALIGN.CENTER,
                    line_spacing=1.3)
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_stat_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    add_textbox(slide, Inches(0.75), Inches(1.1), Inches(12), Inches(0.5),
                spec.get("title", ""), fonts["body"], 14, palette["accent"], bold=True,
                align=PP_ALIGN.CENTER)
    # Auto-shrink the stat font if the AI sent a long string instead of a
    # short number/percent ("Kuchli o'sish" overflows; "$4.2M" fits at 140).
    stat = spec.get("stat_value", "")
    n = len(stat)
    if   n <= 6:  stat_pt = 140
    elif n <= 9:  stat_pt = 110
    elif n <= 13: stat_pt = 80
    elif n <= 20: stat_pt = 56
    else:         stat_pt = 40
    add_textbox(slide, Inches(0.5), Inches(2.0), Inches(12.5), Inches(2.6),
                stat, fonts["heading"], stat_pt, palette["primary"], bold=True,
                align=PP_ALIGN.CENTER, anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.0)
    add_rect(slide, Inches(5.7), Inches(4.85), Inches(2), Inches(0.08), palette["accent"])
    add_textbox(slide, Inches(0.75), Inches(5.05), Inches(12), Inches(0.6),
                spec.get("stat_label", ""), fonts["body"], 22, palette["text"],
                align=PP_ALIGN.CENTER)
    if spec.get("support"):
        add_textbox(slide, Inches(2), Inches(5.85), Inches(9.3), Inches(0.9),
                    spec["support"], fonts["body"], 14,
                    palette.get("muted", darken(palette["text"], 0.3)),
                    align=PP_ALIGN.CENTER, line_spacing=1.4)
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_quote_slide(slide, spec, palette, fonts, ctx):
    add_solid_bg(slide, palette["bg"])
    add_rect(slide, 0, 0, Inches(0.35), SLIDE_H, palette["accent"])
    qm = slide.shapes.add_textbox(Inches(1.0), Inches(0.6), Inches(2.5), Inches(2.5))
    qp = qm.text_frame.paragraphs[0]
    qr = qp.add_run(); qr.text = "“"
    qr.font.name = fonts["heading"]; qr.font.size = Pt(180); qr.font.bold = True
    qr.font.color.rgb = hex_to_rgb(palette["accent"])
    add_textbox(slide, Inches(1.6), Inches(2.4), Inches(11), Inches(3.2),
                spec.get("quote", ""), fonts["heading"], 32, palette["text"],
                italic=False, line_spacing=1.25)
    add_textbox(slide, Inches(1.6), Inches(5.85), Inches(11), Inches(0.5),
                "— " + spec.get("author", ""), fonts["body"], 17,
                palette.get("muted", darken(palette["text"], 0.3)), bold=True)
    add_chrome(slide, ctx["page"], ctx["total"], palette, fonts, ctx["footer"])


def render_thanks_slide(slide, spec, palette, fonts, _ctx):
    add_solid_bg(slide, palette["primary"])
    add_rect(slide, Inches(11.5), Inches(5.5), Inches(2), Inches(2), palette["accent"])
    add_textbox(slide, Inches(1.0), Inches(2.6), Inches(11.3), Inches(2.0),
                spec.get("title", "Thank you."), fonts["heading"], 80, "#ffffff", bold=True,
                anchor=MSO_ANCHOR.MIDDLE, line_spacing=1.05)
    if spec.get("subtitle"):
        add_textbox(slide, Inches(1.0), Inches(4.7), Inches(11.3), Inches(1.0),
                    spec["subtitle"], fonts["body"], 22, lighten(palette["primary"], 0.7),
                    line_spacing=1.3)
    add_rect(slide, Inches(1.0), Inches(6.0), Inches(2.5), Inches(0.1), palette["accent"])


LAYOUTS = {
    "title":      render_title_slide,
    "agenda":     render_agenda_slide,
    "section":    render_section_slide,
    "big_text":   render_big_text_slide,
    "bullets":    render_bullets_slide,
    "two_column": render_two_column_slide,
    "comparison": render_comparison_slide,
    "timeline":   render_timeline_slide,
    "stat":       render_stat_slide,
    "quote":      render_quote_slide,
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
    footer  = spec.get("footer", "")
    slides  = spec["slides"]
    total   = len(slides)

    for idx, ss in enumerate(slides):
        layout = ss.get("layout", "bullets")
        blank = prs.slide_layouts[6]
        slide = prs.slides.add_slide(blank)
        ctx = {"page": idx + 1, "total": total, "footer": footer}
        renderer = LAYOUTS.get(layout, render_bullets_slide)
        try:
            renderer(slide, ss, palette, fonts, ctx)
        except Exception as e:
            render_section_slide(slide, {"title": ss.get("title", "Slide"), "subtitle": str(e)[:80]}, palette, fonts, ctx)

    prs.save(out_path)
    print(f"OK {out_path}")


if __name__ == "__main__":
    main()
