# Lightweight App Review mock API — no dashboard / kube client.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/demo-api ./cmd/demo-api

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /demo-api ./cmd/demo-api

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.title="KubePilot Demo API" \
      org.opencontainers.image.description="Fixture KubePilot API for Apple App Review"

COPY --from=build /demo-api /demo-api
USER nonroot:nonroot
EXPOSE 8383
ENV DEMO_API_ADDR=:8383
ENTRYPOINT ["/demo-api"]
