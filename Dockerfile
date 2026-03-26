# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for any VCS-based go module fetches.
RUN apk add --no-cache git ca-certificates

# Download dependencies first (layer cache optimization).
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /out/server ./cmd/server

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.20

WORKDIR /app

# ca-certs for HTTPS calls; tzdata for correct time zone handling.
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary and migrations from the build stage.
COPY --from=builder /out/server /usr/local/bin/server
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
