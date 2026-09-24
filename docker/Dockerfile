# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# Tahap 1 — bangun CSS dengan Tailwind.
# Node hanya hidup di tahap ini dan tidak ikut ke image akhir.
# ---------------------------------------------------------------------------
FROM node:22-alpine AS css

WORKDIR /build

COPY package.json ./
RUN npm install --no-audit --no-fund --loglevel=error

COPY tailwind.config.js ./
COPY web ./web
RUN npx tailwindcss \
      -i ./web/static/css/source.css \
      -o ./web/static/css/app.css \
      --minify

# ---------------------------------------------------------------------------
# Tahap 2 — kompilasi aplikasi Go.
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

# git dibutuhkan go toolchain untuk sebagian modul.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Unduh dependensi lebih dulu agar layer ini tetap ter-cache
# selama go.mod/go.sum tidak berubah.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# templ CLI dipasang dengan versi yang sama seperti library di go.mod.
RUN --mount=type=cache,target=/go/pkg/mod \
    go install github.com/a-h/templ/cmd/templ@v0.3.1020

COPY . .

# Berkas *_templ.go dibuat ulang saat build supaya tidak pernah tertinggal
# dari berkas .templ sumbernya.
RUN templ generate

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/server ./cmd/server

# ---------------------------------------------------------------------------
# Tahap 3 — image akhir.
# Alpine dipilih (bukan distroless murni) karena `make shell`, healthcheck,
# dan pemeriksaan di server membutuhkan shell. Ukurannya tetap di bawah 20 MB.
# ---------------------------------------------------------------------------
FROM alpine:3.21 AS final

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 10001 -S app && \
    adduser -u 10001 -S -G app -h /app app

ENV TZ=Asia/Jakarta
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=css     /build/web/static /app/web/static

# Folder upload dibuat sebagai milik user aplikasi; isinya datang dari volume.
RUN mkdir -p /app/storage/uploads && chown -R app:app /app

USER app:app

EXPOSE 3000

# Port tidak dipublikasikan ke host — Nginx Proxy Manager di server yang
# meneruskan trafik ke container ini lewat jaringan Docker.
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:3000/sehat || exit 1

ENTRYPOINT ["/app/server"]
