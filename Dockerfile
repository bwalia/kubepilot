# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: Build the Next.js dashboard as a static export
# ─────────────────────────────────────────────────────────────────────────────
# Pin to the native build platform: the dashboard static export is
# architecture-independent, and building it under QEMU emulation for a foreign
# target arch is slow and can silently fail to emit `out/` (the cause of
# `"/build/dashboard/out": not found` in multi-arch buildx runs).
FROM --platform=$BUILDPLATFORM node:20-alpine AS dashboard-builder

WORKDIR /build/dashboard

COPY dashboard/package.json dashboard/package-lock.json* ./
RUN npm ci --prefer-offline

COPY dashboard/ ./
RUN npm run build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: Build the KubePilot Go binary
# ─────────────────────────────────────────────────────────────────────────────
# Build natively and cross-compile for the target arch (Go cross-compiles
# cheaply), rather than emulating the whole toolchain under QEMU.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS go-builder

# Redeclared with NO default on purpose. TARGETOS/TARGETARCH are automatic
# platform args: a stage only sees them if it redeclares them, and a default
# value shadows the value BuildKit injects. `ARG TARGETARCH=amd64` therefore
# pinned every target to amd64 — the linux/arm64 image shipped an x86-64 binary
# that died with `exec /kubepilot: no such file or directory` on arm64 nodes.
# Left empty (a plain non-BuildKit `docker build`), go builds for the host arch,
# which is the right answer there anyway.
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git

WORKDIR /build

# Cache module downloads separately from source to speed up rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Embed the pre-built dashboard static files into the binary's working directory.
COPY --from=dashboard-builder /build/dashboard/out ./dashboard/out

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-s -w -X github.com/kubepilot/kubepilot/internal/version.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
    -o /kubepilot ./cmd/kubepilot

# ─────────────────────────────────────────────────────────────────────────────
# Stage 3: Minimal production image
# ─────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.title="KubePilot" \
      org.opencontainers.image.description="AI-driven Kubernetes autopilot — single binary" \
      org.opencontainers.image.source="https://github.com/kubepilot/kubepilot"

# Copy the binary (includes embedded dashboard static files via filesystem).
COPY --from=go-builder /kubepilot /kubepilot
# Copy the dashboard out/ directory so the binary can serve it at runtime.
COPY --from=go-builder /build/dashboard/out /dashboard/out

# The server resolves the dashboard as the relative path ./dashboard/out, so the
# working directory has to be the one the files were copied under.
#
# This is not redundant: the distroless nonroot base sets WORKDIR=/home/nonroot,
# which made every dashboard URL resolve to a path that does not exist and
# return 404 while /healthz and the API kept working — so the image looked
# healthy to probes while serving no UI at all.
WORKDIR /

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["/kubepilot", "serve"]
