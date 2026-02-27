# ============================================================
# strandctl -- Multi-stage Docker build
# Produces the strandctl CLI binary.
# ============================================================

# -----------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------
FROM golang:1.22-bookworm AS builder

WORKDIR /src

# Cache dependency downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full module source.
COPY . .

# Build the strandctl binary (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /strandctl \
    .

# -----------------------------------------------------------
# Stage 2: Minimal runtime
# -----------------------------------------------------------
FROM gcr.io/distroless/static-debian12

COPY --from=builder /strandctl /strandctl

ENTRYPOINT ["/strandctl"]
