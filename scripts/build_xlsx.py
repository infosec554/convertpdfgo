#!/usr/bin/env python3
"""Build a professional XLSX from a JSON spec produced by the LLM.

v2 (2026-06): added cached-value computation for formulas so that
LibreOffice / preview tools that don't recalculate on open still
show the right numbers. Plus a robust topic-faithful renderer.

Spec shape (see service/xlsx_builder.go for the canonical version):

{
  "title": str, "language": "en|uz|ru|es|ar",
  "palette": {"header_bg": "#hex", "header_text": "#hex", "accent": "#hex", "zebra": "#hex"},
  "sheets": [{
    "name": "...", "title": "...",
    "headers": ["A", "B", ...],
    "rows": [["label", num, "=B4*0.9", 90], ...],
    "totals_row": ["TOTAL", "=SUM(B4:B6)", null, ...],
    "number_format": "$#,##0",
    "freeze_panes": "B4",
    "column_widths": [...],
    "chart": {"type": "column|line|bar|pie", "title": "...", "categories_col": 1,
              "value_cols": [2,3], "categories_first_row": 4, "categories_last_row": 6,
              "position": "H4"}
  }]
}
"""
import json
import re
import sys
import xlsxwriter


# ─── tiny formula evaluator ─────────────────────────────────────────────
#
# xlsxwriter cannot evaluate formulas — Excel/LibreOffice recalc on
# their own when the file opens. LibreOffice's headless conversion
# doesn't recalc, so preview / PDF renders show "0" for formula cells.
# We solve this by pre-computing simple formulas in Python and writing
# the resolved value alongside the formula via write_formula(...,
# value=...). Excel/Sheets/Calc all honour the cached value when they
# open the file and only recalc on edit.

CELL_RE = re.compile(r"([A-Z]+)([0-9]+)")
RANGE_RE = re.compile(r"([A-Z]+[0-9]+):([A-Z]+[0-9]+)")
FUNC_RE  = re.compile(r"([A-Z]+)\(([^)]*)\)")


def col_letter_to_index(s):
    """A->0, B->1, ..., Z->25, AA->26"""
    idx = 0
    for ch in s:
        idx = idx * 26 + (ord(ch) - ord("A") + 1)
    return idx - 1


def cell_to_rc(ref):
    """'B4' → (row=3, col=1) — 0-indexed."""
    m = CELL_RE.match(ref.upper())
    if not m:
        return None
    col = col_letter_to_index(m.group(1))
    row = int(m.group(2)) - 1
    return (row, col)


def evaluate_formula(formula, grid):
    """Resolve =SUM(...), =AVERAGE(...), =MAX(...), =MIN(...), =COUNT(...)
    and arithmetic combos using cells that already have numeric values
    in `grid` (dict {(row, col): number}). Unknown → None."""
    if not isinstance(formula, str) or not formula.startswith("="):
        return None

    expr = formula[1:].strip().upper()

    # Replace =SUM(A1:A5) / =AVERAGE(...) / =MIN/MAX/COUNT(...)
    def replace_func(m):
        fn   = m.group(1)
        args = m.group(2).strip()
        # Expand ranges to lists of cell refs
        values = []
        for piece in [p.strip() for p in args.split(",")]:
            if not piece:
                continue
            rm = RANGE_RE.match(piece)
            if rm:
                r1, c1 = cell_to_rc(rm.group(1))
                r2, c2 = cell_to_rc(rm.group(2))
                if None in (r1, c1, r2, c2):
                    continue
                for rr in range(min(r1, r2), max(r1, r2) + 1):
                    for cc in range(min(c1, c2), max(c1, c2) + 1):
                        v = grid.get((rr, cc))
                        if isinstance(v, (int, float)):
                            values.append(v)
            else:
                rc = cell_to_rc(piece)
                if rc:
                    v = grid.get(rc)
                    if isinstance(v, (int, float)):
                        values.append(v)
        if not values:
            return "0"
        if fn == "SUM":     return str(sum(values))
        if fn == "AVERAGE": return str(sum(values) / len(values))
        if fn == "MAX":     return str(max(values))
        if fn == "MIN":     return str(min(values))
        if fn == "COUNT":   return str(len(values))
        return "0"

    expr = FUNC_RE.sub(replace_func, expr)

    # Replace bare cell refs ("B4") with their value
    def replace_cell(m):
        rc = cell_to_rc(m.group(0))
        v = grid.get(rc) if rc else None
        return str(v) if isinstance(v, (int, float)) else "0"

    expr = CELL_RE.sub(replace_cell, expr)

    # Sanitise — only allow digits, ops, parens, dot, minus.
    if not re.match(r"^[0-9+\-*/().\s]+$", expr):
        return None
    try:
        return eval(expr, {"__builtins__": {}}, {})  # noqa: S307 — sanitised
    except Exception:
        return None


