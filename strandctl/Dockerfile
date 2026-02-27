# ============================================================
# nexctl -- Multi-stage Docker build
# Produces the nexctl CLI binary.
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

# Build the nexctl binary (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /nexctl \
    .

# -----------------------------------------------------------
# Stage 2: Minimal runtime
# -----------------------------------------------------------
FROM gcr.io/distroless/static-debian12

COPY --from=builder /nexctl /nexctl

ENTRYPOINT ["/nexctl"]
