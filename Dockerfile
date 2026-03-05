# syntax=docker/dockerfile:1

# ── Stage 1: download modules (cached layer) ─────────────────────────────────
FROM golang:1.25-alpine AS deps
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# ── Stage 2: builder — full source + Go toolchain (used by compose services) ─
FROM deps AS builder
RUN go install github.com/air-verse/air@latest
COPY . .

# ── Stage 3: release — compile optimised static binary ───────────────────────
FROM builder AS release
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/server ./cmd/server

# ── Stage 4: final — minimal distroless runtime image ────────────────────────
FROM gcr.io/distroless/static-debian12 AS final
COPY --from=release /out/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
