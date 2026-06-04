# syntax=docker/dockerfile:1.6

# === Build stage ===
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache modules separately so re-builds don't redownload everything when
# only application code changes. We tidy + build in one step so go.sum
# updates from new deps land in the cache.
COPY go.mod go.sum ./
COPY . .

ARG TARGET=api
# api/docs/ is .gitignored — regenerate via swag before building so the
# `_ "convertpdfgo/api/docs"` import in api/router.go resolves. The
# worker target doesn't need it (no router import), but generating
# unconditionally is simpler than branching here.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init --g api/router.go -o api/docs && \
    case "$TARGET" in \
      api)    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd ;; \
      worker) CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd/worker ;; \
      *) echo "unknown TARGET: $TARGET" && exit 1 ;; \
    esac

# === Runtime stage ===
# Debian (not Alpine) because pdf2docx's transitive dep on PyMuPDF +
# opencv-python ships manylinux wheels that need glibc. Building those
# from source on Alpine takes 20+ minutes and ~1 GB of build deps —
# not worth it for one library. Tesseract / LibreOffice / poppler /
# ocrmypdf all have matching debian packages.
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive
# Runtime dependencies — same set as the alpine version, debian names:
# - poppler-utils: pdftoppm (PDF→JPG), pdftotext (extract-text)
# - libreoffice + fonts-liberation: PDF→DOCX fallback via pkg/libreoffice;
#   fonts-liberation gives us serif/sans/mono so the renderer doesn't
#   fall back to glyph squares.
# - pdfcpu CLI is bundled from the release tarball.
# - python3 + pip + pdf2docx: primary PDF→DOCX path (flowing paragraph
#   output instead of LibreOffice's absolute-positioned textboxes).
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates tzdata xz-utils wget \
        poppler-utils \
        libreoffice fonts-liberation openjdk-17-jre-headless \
        ocrmypdf tesseract-ocr \
        tesseract-ocr-eng tesseract-ocr-rus \
        python3 python3-pip && \
    # Uzbek (Latin + Cyrillic) and Kyrgyz aren't always in debian's
    # tesseract package — pull them straight from upstream so the OCR
    # endpoints have the full advertised language set. `osd` is the
    # orientation/script detection model needed for psm=0.
    for lang in uzb uzb_cyrl kir osd; do \
      wget -qO "/usr/share/tesseract-ocr/5/tessdata/${lang}.traineddata" \
        "https://github.com/tesseract-ocr/tessdata/raw/main/${lang}.traineddata"; \
    done && \
    wget -qO- https://github.com/pdfcpu/pdfcpu/releases/download/v0.11.0/pdfcpu_0.11.0_Linux_x86_64.tar.xz \
        | tar -xJf- -C /tmp && \
    install -m 0755 /tmp/pdfcpu_0.11.0_Linux_x86_64/pdfcpu /usr/local/bin/pdfcpu && \
    rm -rf /tmp/pdfcpu_0.11.0_Linux_x86_64 && \
    pdfcpu version && \
    # pdf2docx — the de-facto best free PDF→DOCX library. We
    # --break-system-packages because debian's PEP-668 system-wide pip
    # refuses by default; using a venv would mean an extra binary in
    # PATH for one library, not worth the complexity.
    pip3 install --no-cache-dir --break-system-packages pdf2docx python-pptx lxml xlsxwriter && \
    python3 -c "from pdf2docx import Converter; print('pdf2docx OK')" && \
    python3 -c "from pptx import Presentation; print('python-pptx OK')" && \
    python3 -c "import xlsxwriter; print('xlsxwriter OK')" && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/app /app/app
COPY scripts/pdf2docx_convert.py /app/scripts/pdf2docx_convert.py
COPY scripts/build_pptx.py /app/scripts/build_pptx.py
COPY scripts/build_xlsx.py /app/scripts/build_xlsx.py
COPY scripts/sandbox_xlsx.py /app/scripts/sandbox_xlsx.py
COPY migrations ./migrations

EXPOSE 8080
ENTRYPOINT ["/app/app"]
