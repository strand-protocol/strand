# ============================================================
# nexapi -- Multi-stage Docker build
# Produces the NexAPI inference and echo example binaries.
# ============================================================

# -----------------------------------------------------------
# Stage 1: Builder
# -----------------------------------------------------------
FROM golang:1.22-bookworm AS builder

WORKDIR /src

# Cache dependency downloads in a separate layer.
COPY go.mod ./
# go.sum may not exist if there are no external dependencies.
COPY go.su[m] ./
RUN go mod download

# Copy the full module source.
COPY . .

# Build the inference example server (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /nexapi-inference \
    ./examples/inference

# Build the echo example server (static, stripped).
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w" \
    -o /nexapi-echo \
    ./examples/echo

# -----------------------------------------------------------
# Stage 2: Minimal runtime
# -----------------------------------------------------------
FROM gcr.io/distroless/static-debian12

COPY --from=builder /nexapi-inference /nexapi-inference
COPY --from=builder /nexapi-echo /nexapi-echo

EXPOSE 9000

ENTRYPOINT ["/nexapi-inference"]
