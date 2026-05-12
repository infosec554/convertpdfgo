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
FROM alpine:3.19

# Runtime dependencies:
# - poppler-utils: pdftoppm (PDF→JPG), pdftotext (extract-text)
# - libreoffice + ttf-liberation: PDF→DOCX via pkg/libreoffice (worker
#   only, but the api image and worker image are the same Dockerfile
#   target — bigger image accepted for one consistent runtime).
#   ttf-liberation gives us serif/sans/mono fonts so the renderer
#   doesn't fall back to glyph squares.
# - pdfcpu CLI is bundled from the release tarball (rotate/crop/unlock/
#   protect/header-footer shell out to it; library API is also linked).
RUN apk add --no-cache \
        ca-certificates tzdata xz \
        poppler-utils \
        libreoffice ttf-liberation \
        ocrmypdf tesseract-ocr \
        tesseract-ocr-data-eng tesseract-ocr-data-rus && \
    # Uzbek (Latin + Cyrillic) and Kyrgyz aren't always packaged on
    # alpine — pull them straight from tesseract-ocr/tessdata so /usr/
    # share/tessdata has the full set our /api/pdf/ocr endpoints
    # advertise. ~10 MB total.
    # `osd` is the orientation/script detection model — required for
    # `tesseract --psm 0` which our /api/pdf/ocr-detect-lang endpoint
    # invokes. Alpine doesn't package it separately, so pull alongside
    # the language data.
    for lang in uzb uzb_cyrl kir osd; do \
      wget -qO "/usr/share/tessdata/${lang}.traineddata" \
        "https://github.com/tesseract-ocr/tessdata/raw/main/${lang}.traineddata"; \
    done && \
    wget -qO- https://github.com/pdfcpu/pdfcpu/releases/download/v0.11.0/pdfcpu_0.11.0_Linux_x86_64.tar.xz \
        | tar -xJf- -C /tmp && \
    install -m 0755 /tmp/pdfcpu_0.11.0_Linux_x86_64/pdfcpu /usr/local/bin/pdfcpu && \
    rm -rf /tmp/pdfcpu_0.11.0_Linux_x86_64 && \
    pdfcpu version

WORKDIR /app
COPY --from=builder /out/app /app/app
COPY migrations ./migrations

EXPOSE 8080
ENTRYPOINT ["/app/app"]
