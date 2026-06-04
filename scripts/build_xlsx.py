#!/usr/bin/env python3
"""Build a professional XLSX from a JSON spec produced by the LLM.

Spec shape:
{
  "title": str,
  "language": "en|uz|ru|es|ar",
  "palette": {"header_bg": "#hex", "header_text": "#hex", "accent": "#hex", "zebra": "#hex"},
  "sheets": [
    {
      "name": "Q1",
      "headers": ["Item", "Q1", "Q2", "Q3", "Q4", "Total"],
      "rows": [
        ["Revenue",  1000, 1200, 1500, 1800, "=SUM(B2:E2)"],
        ["COGS",     -400, -480, -600, -720, "=SUM(B3:E3)"]
      ],
      "totals_row": ["TOTAL", "=SUM(B2:B3)", "=SUM(C2:C3)", "=SUM(D2:D3)", "=SUM(E2:E3)", "=SUM(F2:F3)"],
      "number_format":  "$#,##0",
      "freeze_panes": "B2",                          // optional
      "column_widths": [22, 12, 12, 12, 12, 14],     // optional, len matches headers
      "chart": {                                     // optional
        "type":  "column" | "line" | "bar" | "pie",
        "title": "Quarterly revenue",
        "categories_col":  1,                        // 1-indexed within data range
        "value_cols":      [2, 3, 4, 5],
        "categories_first_row": 2,                   // first row of data (after headers)
        "categories_last_row":  3,                   // last row of data (totals row excluded)
        "position": "H2"
      }
    }
  ]
}

Usage:
  python3 build_xlsx.py <spec.json> <output.xlsx>
"""
import json
import sys
import xlsxwriter


def main():
    spec_path = sys.argv[1]
    out_path  = sys.argv[2]
    with open(spec_path) as f:
        spec = json.load(f)

    # nan_inf_to_errors avoids hard crashes on edge cell values;
    # use_zip64 lets large workbooks save without running into the
    # 4GB ZIP limit. We also flip strings_to_formulas on so plain
    # "=SUM(...)" in the JSON spec gets treated as a formula even
    # if downstream callers don't normalise.
    wb = xlsxwriter.Workbook(out_path, {
        "in_memory":          True,
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
    cell_fmt = wb.add_format({"font_name": "Calibri", "font_size": 11, "border": 1, "border_color": "#e2e8f0", "valign": "vcenter"})
    cell_zebra_fmt = wb.add_format({"font_name": "Calibri", "font_size": 11, "border": 1, "border_color": "#e2e8f0", "bg_color": zebra, "valign": "vcenter"})
    totals_fmt = wb.add_format({"bold": True, "bg_color": accent, "font_color": "#ffffff", "border": 1, "border_color": accent, "font_size": 11, "font_name": "Calibri"})
    title_fmt  = wb.add_format({"bold": True, "font_size": 14, "font_color": header_bg, "font_name": "Calibri"})

    for sheet_spec in spec.get("sheets", []):
        sheet = wb.add_worksheet(sheet_spec["name"][:31])

        headers = sheet_spec.get("headers", [])
        rows    = sheet_spec.get("rows", [])
        totals  = sheet_spec.get("totals_row")
        num_fmt = sheet_spec.get("number_format", "")

        # Title on row 0
        title = sheet_spec.get("title") or spec.get("title")
        if title:
            sheet.merge_range(0, 0, 0, max(0, len(headers) - 1), title, title_fmt)
            sheet.set_row(0, 26)

        header_row = 2
        for col, h in enumerate(headers):
            sheet.write(header_row, col, h, header_fmt)
        sheet.set_row(header_row, 24)

        # Data rows
        for r, row in enumerate(rows):
            actual_row = header_row + 1 + r
            fmt_base = cell_zebra_fmt if r % 2 == 1 else cell_fmt
            row_fmt = wb.add_format({
                "font_name": "Calibri", "font_size": 11, "border": 1, "border_color": "#e2e8f0",
                "bg_color": zebra if r % 2 == 1 else "#ffffff", "valign": "vcenter",
                "num_format": num_fmt or "General",
            })
            row_fmt_text = wb.add_format({
                "font_name": "Calibri", "font_size": 11, "border": 1, "border_color": "#e2e8f0",
                "bg_color": zebra if r % 2 == 1 else "#ffffff", "valign": "vcenter",
            })
            for c, val in enumerate(row):
                if isinstance(val, str) and val.startswith("="):
                    sheet.write_formula(actual_row, c, val, row_fmt)
                elif isinstance(val, (int, float)):
                    sheet.write_number(actual_row, c, val, row_fmt)
                else:
                    sheet.write(actual_row, c, val, row_fmt_text)

        last_data_row = header_row + len(rows)
        totals_row_idx = None
        # Totals row
        if totals:
            totals_row_idx = last_data_row + 1
            for c, val in enumerate(totals):
                tfmt = wb.add_format({
                    "bold": True, "bg_color": accent, "font_color": "#ffffff",
                    "border": 1, "border_color": accent, "font_size": 11, "font_name": "Calibri",
                    "num_format": num_fmt or "General",
                })
                if isinstance(val, str) and val.startswith("="):
                    sheet.write_formula(totals_row_idx, c, val, tfmt)
                elif isinstance(val, (int, float)):
                    sheet.write_number(totals_row_idx, c, val, tfmt)
                else:
                    sheet.write(totals_row_idx, c, val, totals_fmt)

        # Column widths
        widths = sheet_spec.get("column_widths") or [16] * len(headers)
        for c, w in enumerate(widths):
            sheet.set_column(c, c, w)

        # Freeze panes
        fz = sheet_spec.get("freeze_panes")
        if fz:
            try:
                # Excel ref like "B4" → split row + col
                col_letters = "".join(ch for ch in fz if ch.isalpha())
                row_digits  = "".join(ch for ch in fz if ch.isdigit())
                col_idx = 0
                for ch in col_letters.upper():
                    col_idx = col_idx * 26 + (ord(ch) - ord("A") + 1)
                col_idx -= 1
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
            cat_first = header_row + 1
            cat_last  = last_data_row
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
