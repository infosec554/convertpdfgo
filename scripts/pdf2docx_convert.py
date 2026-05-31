#!/usr/bin/env python3
"""Convert a PDF to a DOCX file via the pdf2docx library.

Used by service/pdf_to_word.go. We shell out to Python because the Go
ecosystem has no comparable library — every Go option produces the
same absolute-positioned text boxes LibreOffice's writer_pdf_import
filter does. pdf2docx reconstructs paragraph flow, headings, lists,
and tables, which is what users actually expect from a PDF→Word
converter.

Usage:
    python3 pdf2docx_convert.py <input.pdf> <output.docx>

Exit codes: 0 on success, 1 on failure (with reason on stderr).
"""

import sys
from pdf2docx import Converter


def main() -> int:
    if len(sys.argv) != 3:
        print(f"usage: {sys.argv[0]} <input.pdf> <output.docx>", file=sys.stderr)
        return 1

    in_pdf, out_docx = sys.argv[1], sys.argv[2]
    cv = Converter(in_pdf)
    try:
        cv.convert(out_docx)
    finally:
        cv.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