def main():
    spec_path = sys.argv[1]
    out_path  = sys.argv[2]
    with open(spec_path) as f:
        spec = json.load(f)

    wb = xlsxwriter.Workbook(out_path, {
        "in_memory":           True,
        "strings_to_formulas": True,
        "nan_inf_to_errors":   True,
    })

    palette = spec.get("palette", {})
    header_bg   = palette.get("header_bg",   "#1a365d")
    header_text = palette.get("header_text", "#ffffff")
    accent      = palette.get("accent",      "#ed8936")
    zebra       = palette.get("zebra",       "#f7fafc")

    header_fmt = wb.add_format({
        "bold": True, "font_color": header_text, "bg_color": header_bg,
        "align": "center", "valign": "vcenter", "border": 1, "border_color": header_bg,
        "font_size": 11, "font_name": "Calibri",
    })
    title_fmt = wb.add_format({"bold": True, "font_size": 14, "font_color": header_bg, "font_name": "Calibri"})

    for sheet_spec in spec.get("sheets", []):
        sheet = wb.add_worksheet(sheet_spec["name"][:31])

        headers = sheet_spec.get("headers", [])
        rows    = sheet_spec.get("rows", [])
        totals  = sheet_spec.get("totals_row")
        num_fmt = sheet_spec.get("number_format", "")

        # Title on row 0
        title = sheet_spec.get("title") or spec.get("title")
        if title and headers:
            sheet.merge_range(0, 0, 0, max(0, len(headers) - 1), title, title_fmt)
            sheet.set_row(0, 26)

        header_row = 2
        for col, h in enumerate(headers):
            sheet.write(header_row, col, h, header_fmt)
        sheet.set_row(header_row, 24)

        # ── pass 1: write data rows, build a (row, col) → numeric value map
        grid = {}
        pending_formulas = []   # (row, col, formula, format) — resolve in pass 2

        for r, row in enumerate(rows):
            actual_row = header_row + 1 + r
            zebra_on = r % 2 == 1
            row_fmt_num = wb.add_format({
                "font_name": "Calibri", "font_size": 11, "border": 1, "border_color": "#e2e8f0",
                "bg_color": zebra if zebra_on else "#ffffff", "valign": "vcenter",
                "num_format": num_fmt or "General",
            })
            row_fmt_txt = wb.add_format({
                "font_name": "Calibri", "font_size": 11, "border": 1, "border_color": "#e2e8f0",
                "bg_color": zebra if zebra_on else "#ffffff", "valign": "vcenter",
            })
            for c, val in enumerate(row):
                if isinstance(val, bool):
                    sheet.write(actual_row, c, val, row_fmt_txt)
                elif isinstance(val, (int, float)):
                    sheet.write_number(actual_row, c, val, row_fmt_num)
                    grid[(actual_row, c)] = val
                elif isinstance(val, str) and val.startswith("="):
                    pending_formulas.append((actual_row, c, val, row_fmt_num))
                elif val is None or val == "":
                    sheet.write_blank(actual_row, c, None, row_fmt_txt)
                else:
                    sheet.write(actual_row, c, val, row_fmt_txt)

        last_data_row = header_row + len(rows)

        # ── totals row (cells may also be formulas — handled in pass 2)
        totals_row_idx = None
        if totals:
            totals_row_idx = last_data_row + 1
            totals_fmt_num = wb.add_format({
                "bold": True, "bg_color": accent, "font_color": "#ffffff",
                "border": 1, "border_color": accent, "font_size": 11, "font_name": "Calibri",
                "num_format": num_fmt or "General",
            })
            totals_fmt_txt = wb.add_format({
                "bold": True, "bg_color": accent, "font_color": "#ffffff",
                "border": 1, "border_color": accent, "font_size": 11, "font_name": "Calibri",
            })
            for c, val in enumerate(totals):
                if isinstance(val, bool):
                    sheet.write(totals_row_idx, c, val, totals_fmt_txt)
                elif isinstance(val, (int, float)):
                    sheet.write_number(totals_row_idx, c, val, totals_fmt_num)
                    grid[(totals_row_idx, c)] = val
                elif isinstance(val, str) and val.startswith("="):
                    pending_formulas.append((totals_row_idx, c, val, totals_fmt_num))
                elif val is None or val == "":
                    sheet.write_blank(totals_row_idx, c, None, totals_fmt_txt)
                else:
                    sheet.write(totals_row_idx, c, val, totals_fmt_txt)

        # ── pass 2: resolve formulas using the numeric grid.
        # We iterate up to 3 times so formulas that reference other
        # formulas (rare but possible) get resolved transitively.
        for _ in range(3):
            unresolved = []
            for (rr, cc, formula, fmt) in pending_formulas:
                v = evaluate_formula(formula, grid)
                if isinstance(v, (int, float)):
                    sheet.write_formula(rr, cc, formula, fmt, v)
                    grid[(rr, cc)] = v
                else:
                    unresolved.append((rr, cc, formula, fmt))
            pending_formulas = unresolved
            if not pending_formulas:
                break
        # Any leftover formulas could not be statically resolved (e.g.
        # circular refs, unsupported function). Write them without a
        # cached value — Excel/Calc will recompute on first open.
        for (rr, cc, formula, fmt) in pending_formulas:
            sheet.write_formula(rr, cc, formula, fmt)

        # Column widths
        widths = sheet_spec.get("column_widths") or [16] * len(headers)
        for c, w in enumerate(widths):
            sheet.set_column(c, c, w)

        # Freeze panes
        fz = sheet_spec.get("freeze_panes")
        if fz:
            try:
                col_letters = "".join(ch for ch in fz if ch.isalpha())
                row_digits  = "".join(ch for ch in fz if ch.isdigit())
                col_idx = col_letter_to_index(col_letters.upper())
                row_idx = int(row_digits) - 1
                sheet.freeze_panes(row_idx, col_idx)
            except Exception:
                sheet.freeze_panes(header_row + 1, 1)
        else:
            sheet.freeze_panes(header_row + 1, 1)

        # Chart
        ch_spec = sheet_spec.get("chart")
        if ch_spec and rows:
            chart = wb.add_chart({"type": ch_spec.get("type", "column")})
            cat_col   = ch_spec.get("categories_col", 1) - 1
            cat_first = (ch_spec.get("categories_first_row") or (header_row + 2)) - 1
            cat_last  = (ch_spec.get("categories_last_row")  or (last_data_row  + 1)) - 1
            # Make sure the chart range only covers data rows (not totals)
            cat_last  = min(cat_last, last_data_row)
            for vcol_1 in ch_spec.get("value_cols", []):
                vcol = vcol_1 - 1
                if 0 <= vcol < len(headers):
                    chart.add_series({
                        "name":       [sheet.name, header_row, vcol],
                        "categories": [sheet.name, cat_first, cat_col, cat_last, cat_col],
                        "values":     [sheet.name, cat_first, vcol,    cat_last, vcol],
                        "fill":       {"color": accent if (vcol_1 % 2 == 0) else header_bg},
                        "border":     {"color": "#ffffff"},
                    })
            chart.set_title({"name": ch_spec.get("title", "")})
            chart.set_legend({"position": "bottom"})
            chart.set_size({"width": 520, "height": 320})
            sheet.insert_chart(ch_spec.get("position", "H3"), chart)

    wb.close()
    print(f"OK {out_path}")


if __name__ == "__main__":
    main()
