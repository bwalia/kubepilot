# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: Build the Next.js dashboard as a static export
# ─────────────────────────────────────────────────────────────────────────────
FROM node:20-alpine AS dashboard-builder

WORKDIR /build/dashboard

COPY dashboard/package.json dashboard/package-lock.json* ./
RUN npm ci --prefer-offline

COPY dashboard/ ./
RUN npm run build

# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: Build the KubePilot Go binary
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS go-builder

RUN apk add --no-cache git

WORKDIR /build

# Cache module downloads separately from source to speed up rebuilds.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Embed the pre-built dashboard static files into the binary's working directory.
COPY --from=dashboard-builder /build/dashboard/out ./dashboard/out

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
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

USER nonroot:nonroot

EXPOSE 8080 9090

ENTRYPOINT ["/kubepilot", "serve"]
