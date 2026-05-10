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
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod tidy && \
    case "$TARGET" in \
      api)    CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd ;; \
      worker) CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd/worker ;; \
      *) echo "unknown TARGET: $TARGET" && exit 1 ;; \
    esac

# === Runtime stage ===
FROM alpine:3.19

# pdfcpu is bundled at build time (Go binary) for the services that use
# its library API, but rotate/crop/unlock/protect/header-footer shell out
# to the `pdfcpu` CLI — install it from the release tarball. poppler-utils
# brings pdftoppm (PDF→JPG) and pdftotext (extract-text).
RUN apk add --no-cache ca-certificates tzdata poppler-utils xz && \
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
